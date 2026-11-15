package roadscheduling

import (
	"testing"
	model "traffic/internal/infrastructure/road"
)

func TestTaxiOriginalMap(t *testing.T) {
	c, e := model.LoadConfig("../../example/Map1/road.json")
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
	// The 2 km ring creates longer real journeys than the previous compact map.
	for i := 0; i < 48000 && s.taxi.completed < 2; i++ {
		s.Step(.05)
	}
	if s.taxi.completed < 2 {
		t.Fatalf("original-map taxi service stalled: %+v", s.Snapshot().Taxi)
	}
}
