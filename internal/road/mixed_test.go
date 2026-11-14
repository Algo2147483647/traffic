package sim

import (
	"testing"
	"traffic/internal/operations/demand"
)

func TestMixedDemandPreservesTasksAndConservesTrips(t *testing.T) {
	g := taxiGraph(t)
	s := New(g)
	defer s.Close()
	s.people = nil
	s.maxPeople = 1000
	c := DefaultTaxiConfig()
	c.Fleet = 8
	c.Demand = 6
	c.Dwell = .2
	c.MaxWait = 3600
	if e := s.SetMode("mixed", c); e != nil {
		t.Fatal(e)
	}
	initialRun := s.stamp.Run
	carSeen, taxiSeen := false, false
	for step := 0; step < 24000; step++ {
		s.Step(.05)
		d := s.demandStream.Summary()
		if d.Generated != d.Queued+d.Active+d.Completed+d.Cancelled {
			t.Fatalf("accounting: %+v", d)
		}
		for _, p := range s.people {
			carSeen = carSeen || p.journeyID > 0
		}
		for _, v := range s.taxi.vehicles {
			taxiSeen = taxiSeen || v.State == "WithPassenger"
		}
		if step == 100 {
			now := s.clock.Time
			orders := map[int]int{}
			positions := map[int]float64{}
			for _, v := range s.taxi.vehicles {
				orders[v.ID] = v.OrderID
				positions[v.ID] = v.p.along
			}
			if e := s.Execute(Command{Kind: "set_demand", Demand: demand.Config{Rate: 6, TaxiShare: .25}}); e != nil {
				t.Fatal(e)
			}
			if s.clock.Time != now || s.stamp.Run != initialRun {
				t.Fatal("mix reset the run")
			}
			for _, v := range s.taxi.vehicles {
				if v.OrderID != orders[v.ID] || v.p.along != positions[v.ID] {
					t.Fatal("mix changed active task")
				}
			}
		}
		if d.Completed >= 8 && s.taxi.completed >= 2 && carSeen && taxiSeen {
			break
		}
	}
	d := s.demandStream.Summary()
	if !carSeen || !taxiSeen || s.taxi.completed < 2 || d.Completed <= s.taxi.completed {
		t.Fatalf("both modes did not finish: %+v taxi=%d", d, s.taxi.completed)
	}
}
