package model

import (
	"math"
	"testing"
)

func TestFourCellInterchanges(t *testing.T) {
	c, err := LoadConfig("../../../configs/presets/four-cell-interchanges.json")
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
			if d := math.Abs(w.StartElevation - w.EndElevation); d != 24 && !(d == 0 && w.Role == "right_ramp") {
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
	c, err := LoadConfig("../../../configs/presets/four-cell-interchanges.json")
	if err != nil {
		t.Fatal(err)
	}
	c.Ways[0].Structure = "ground"
	if _, err = BuildGraph(c); err == nil {
		t.Fatal("height transition accepted without ramp")
	}
}

func TestTrumpetTopology(t *testing.T) {
	c, e := LoadConfig("../../../configs/presets/four-cell-interchanges.json")
	if e != nil {
		t.Fatal(e)
	}
	g, e := BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, group := range g.Interchanges {
		if group.Type != "trumpet" {
			continue
		}
		count++
		if len(group.Routes) != 6 {
			t.Fatal("trumpet needs six movements", group.ID)
		}
		roles := map[string]int{}
		for _, w := range g.Ways {
			if g.Intersections[w.From].Interchange == group.ID && g.Intersections[w.To].Interchange == group.ID {
				roles[w.Role]++
			}
		}
		if roles["loop_ramp"] != 1 || roles["semi_direct_ramp"] != 1 || roles["right_ramp"] != 2 {
			t.Fatal("invalid trumpet", group.ID, roles)
		}
		for _, r := range group.Routes {
			for i, id := range r.WayIDs {
				w := g.Ways[id]
				if i > 0 {
					a := g.Ways[r.WayIDs[i-1]]
					if a.To != w.From {
						t.Fatal("broken route", group.ID)
					}
					if g.Intersections[w.From].TrafficControl.Mode != "flow" {
						t.Fatal("signal in trumpet", group.ID)
					}
				}
			}
		}
	}
	if count != 4 {
		t.Fatal("expected four trumpets", count)
	}
	if g.WorldUnitsPerKM != 1000 {
		t.Fatal("missing map scale")
	}
	c.WorldUnitsPerKM = -1
	if _, e = BuildGraph(c); e == nil {
		t.Fatal("invalid map scale accepted")
	}
}
