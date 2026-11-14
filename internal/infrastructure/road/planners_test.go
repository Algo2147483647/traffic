package model

import "testing"

func TestPlannerRevalidatesOutput(t *testing.T) {
	c, e := LoadConfig("../../../configs/meridian-city.json")
	if e != nil {
		t.Fatal(e)
	}
	_ = RegisterPlanner("test.invalid", func(input CityConfig) (CityConfig, error) { input.Ways[0].To = "missing"; return input, nil })
	c.GeometryAlgorithm = "test.invalid"
	if _, e = BuildGraph(c); e == nil {
		t.Fatal("invalid algorithm output accepted")
	}
	if c.Ways[0].To == "missing" {
		t.Fatal("planner mutated original config")
	}
}
