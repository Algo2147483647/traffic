package metroscheduling

import (
	"math"
	"testing"
	track "traffic/internal/infrastructure/metro"
)

func TestMap3CircularLineCompletesBothDirections(t *testing.T) {
	l, e := track.LoadLayout("../../example/Map3/metro.json")
	if e != nil {
		t.Fatal(e)
	}
	c := l.Defaults
	c.Demand = 0
	c.Dwell = 5
	r := NewNetwork(l, c)
	line := r.lines[3]
	for a := 0; a < 6; a++ {
		for b := 0; b < 6; b++ {
			if a != b {
				line.AddJourney(a, b, a*6+b+1)
			}
		}
	}
	before := map[int]Point{}
	visits := map[int]map[int]bool{1: {}, 2: {}}
	for tick := 0; tick < 80000; tick++ {
		r.Advance(.05)
		for _, tr := range line.trains {
			if p, ok := before[tr.ID]; ok && math.Hypot(tr.Position.X-p.X, tr.Position.Y-p.Y) > 300/3.6*.05+.001 {
				t.Fatal("ring teleport")
			}
			before[tr.ID] = tr.Position
			if visits[tr.ID] == nil {
				visits[tr.ID] = map[int]bool{}
			}
			visits[tr.ID][tr.Station] = true
			if (tr.ID == 1 && tr.Direction != 1) || (tr.ID == 2 && tr.Direction != -1) {
				t.Fatal("ring reversed train")
			}
		}
	}
	if line.completed != 30 || len(line.ActiveJourneys()) != 0 {
		t.Fatalf("ring passengers: %+v", line.Snapshot().Completed)
	}
	for _, v := range visits {
		if len(v) != 6 {
			t.Fatal("incomplete orbit")
		}
	}
	for _, s := range l.Lines[3].Stations {
		if math.Abs(math.Hypot(s.X, s.Y)-4096) > .001 {
			t.Fatal("ring station radius")
		}
		matches := 0
		for _, axis := range l.Lines[:3] {
			for _, p := range axis.Stations {
				if math.Hypot(s.X-p.X, s.Y-p.Y) < .001 {
					matches++
				}
			}
		}
		if matches != 1 {
			t.Fatal("ring station not co-located")
		}
	}
}
