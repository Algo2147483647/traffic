package sim

import (
	"testing"
	model "traffic/internal/infrastructure/road"
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
	for i := 0; i < 8000; i++ {
		s.Step(.05)
		if i%20 == 0 {
			for _, p := range s.Snapshot().People {
				if p.Elevation < 0 || p.Elevation > 6.5 {
					t.Fatal("invalid height")
				}
				if w := g.Ways[p.WayID]; w != nil && w.Role == "loop_ramp" {
					loop = true
				}
				ramp = ramp || p.Structure == "ramp"
				deck = deck || p.Structure == "bridge"
				slope = slope || (p.Structure == "ramp" && p.Elevation > 0 && p.Elevation < 6.5)
			}
		}
	}
	if !ramp || !deck || !slope || !loop {
		t.Fatalf("ramp=%v deck=%v slope=%v loop=%v", ramp, deck, slope, loop)
	}
}

// Exercise each advertised movement independently so demand cannot hide an unused ramp.
func TestEveryInterchangeMovementDrivesThrough(t *testing.T) {
	c, e := model.LoadConfig("../../configs/presets/four-cell-interchanges.json")
	if e != nil {
		t.Fatal(e)
	}
	c.Simulation.People = 1
	c.Simulation.SpawnRate = 0
	g, e := model.BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	for _, group := range g.Interchanges {
		for _, route := range group.Routes {
			t.Run(group.ID+"/"+route.Origin+"_"+route.Destination, func(t *testing.T) {
				s := New(g)
				s.spawnRate = 0
				first, last := g.Ways[route.WayIDs[0]], g.Ways[route.WayIDs[len(route.WayIDs)-1]]
				lane := s.laneForMovement(first.ID, route.WayIDs[1], 1)
				p := &personState{id: 1, state: "driving", wayID: first.ID, route: route.WayIDs, lane: lane, lanePosition: float64(lane), along: 25, destinationAlong: last.Length - 30}
				p.position, p.heading = first.Sample(p.along, lane)
				p.exitPoint, _ = last.Sample(p.destinationAlong, 0)
				p.destination = p.exitPoint
				s.people = []*personState{p}
				seen := map[string]bool{}
				for step := 0; step < 12000; step++ {
					seen[p.wayID] = true
					if p.state == "walking_to_destination" || p.state == "arrived" {
						break
					}
					s.Step(.05)
				}
				for _, id := range route.WayIDs {
					if !seen[id] {
						t.Fatalf("did not traverse %s; stopped at %s: %s", id, p.wayID, p.waitingReason)
					}
				}
				if p.state != "walking_to_destination" && p.state != "arrived" {
					t.Fatalf("did not finish: %s %.1f %s", p.wayID, p.along, p.waitingReason)
				}
			})
		}
	}
}
