package sim

import (
	"testing"
	"traffic/internal/model"
)

func TestTaxiOriginalMap(t *testing.T) {
	c, e := model.LoadConfig("../../configs/meridian-city.json")
	if e != nil {
		t.Fatal(e)
	}
	g, e := model.BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	s := New(g)
	cfg := DefaultTaxiConfig()
	cfg.Fleet = 6
	cfg.Demand = 0
	if e = s.SetMode("taxi", cfg); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 24000 && s.taxi.completed < 2; i++ {
		s.Step(.05)
	}
	if s.taxi.completed < 2 {
		t.Fatalf("original-map taxi service stalled: %+v", s.Snapshot().Taxi)
	}
}
