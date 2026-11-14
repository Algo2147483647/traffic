package model

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestMetropolitanGridPresetStructure(t *testing.T) {
	config, err := LoadConfig("testdata/atlas-metropolitan-grid.json")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := BuildGraph(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Intersections) != 81 {
		t.Fatalf("intersections = %d, want 81", len(graph.Intersections))
	}
	if len(graph.Ways) != 288 {
		t.Fatalf("directed ways = %d, want 288", len(graph.Ways))
	}
	if width := graph.Intersections["N00_08"].Position.X - graph.Intersections["N00_00"].Position.X; width != 16000 {
		t.Fatalf("grid width = %.0f m, want 16000 m", width)
	}
	if height := graph.Intersections["N08_00"].Position.Y - graph.Intersections["N00_00"].Position.Y; height != 16000 {
		t.Fatalf("grid height = %.0f m, want 16000 m", height)
	}

	classCount := map[string]int{}
	totalLanes := 0
	for _, way := range graph.Ways {
		classCount[way.RoadClass]++
		totalLanes += way.Lanes
		from := graph.Intersections[way.From].Position
		to := graph.Intersections[way.To].Position
		if distance := math.Hypot(to.X-from.X, to.Y-from.Y); distance != 2000 {
			t.Errorf("road %s length = %.0f m, want 2000 m", way.ID, distance)
		}

		line := int(from.Y / 2000)
		if from.X == to.X {
			line = int(from.X / 2000)
		}
		wantClass, wantLanes := "collector", 3
		if line%2 == 0 {
			wantClass, wantLanes = "arterial", 4
		}
		if way.RoadClass != wantClass || way.Lanes != wantLanes {
			t.Errorf("road %s on line %d = %s/%d lanes, want %s/%d", way.ID, line, way.RoadClass, way.Lanes, wantClass, wantLanes)
		}
	}

	if got := classCount["arterial"]; got != 160 {
		t.Fatalf("arterial directed ways = %d, want 160", got)
	}
	if got := classCount["collector"]; got != 128 {
		t.Fatalf("collector directed ways = %d, want 128", got)
	}
	if got := classCount["local"]; got != 0 {
		t.Fatalf("local directed ways = %d, want 0", got)
	}
	if totalLanes != 1024 {
		t.Fatalf("directed lane count = %d, want 1024", totalLanes)
	}

	path, ok := graph.ShortestPath("N00_00", "N08_08")
	if !ok || len(path) == 0 {
		t.Fatal("no route across the complete nested grid")
	}
}

func TestLoadPresetDirectoryUsesJSONFilenamesAsStableIDs(t *testing.T) {
	directory := t.TempDir()
	for name, contents := range map[string]string{
		"20-second.json": `{"name":"Second"}`,
		"10-first.JSON":  `{"name":"First"}`,
		"notes.txt":      "ignored",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	presets, err := LoadPresetDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(presets) != 2 {
		t.Fatalf("loaded presets = %d, want 2", len(presets))
	}
	if presets[0].ID != "10-first" || presets[0].Config.Name != "First" || presets[1].ID != "20-second" {
		t.Fatalf("unexpected preset order or identity: %+v", presets)
	}
}
