package model

import "testing"

func TestMetropolitanGridPresetStructure(t *testing.T) {
	config := MetropolitanGridConfig()
	graph, err := BuildGraph(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Intersections) != 121 {
		t.Fatalf("intersections = %d, want 121", len(graph.Intersections))
	}
	if len(graph.Ways) != 336 {
		t.Fatalf("directed ways = %d, want 336", len(graph.Ways))
	}
	if width := graph.Intersections["G04"].Position.X - graph.Intersections["G00"].Position.X; width != 16000 {
		t.Fatalf("core width = %.0f m, want 16000 m", width)
	}
	if height := graph.Intersections["G40"].Position.Y - graph.Intersections["G00"].Position.Y; height != 16000 {
		t.Fatalf("core height = %.0f m, want 16000 m", height)
	}

	classCount := map[string]int{}
	totalLanes := 0
	for _, way := range graph.Ways {
		classCount[way.RoadClass]++
		totalLanes += way.Lanes
		if way.RoadClass == "arterial" && way.Lanes != 4 {
			t.Errorf("arterial %s has %d lanes per direction, want 4", way.ID, way.Lanes)
		}
		if (way.RoadClass == "local" || way.RoadClass == "collector" || way.RoadClass == "ramp") && (way.From[0] == 'G' || way.To[0] == 'G') {
			t.Errorf("peripheral %s road %s connects directly to a core junction", way.RoadClass, way.ID)
		}
		for _, point := range way.Points {
			if point.X < 1000 || point.X > 17000 || point.Y < 1000 || point.Y > 17000 {
				t.Errorf("road %s expands beyond the 16 km core: %+v", way.ID, point)
			}
		}
	}
	if totalLanes != 864 {
		t.Fatalf("directed lane count = %d, want 864", totalLanes)
	}
	for _, class := range []string{"arterial", "ramp", "collector", "local"} {
		if classCount[class] == 0 {
			t.Errorf("preset has no %s roads", class)
		}
	}

	path, ok := graph.ShortestPath("A00", "B33")
	if !ok {
		t.Fatal("no north-to-south hierarchical route")
	}
	seen := map[string]bool{}
	for _, way := range path {
		seen[way.RoadClass] = true
	}
	for _, class := range []string{"local", "collector", "ramp", "arterial"} {
		if !seen[class] {
			t.Errorf("cross-city route never uses %s roads: %v", class, seen)
		}
	}
}
