package model

import (
	"math"
	"testing"
)

func TestMeridianCityConfig(t *testing.T) {
	config, err := LoadConfig("../../../configs/meridian-city.json")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := BuildGraph(config)
	if err != nil {
		t.Fatal(err)
	}
	if graph.DrivingSide != DrivingSideRight || graph.MovementRules.ThroughToleranceDegrees != 40 {
		t.Fatalf("unexpected traffic conventions: side=%q rules=%+v", graph.DrivingSide, graph.MovementRules)
	}
	if len(graph.Intersections) != 21 {
		t.Fatalf("intersections = %d, want 21", len(graph.Intersections))
	}
	if len(graph.Ways) != 48 {
		t.Fatalf("directed ways = %d, want 48", len(graph.Ways))
	}
	totalWeight := 0
	for _, edge := range graph.Edges {
		totalWeight += edge.Weight
	}
	if totalWeight < 80 {
		t.Fatalf("lane-weight sum = %d, want a complex network", totalWeight)
	}
	for id, intersection := range graph.Intersections {
		if len(intersection.Approaches) < 2 {
			t.Errorf("intersection %s has only %d approaches", id, len(intersection.Approaches))
		}
		if len(graph.Incoming[id]) > 1 && len(intersection.TrafficControl.Phases) < 2 {
			t.Errorf("intersection %s has fewer than two signal phases", id)
		}
		if intersection.TrafficControl.CycleSeconds < 15 {
			t.Errorf("intersection %s has an invalid signal cycle", id)
		}
		if !intersection.TrafficControl.Generated || intersection.TrafficControl.PlanningBasis == "" {
			t.Errorf("intersection %s does not expose its generated planning basis", id)
		}
		seen := make(map[string]bool)
		for elapsed := 0.0; elapsed < intersection.TrafficControl.CycleSeconds; elapsed += .1 {
			seen[intersection.TrafficControl.Active(elapsed).Phase.ID] = true
		}
		if len(seen) != len(intersection.TrafficControl.Phases) {
			t.Errorf("intersection %s did not cycle through every phase: got %d of %d", id, len(seen), len(intersection.TrafficControl.Phases))
		}
		for _, incoming := range graph.Incoming[id] {
			allowed := false
			for _, phase := range intersection.TrafficControl.Phases {
				for _, wayID := range phase.AllowedIncoming {
					allowed = allowed || wayID == incoming.ID
				}
			}
			if !allowed {
				t.Errorf("intersection %s permanently blocks %s", id, incoming.ID)
			}
			for _, outgoing := range graph.Outgoing[id] {
				turn := classifyMovement(incoming, outgoing, graph.MovementRules)
				if turn != "u_turn" && !intersection.TrafficControl.HasMovement(incoming.ID, outgoing.ID) {
					t.Errorf("intersection %s omits feasible road movement %s -> %s", id, incoming.ID, outgoing.ID)
				}
			}
		}
		for _, incoming := range graph.Incoming[id] {
			coveredSourceLanes := make(map[int]bool)
			hasOrdinaryTarget := false
			for _, outgoing := range graph.Outgoing[id] {
				turn := classifyMovement(incoming, outgoing, graph.MovementRules)
				if turn == "u_turn" && len(graph.Outgoing[id]) > 1 {
					continue
				}
				hasOrdinaryTarget = hasOrdinaryTarget || turn != "u_turn"
				targets := make(map[int]bool)
				for fromLane := 0; fromLane < incoming.Lanes; fromLane++ {
					movement, ok := intersection.TrafficControl.Movement(incoming.ID, outgoing.ID, fromLane)
					if !ok {
						continue
					}
					coveredSourceLanes[fromLane] = true
					for _, target := range movement.TargetLanes(fromLane) {
						targets[target] = true
					}
				}
				if len(targets) != outgoing.Lanes {
					t.Errorf("intersection %s movement %s -> %s collectively reaches %d of %d target lanes", id, incoming.ID, outgoing.ID, len(targets), outgoing.Lanes)
				}
			}
			if hasOrdinaryTarget && len(coveredSourceLanes) != incoming.Lanes {
				t.Errorf("intersection %s assigns only %d of %d lanes on %s", id, len(coveredSourceLanes), incoming.Lanes, incoming.ID)
			}
		}
	}
	for from := range graph.Intersections {
		for to := range graph.Intersections {
			if _, ok := graph.ShortestPath(from, to); !ok {
				t.Errorf("no connector-valid route from %s to %s", from, to)
			}
		}
	}
	path, ok := graph.ShortestPath("NW1", "SE1")
	if !ok {
		t.Fatal("no cross-city hierarchical route")
	}
	seenClass := map[string]bool{}
	for _, way := range path {
		seenClass[way.RoadClass] = true
	}
	for _, class := range []string{"local", "collector", "ramp", "arterial"} {
		if !seenClass[class] {
			t.Errorf("cross-city route never uses %s roads: %v", class, seenClass)
		}
	}
	length := 0.0
	for _, way := range graph.Ways {
		length += way.Length
	}
	if average := length / float64(len(graph.Ways)); average < 400 || math.IsNaN(average) {
		t.Fatalf("average way length = %.1f, want at least 400", average)
	}
	if graph.Simulation.People != 320 || graph.Simulation.SpawnRate != 0 || graph.Simulation.MaxPeople < graph.Simulation.People {
		t.Fatalf("stable default-load configuration was not applied: %+v", graph.Simulation)
	}
}
