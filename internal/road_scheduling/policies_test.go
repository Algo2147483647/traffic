package roadscheduling

import (
	"math"
	"testing"
	"traffic/internal/operations/taxi/dispatch"
)

func TestRoadProfileExtensions(t *testing.T) {
	if e := RegisterStrategies("test.slow", "1", func(p map[string]float64) Strategies {
		return Strategies{DesiredSpeed: func(v float64) float64 { return 5 }, RouteCost: func(r RoadCost) float64 { return 99 }}
	}); e != nil {
		t.Fatal(e)
	}
	s, e := NewWithPolicy(taxiGraph(t), "test.slow", nil)
	if e != nil {
		t.Fatal(e)
	}
	if s.policySpeed(3) != 3 || s.policySpeed(10) != 5 {
		t.Fatal("speed safety bound")
	}
	for _, w := range s.graph.Ways {
		if s.congestionWayCost(w, nil) != 99 {
			t.Fatal("custom route cost not used")
		}
		break
	}
	s.policies.DesiredSpeed = func(float64) float64 { return math.NaN() }
	if s.policySpeed(10) != 10 {
		t.Fatal("invalid speed accepted")
	}
}
func TestBusyTaxiProposalCannotBypassRestriction(t *testing.T) {
	s := New(taxiGraph(t))
	c := DefaultTaxiConfig()
	c.Fleet = 2
	_ = s.SetMode("taxi", c)
	v, o := s.taxi.vehicles[0], s.taxi.orders[0]
	v.State = "ToPickup"
	s.taxi.input = dispatch.Input{Version: 1, Epoch: s.taxiEpoch, Request: 1, Candidates: []dispatch.Candidate{{VehicleID: v.ID, OrderID: o.ID}}}
	p := dispatch.Empty(s.taxi.input)
	p.Assignments = []dispatch.Pair{{VehicleID: v.ID, OrderID: o.ID}}
	s.commitTaxiDecision(decisionResult{proposal: p})
	if o.State != "Waiting" || v.OrderID != 0 || s.taxi.failures != 1 {
		t.Fatal("busy vehicle received another assignment")
	}
}
