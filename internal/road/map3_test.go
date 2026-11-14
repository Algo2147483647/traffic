package sim

import (
	"testing"
	model "traffic/internal/infrastructure/road"
)

func TestMap3TaxiService(t *testing.T) {
	c, e := model.LoadConfig("../../example/Map3/road.json")
	if e != nil {
		t.Fatal(e)
	}
	c.Simulation.People = 1
	g, e := model.BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	s := New(g)
	defer s.Close()
	cfg := DefaultTaxiConfig()
	cfg.Fleet = 8
	cfg.Demand = 0
	cfg.MaxWait = 3600
	cfg.Dwell = .2
	if e = s.SetMode("taxi", cfg); e != nil {
		t.Fatal(e)
	}
	for tick := 0; tick < 100000 && s.taxi.completed < 2; tick++ {
		s.Step(.05)
	}
	if s.taxi.completed < 2 || !s.auditDemand().Conserved {
		t.Fatalf("Map3 service failed: completed=%d accounting=%+v", s.taxi.completed, s.auditDemand())
	}
}
