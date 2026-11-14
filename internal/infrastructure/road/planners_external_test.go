package model

import (
	"os/exec"
	"path/filepath"
	"testing"
	"traffic/internal/algorithms"
)

func TestExternalScenarioPlanners(t *testing.T) {
	python, e := exec.LookPath("python")
	if e != nil {
		t.Fatal(e)
	}
	dir, _ := filepath.Abs("../../../experiments/algorithms")
	for _, cap := range []string{"geometry", "phases"} {
		t.Run(cap, func(t *testing.T) {
			c, e := LoadConfig("../../../example/Map1/road.json")
			if e != nil {
				t.Fatal(e)
			}
			x := c.Intersections[1].Position.X
			id := t.Name()
			if e = algorithms.Register(algorithms.Manifest{ID: id, Capability: cap, AlgorithmVersion: "1", ContractVersion: 1, Command: []string{python, "extensions.py"}, Directory: dir, Timeout: 1500}); e != nil {
				t.Fatal(e)
			}
			if cap == "geometry" {
				c.GeometryAlgorithm = id
			} else {
				c.PhaseAlgorithm = id
			}
			g, e := BuildGraph(c)
			if e != nil {
				t.Fatal(e)
			}
			if g.Metro == nil {
				t.Fatal("planner lost metro")
			}
			if cap == "geometry" && g.Intersections[c.Intersections[1].ID].Position.X == x {
				t.Fatal("geometry not applied")
			}
			if c.Intersections[1].Position.X != x {
				t.Fatal("input mutated")
			}
		})
	}
}
