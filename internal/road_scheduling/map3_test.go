package roadscheduling

import (
	"testing"
	model "traffic/internal/infrastructure/road"
	"traffic/internal/operations/demand"
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

func TestMap3TunnelVehiclePassage(t *testing.T) {
	c, e := model.LoadConfig("../../example/Map3/road.json")
	if e != nil {
		t.Fatal(e)
	}
	g, e := model.BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	for _, axis := range []string{"H", "V"} {
		for _, sign := range []string{"-1", "1"} {
			for _, reverse := range []bool{false, true} {
				s := New(g)
				s.spawnRate = 0
				s.people = nil
				ids := []string{"T_" + axis + "_" + sign + "_1", "T_" + axis + "_" + sign + "_2", "T_" + axis + "_" + sign + "_3", "T_" + axis + "_" + sign + "_4"}
				if reverse {
					for a, b := 0, len(ids)-1; a < b; a, b = a+1, b-1 {
						ids[a], ids[b] = ids[b], ids[a]
					}
				}
				route := []string{}
				for i := 0; i < len(ids)-1; i++ {
					route = append(route, ids[i]+"__"+ids[i+1])
				}
				first, last := g.Ways[route[0]], g.Ways[route[len(route)-1]]
				p := &personState{id: 1, state: "driving", wayID: first.ID, route: route, lane: 0, lanePosition: 0, along: 5, speed: 8, destinationAlong: last.Length - 20}
				s.people = []*personState{p}
				underground := false
				for i := 0; i < 8000 && p.routeIndex < len(route)-1; i++ {
					s.Step(.05)
					for _, v := range s.Snapshot().People {
						if v.Structure == "tunnel" && v.Elevation < 0 {
							underground = true
						}
					}
				}
				if p.routeIndex != len(route)-1 || !underground {
					t.Fatal("tunnel passage stalled", axis, sign, reverse, p.state, p.wayID)
				}
				s.Close()
			}
		}
	}
}

func TestMap3SharedTransferTasksConserved(t *testing.T) {
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
	s.people = nil
	mc := s.metro.Snapshot().Config
	mc.Demand = 0
	mc.Capacity = 12
	mc.Dwell = 5
	mc.Headway = 30
	s.metro.Configure(mc)
	if e := s.setDemand(demand.Config{Rate: 0, MetroShare: 1, Seed: 42}); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 24; i++ {
		s.demandContext.Stream.Add(len(s.taxiStops()))
	}
	walking := false
	for tick := 0; tick < 200000 && s.demandContext.Stream.Summary().Completed < 24; tick++ {
		s.Step(.05)
		if tick%100 == 0 {
			a := s.auditDemand()
			if !a.Conserved {
				t.Fatal(a.Errors)
			}
			walking = walking || s.metro.Snapshot().Transferring > 0
		}
	}
	if !walking || s.demandContext.Stream.Summary().Completed != 24 {
		t.Fatal("shared transfer journeys did not finish", s.demandContext.Stream.Summary())
	}
	s.Reset(1)
	if !s.auditDemand().Conserved || s.metro.Snapshot().Generated != 0 {
		t.Fatal("reset left transfer tasks")
	}
}
