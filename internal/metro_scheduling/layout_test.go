package metroscheduling

import (
	"math"
	"testing"
	track "traffic/internal/infrastructure/metro"
)

func TestExampleCircuitRunsAndLayerSeparation(t *testing.T) {
	l, e := track.LoadLayout("../testdata/signal-network/metro.json")
	if e != nil {
		t.Fatal(e)
	}
	c := l.Defaults
	c.Demand = 0
	c.Dwell = 5
	c.Headway = 30
	r := NewNetwork(l, c)
	for i := 0; i < 3; i++ {
		r.AddPassenger(i*5, i*5+4)
		r.AddPassenger(i*5+4, i*5)
	}
	loops := map[int]bool{}
	previous := map[int]Point{}
	parallel := false
	for step := 0; step < 100000; step++ {
		r.Advance(.05)
		owners := 0
		for _, line := range r.lines {
			if line.gate.owner != 0 {
				owners++
			}
			for _, tr := range line.trains {
				key := line.lineID*100 + tr.ID
				if tr.State == "turning" {
					loops[key] = true
				}
				if old, ok := previous[key]; ok && math.Hypot(tr.Position.X-old.X, tr.Position.Y-old.Y) > 300/3.6*.05+.001 {
					t.Fatal("train teleported", key, tr.State)
				}
				previous[key] = tr.Position
			}
		}
		parallel = parallel || owners > 1
		if step%100 == 0 {
			v := r.Snapshot()
			checkCars(t, v)
			for _, tr := range v.Trains {
				if len(tr.CarPoses) != len(tr.Cars) || tr.CarWidth != 3 || tr.CarLength != 22.8 {
					t.Fatal("physical car geometry missing")
				}
			}
			if len(loops) >= 6 && v.Completed == 6 {
				break
			}
		}
	}
	if len(loops) < 6 || r.Snapshot().Completed != 6 || !parallel {
		t.Fatal("incomplete loops/delivery/independent levels", len(loops), r.Snapshot().Completed, parallel)
	}
}
func TestSameLevelCrossingAndSafeReconfiguration(t *testing.T) {
	l, e := track.LoadLayout("../testdata/signal-network/metro.json")
	if e != nil {
		t.Fatal(e)
	}
	c := l.Defaults
	c.Demand = 0
	c.Dwell = 5
	c.LineLayers = map[string]int{"0": -1, "1": -1, "2": -1}
	r := NewNetwork(l, c)
	seen := map[int]bool{}
	for i := 0; i < 80000; i++ {
		r.Advance(.05)
		for _, line := range r.lines {
			for _, tr := range line.trains {
				if tr.Station == 2 || tr.Next == 2 && tr.State == "moving" {
					if tr.Station == 2 && tr.State == "moving" && tr.Distance >= tr.Length {
						continue
					}
					if line.gate.owner != line.lineID*100+tr.ID {
						t.Fatal("same-level crossing not reserved")
					}
					seen[line.lineID*100+tr.ID] = true
				}
			}
		}
		if len(seen) == 6 {
			break
		}
	}
	if len(seen) != 6 {
		t.Fatal("same-level starvation", seen)
	}
	c.LineLayers = map[string]int{"0": 0}
	if r.Configure(c) == nil {
		t.Fatal("non-underground metro accepted")
	}
}
func TestMalformedExampleRejected(t *testing.T) {
	l, e := track.LoadLayout("../testdata/signal-network/metro.json")
	if e != nil {
		t.Fatal(e)
	}
	l.Lines[0].Route[0].X++
	if l.Validate() == nil {
		t.Fatal("open terminal route accepted")
	}
}
