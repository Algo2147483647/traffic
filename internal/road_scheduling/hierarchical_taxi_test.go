package roadscheduling

import (
	"testing"
)

// Exercise dispatch -> empty trip -> real pickup -> occupied trip -> dropoff.
// The routes are selected by the planner rather than injected from the catalogue.
func TestHierarchicalTaxiPickupAndDelivery(t *testing.T) {
	s := New(taxiGraph(t))
	defer s.Close()
	c := DefaultTaxiConfig()
	c.Fleet = 1
	c.Demand = 0
	c.MaxWait = 3600
	c.Dwell = .2
	if err := s.SetMode("taxi", c); err != nil {
		t.Fatal(err)
	}
	v := s.taxi.vehicles[0]
	p := v.p
	s.people = []*personState{p}
	w := s.graph.Ways["W_S_out__C_D0_in"]
	p.wayID = w.ID
	p.along = 25
	p.lane = w.Lanes - 1
	p.lanePosition = float64(p.lane)
	p.route = []string{w.ID}
	p.destinationAlong = p.along
	p.position, p.heading = w.Sample(p.along, p.lane)
	o := &TaxiOrder{ID: 1, Pickup: "stop:N_E_out__NE", Destination: "stop:S_E_out__SW", State: "Waiting"}
	s.taxi.orders = []*TaxiOrder{o}
	s.taxi.created = 1
	seenStates := map[string]bool{}
	pickupGroups, deliveryGroups := map[string]bool{}, map[string]bool{}
	ramp, deck := false, false
	for i := 0; i < 40000 && o.State != "Completed"; i++ {
		s.Step(.05)
		seenStates[v.State] = true
		if v.State == "ToPickup" || v.State == "WithPassenger" {
			if len(p.routeEntryLanes) != len(p.route) || len(p.routeLanes) != len(p.route) {
				t.Fatal("task has no expanded lanes")
			}
			way := s.graph.Ways[p.wayID]
			if way == nil {
				continue
			}
			group := s.graph.Intersections[way.To].Interchange
			if group != "" {
				if v.State == "ToPickup" {
					pickupGroups[group] = true
				} else {
					deliveryGroups[group] = true
				}
			}
			ramp = ramp || way.Structure == "ramp"
			deck = deck || way.Structure == "bridge"
			if v.OrderID != 1 {
				t.Fatal("accepted task was replaced")
			}
		}
	}
	if o.State != "Completed" {
		t.Fatalf("taxi did not finish: %s, %s at %s: %s", o.State, v.State, p.wayID, p.waitingReason)
	}
	for _, state := range []string{"ToPickup", "Boarding", "WithPassenger", "Alighting", "Idle"} {
		if !seenStates[state] {
			t.Fatal("missing service stage", state)
		}
	}
	if !pickupGroups["C"] || !pickupGroups["N"] || len(deliveryGroups) < 2 || !ramp || !deck {
		t.Fatal("service did not traverse required interchanges", pickupGroups, deliveryGroups, ramp, deck)
	}
	if s.taxi.picked != 1 || s.taxi.completed != 1 || s.taxi.cancelled != 0 || o.Completed <= o.Picked {
		t.Fatal("invalid lifecycle totals")
	}
	t.Logf("completed at %.1fs; pickup interchanges=%v, delivery interchanges=%v", o.Completed, pickupGroups, deliveryGroups)
}
