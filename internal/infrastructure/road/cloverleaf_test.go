package model

import "testing"

func TestCloverleafMovements(t *testing.T) {
	c, e := LoadConfig("../../../configs/presets/four-cell-interchanges.json")
	if e != nil {
		t.Fatal(e)
	}
	g, e := BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	if len(g.Interchanges) != 5 || len(g.Interchanges[0].Routes) != 12 {
		t.Fatal("missing interchange route catalogue")
	}
	loops, rights := 0, 0
	for _, w := range g.Ways {
		if g.Intersections[w.From].Interchange != "C" {
			continue
		}
		if w.Role == "loop_ramp" {
			loops++
			if len(w.Points) < 80 || w.Lanes != 1 {
				t.Fatal("loop geometry missing")
			}
		}
		if w.Role == "right_ramp" {
			rights++
		}
		if w.Role != "" && w.TripAccess {
			t.Fatal("limited-access road permits pedestrian access")
		}
	}
	if loops != 4 || rights != 4 {
		t.Fatalf("loops=%d rights=%d", loops, rights)
	}
	for _, route := range g.Interchanges[0].Routes {
		laneSet := map[int]bool{}
		first := g.Ways[route.WayIDs[0]]
		for i := 0; i < first.Lanes; i++ {
			laneSet[i] = true
		}
		for i := 1; i < len(route.WayIDs); i++ {
			a, b := g.Ways[route.WayIDs[i-1]], g.Ways[route.WayIDs[i]]
			next := map[int]bool{}
			control := g.Intersections[a.To].TrafficControl
			if control.Mode != "flow" {
				t.Fatal("cloverleaf route crosses signals")
			}
			// Lane changes are allowed between nodes. At least one explicit connector
			// must connect each consecutive pair, and all receiving indices are valid.
			for _, phase := range control.Phases {
				for _, m := range phase.AllowedMovements {
					if m.FromWayID == a.ID && m.ToWayID == b.ID {
						for _, conn := range m.LaneConnections {
							if conn.FromLane < 0 || conn.FromLane >= a.Lanes || conn.ToLane < 0 || conn.ToLane >= b.Lanes {
								t.Fatal("invalid lane")
							}
							next[conn.ToLane] = true
						}
					}
				}
			}
			if len(next) == 0 {
				t.Fatalf("broken %s -> %s movement", route.Origin, route.Destination)
			}
			laneSet = next
			if route.Turn == "through" && b.Role != "mainline" {
				t.Fatal("through traffic forced into C/D")
			}
		}
		_ = laneSet
	}
}
func TestRejectMalformedFlowConnections(t *testing.T) {
	c, e := LoadConfig("../../../configs/presets/four-cell-interchanges.json")
	if e != nil {
		t.Fatal(e)
	}
	for i := range c.Intersections {
		if len(c.Intersections[i].Traffic.Connections) > 0 {
			c.Intersections[i].Traffic.Connections[0].ToLanes = []int{99}
			break
		}
	}
	if _, e = BuildGraph(c); e == nil {
		t.Fatal("invalid connector accepted")
	}
}
