package sim

import (
	"context"
	"testing"
	"time"
	"traffic/internal/dispatch"
	"traffic/internal/model"
)

func taxiGraph(t *testing.T) *model.Graph {
	t.Helper()
	c, e := model.LoadConfig("../../configs/presets/four-cell-interchanges.json")
	if e != nil {
		t.Fatal(e)
	}
	c.Simulation.People = 1
	g, e := model.BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	return g
}
func TestTaxiServiceAndModeSwitch(t *testing.T) {
	s := New(taxiGraph(t))
	c := DefaultTaxiConfig()
	c.Fleet = 6
	c.Demand = 0
	c.MaxWait = 3600
	c.Dwell = .2
	if e := s.SetMode("taxi", c); e != nil {
		t.Fatal(e)
	}
	seen := map[string]bool{}
	for i := 0; i < 30000; i++ {
		s.Step(.05)
		for _, v := range s.taxi.vehicles {
			seen[v.State] = true
		}
		if s.taxi.completed >= 3 {
			break
		}
	}
	v := s.Snapshot()
	if v.Taxi.Completed < 3 {
		for _, x := range s.taxi.vehicles {
			t.Log(x.ID, x.State, x.p.wayID, x.p.along, x.p.waitingReason)
		}
		t.Fatalf("service did not complete: %+v", v.Taxi)
	}
	for _, state := range []string{"ToPickup", "Boarding", "WithPassenger", "Alighting", "Idle"} {
		if !seen[state] {
			t.Fatal("missing state", state)
		}
	}
	for _, o := range v.Taxi.Orders {
		if o.State == "Completed" && (o.Picked < o.Assigned || o.Completed <= o.Picked) {
			t.Fatal("bad lifecycle timestamps")
		}
	}
	s.Reset(3)
	if s.Snapshot().Mode != "taxi" || s.taxi.completed != 0 {
		t.Fatal("reset did not retain taxi configuration")
	}
	if e := s.SetMode("traffic", TaxiConfig{}); e != nil {
		t.Fatal(e)
	}
	if s.taxi != nil || s.Snapshot().Mode != "traffic" || len(s.people) != s.graph.Simulation.People {
		t.Fatal("traffic restoration failed")
	}
}
func TestTaxiCancellationAndDemandDeterminism(t *testing.T) {
	g := taxiGraph(t)
	a, b := New(g), New(g)
	c := DefaultTaxiConfig()
	c.Fleet = 1
	c.Demand = 0
	c.MaxWait = 10
	_ = a.SetMode("taxi", c)
	_ = b.SetMode("taxi", c)
	for i := 0; i < 30; i++ {
		a.createTaxiOrder()
		b.createTaxiOrder()
	}
	for i, o := range a.taxi.orders {
		z := b.taxi.orders[i]
		if o.Pickup != z.Pickup || o.Destination != z.Destination {
			t.Fatal("demand differs")
		}
	}
	o := a.taxi.orders[0]
	o.State = "Assigned"
	o.VehicleID = 1
	a.taxi.vehicles[0].OrderID = o.ID
	a.taxi.vehicles[0].State = "ToPickup"
	p := a.taxi.vehicles[0].p
	old := p.position
	a.cancelTaxiOrder(o)
	if o.State != "Cancelled" || a.taxi.vehicles[0].OrderID != 0 || p.position != old {
		t.Fatal("cancel moved taxi or retained order")
	}
	a.cancelTaxiOrder(o)
	if a.taxi.cancelled != 1 {
		t.Fatal("non-idempotent cancellation")
	}
}
func TestTaxiInterchangeMovements(t *testing.T) {
	g := taxiGraph(t)
	for _, group := range g.Interchanges {
		for _, r := range group.Routes {
			t.Run(group.ID+"/"+r.Origin+r.Destination, func(t *testing.T) {
				s := New(g)
				c := DefaultTaxiConfig()
				c.Fleet = 1
				c.Demand = 0
				_ = s.SetMode("taxi", c)
				s.taxi.orders = nil
				s.taxi.nextDispatch = 1e9
				v := s.taxi.vehicles[0]
				p := v.p
				first, last := g.Ways[r.WayIDs[0]], g.Ways[r.WayIDs[len(r.WayIDs)-1]]
				p.wayID = first.ID
				p.along = 25
				p.lane = s.laneForMovement(first.ID, r.WayIDs[1], 1)
				p.lanePosition = float64(p.lane)
				p.position, p.heading = first.Sample(p.along, p.lane)
				stop := TaxiStop{WayID: last.ID, Along: last.Length - 30}
				stop.Position, _ = last.Sample(stop.Along, 0)
				s.taxiSetPlan(v, taxiPlan{route: r.WayIDs, target: stop})
				v.State = "WithPassenger"
				seen := map[string]bool{}
				for i := 0; i < 16000 && v.State != "Alighting"; i++ {
					seen[p.wayID] = true
					s.Step(.05)
				}
				if v.State != "Alighting" {
					t.Fatalf("did not exit: %s %s %.1f %s", v.State, p.wayID, p.along, p.waitingReason)
				}
				for _, id := range r.WayIDs {
					if !seen[id] {
						t.Fatal("missed road", id)
					}
				}
			})
		}
	}
}

type slowTaxiPolicy struct{}

func (slowTaxiPolicy) Decide(ctx context.Context, in dispatch.Input) (dispatch.Proposal, error) {
	select {
	case <-ctx.Done():
		return dispatch.Empty(in), ctx.Err()
	case <-time.After(100 * time.Millisecond):
		return dispatch.Empty(in), nil
	}
}
func TestTaxiPendingPolicyDoesNotLockUIOrSurviveReset(t *testing.T) {
	_ = dispatch.Register(dispatch.Metadata{ID: "test.slow", Kind: "process"}, func() dispatch.Policy { return slowTaxiPolicy{} })
	s := New(taxiGraph(t))
	c := DefaultTaxiConfig()
	c.Fleet = 1
	c.Algorithm = "test.slow"
	_ = s.SetMode("taxi", c)
	s.Step(.05)
	if s.taxi.pending == nil {
		t.Fatal("no pending decision")
	}
	start := time.Now()
	if !s.Snapshot().Taxi.Solving {
		t.Fatal("missing solver status")
	}
	s.SetPaused(true)
	if time.Since(start) > 50*time.Millisecond {
		t.Fatal("UI blocked by algorithm")
	}
	_ = s.SetMode("traffic", TaxiConfig{})
	time.Sleep(120 * time.Millisecond)
	if s.taxi != nil {
		t.Fatal("stale result restored taxi")
	}
}
