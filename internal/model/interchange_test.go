package model

import (
	"math"
	"testing"
)

func TestFourCellInterchanges(t *testing.T) {
	c, err := LoadConfig("../../configs/presets/four-cell-interchanges.json")
	if err != nil {
		t.Fatal(err)
	}
	g, err := BuildGraph(c)
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string]bool{}
	bridges, ramps := 0, 0
	for _, n := range g.Intersections {
		if n.Interchange != "" {
			groups[n.Interchange] = true
		}
		if n.TrafficControl.Mode == "flow" {
			for _, at := range []float64{0, 3599, 3600, 7201} {
				if n.TrafficControl.Active(at).Stage != "green" {
					t.Fatal("flow node receives red", n.ID)
				}
			}
		}
	}
	for _, w := range g.Ways {
		if w.Structure == "bridge" {
			bridges++
			if w.StartElevation != 24 || w.EndElevation != 24 {
				t.Fatal("invalid deck")
			}
		}
		if w.Structure == "ramp" {
			ramps++
			if math.Abs(w.StartElevation-w.EndElevation) != 24 {
				t.Fatal("invalid ramp")
			}
		}
	}
	if len(groups) != 5 || bridges < 10 || ramps < 16 {
		t.Fatalf("groups=%d bridges=%d ramps=%d", len(groups), bridges, ramps)
	}
	// Full lane-valid reachability: every directed road can reach every other.
	for _, a := range g.SortedWays() {
		for _, b := range g.SortedWays() {
			if _, ok := g.RouteBetweenWays(a.ID, b.ID, true); !ok {
				t.Fatalf("no route %s -> %s", a.ID, b.ID)
			}
		}
	}
	// Pedestrian access cannot originate on the bridge above the ground road.
	if p := g.NearestLane(Vec2{X: 3000, Y: 3020}); p.Way == nil || p.Way.Structure != "ground" {
		t.Fatal("pedestrian access uses elevated road")
	}
}
func TestElevationRequiresRamp(t *testing.T) {
	c, err := LoadConfig("../../configs/presets/four-cell-interchanges.json")
	if err != nil {
		t.Fatal(err)
	}
	c.Ways[0].Structure = "ground"
	if _, err = BuildGraph(c); err == nil {
		t.Fatal("height transition accepted without ramp")
	}
}
