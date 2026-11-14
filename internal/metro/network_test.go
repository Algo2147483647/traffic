package metro

import (
	"fmt"
	"math"
	"testing"
)

func TestRadialGeometryAndAllLinesDeliver(t *testing.T) {
	c := DefaultConfig()
	c.Demand = 0
	c.Dwell = 5
	c.Headway = 30
	center := Point{1800, 1100}
	r := NewRadial(center, c)
	v := r.Snapshot()
	if len(v.Lines) != 3 || len(v.Trains) != 6 || len(v.Stations) != 15 || v.Layer != -1 {
		t.Fatal("radial topology", v)
	}
	for i, line := range v.Lines {
		if len(line.Stations) != 5 {
			t.Fatal("station count")
		}
		a := v.Stations[line.Stations[0]].Position
		b := v.Stations[line.Stations[4]].Position
		angle := math.Atan2(b.Y-a.Y, b.X-a.X)
		if math.Abs(angle-float64(i)*math.Pi/3) > 1e-10 || math.Abs(math.Hypot(a.X-center.X, a.Y-center.Y)-4000) > 1e-8 {
			t.Fatal("line bearing or extent")
		}
		for j := 1; j < 5; j++ {
			a, b := v.Stations[line.Stations[j-1]].Position, v.Stations[line.Stations[j]].Position
			if math.Abs(math.Hypot(b.X-a.X, b.Y-a.Y)-2000) > 1e-8 {
				t.Fatal("station spacing")
			}
		}
		for j := 0; j < 10; j++ {
			if !r.AddPassenger(i*5, i*5+4) || !r.AddPassenger(i*5+4, i*5) {
				t.Fatal("same-line trip rejected")
			}
		}
	}
	if r.AddPassenger(0, 9) {
		t.Fatal("unsupported transfer accepted")
	}
	seen := map[int]bool{}
	for step := 0; step < 100000; step++ {
		r.Advance(.05)
		for _, line := range r.lines {
			for _, tr := range line.trains {
				ownsCrossing := (tr.State == "moving" && (tr.Next == 2 || (tr.Station == 2 && tr.Distance < tr.Length))) || (tr.State != "moving" && tr.Station == 2)
				if ownsCrossing && line.gate.owner != line.lineID*100+tr.ID {
					t.Fatal("central crossing conflict")
				}
				if tr.Station == 2 {
					seen[line.lineID*100+tr.ID] = true
				}
			}
		}
		if step%20 == 0 {
			v = r.Snapshot()
			checkCars(t, v)
			if v.Completed == 60 {
				break
			}
		}
	}
	v = r.Snapshot()
	if v.Completed != 60 || len(seen) != 6 {
		t.Fatalf("incomplete radial service: completed=%d crossed=%d", v.Completed, len(seen))
	}
}

func checkCars(t *testing.T, v View) {
	t.Helper()
	if v.Generated != v.Waiting+v.Onboard+v.Completed {
		t.Fatal("passenger conservation")
	}
	for _, tr := range v.Trains {
		sum := 0
		for i, n := range tr.Cars {
			if n < 0 || n > tr.CarCapacities[i] {
				t.Fatal("car capacity", tr)
			}
			sum += n
		}
		if sum != tr.Passengers || tr.Length != float64(len(tr.Cars))*22.8 {
			t.Fatal("car manifest", tr)
		}
	}
}

func TestConsistChangesAtEmptyTerminal(t *testing.T) {
	r := fixture()
	for i := 0; i < 3; i++ {
		r.AddPassenger(0, 3)
	}
	for i := 0; i < 300; i++ {
		r.Advance(.05)
	}
	before := r.trains[0]
	if before.State != "moving" {
		t.Fatal("fixture did not depart")
	}
	c := r.config
	c.Cars = 4
	c.Capacity = 4
	if err := r.Configure(c); err != nil {
		t.Fatal(err)
	}
	if before.carCount != 6 || before.Length != 136.8 {
		t.Fatal("moving train resized")
	}
	for i := 0; i < 20000; i++ {
		r.Advance(.05)
		if i%20 == 0 {
			checkCars(t, r.Snapshot())
		}
		if before.carCount == 4 {
			break
		}
	}
	if before.carCount != 4 || before.Length != 91.2 || r.completed != 3 {
		t.Fatal("consist did not change after delivery")
	}
}

func TestMetro300KmhLimitAndStationBraking(t *testing.T) {
	c := DefaultConfig()
	c.Demand = 0
	c.Dwell = 5
	r := New([]Point{{0, 0}, {20000, 0}, {40000, 0}}, c)
	peak := 0.0
	previous := 0.0
	arrived := false
	for i := 0; i < 20000; i++ {
		r.Advance(.05)
		tr := r.trains[0]
		peak = math.Max(peak, tr.Speed)
		if tr.Speed > 300/3.6+1e-8 || math.Abs(tr.Speed-previous) > .05001 {
			t.Fatal("speed or acceleration limit", tr.Speed, previous)
		}
		previous = tr.Speed
		if tr.Station == 1 {
			arrived = true
			if tr.Speed != 0 || math.Abs(tr.Position.X-20000) > 1e-6 {
				t.Fatal("station overshoot")
			}
			break
		}
	}
	if !arrived || math.Abs(peak-300/3.6) > 1e-6 {
		t.Fatal("maximum speed not reached", peak)
	}
	c.MaxSpeed = 301
	if r.Configure(c) == nil {
		t.Fatal("invalid speed accepted")
	}
}

// Three intersecting lines share a same-level central throat lock.
func NewRadial(center Point, c Config) *Runtime {
	if c.Cars == 0 {
		c.Cars = 6
	}
	if c.MaxSpeed == 0 {
		c.MaxSpeed = 300
	}
	root := &Runtime{config: c}
	gate := &crossingGate{}
	for line := 0; line < 3; line++ {
		angle := float64(line) * math.Pi / 3
		points := []Point{}
		for i := -2; i <= 2; i++ {
			d := float64(i) * 2000
			points = append(points, Point{center.X + d*math.Cos(angle), center.Y + d*math.Sin(angle)})
		}
		local := c
		local.Demand = c.Demand / 3
		local.Seed += int64(line) * 101
		child := New(points, local)
		child.gate = gate
		child.lineID = line
		for i := range child.stations {
			child.stations[i].Name = fmt.Sprintf("L%d · %d km", line+1, (i-2)*2)
			if i == 2 {
				child.stations[i].Name = fmt.Sprintf("Central · L%d", line+1)
			}
		}
		root.lines = append(root.lines, child)
	}
	return root
}
