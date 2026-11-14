package sim

import (
	"testing"
	"traffic/internal/model"
)

func TestInterchangeRampTravel(t *testing.T) {
	c, e := model.LoadConfig("../../configs/presets/four-cell-interchanges.json")
	if e != nil {
		t.Fatal(e)
	}
	g, e := model.BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	s := New(g)
	ramp, deck, slope, loop := false, false, false, false
	for i := 0; i < 1600; i++ {
		s.Step(.05)
		if i%20 == 0 {
			for _, p := range s.Snapshot().People {
				if p.Elevation < 0 || p.Elevation > 24 {
					t.Fatal("invalid height")
				}
				if w := g.Ways[p.WayID]; w != nil && w.Role == "loop_ramp" {
					loop = true
				}
				ramp = ramp || p.Structure == "ramp"
				deck = deck || p.Structure == "bridge"
				slope = slope || (p.Structure == "ramp" && p.Elevation > 0 && p.Elevation < 24)
			}
		}
	}
	if !ramp || !deck || !slope || !loop {
		t.Fatalf("ramp=%v deck=%v slope=%v loop=%v", ramp, deck, slope, loop)
	}
}
