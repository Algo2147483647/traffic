package sim

import (
	"testing"
)

func TestTaxiSustainedService(t *testing.T) {
	s := New(taxiGraph(t))
	c := DefaultTaxiConfig()
	_ = s.SetMode("taxi", c)
	half := 0
	for i := 0; i < 12000; i++ {
		s.Step(.05)
		if i == 6000 {
			half = s.taxi.completed
		}
	}
	if s.taxi.completed < 20 || s.taxi.completed <= half {
		t.Fatalf("service stalled at lane-count transitions: first half=%d final=%d", half, s.taxi.completed)
	}
	for _, v := range s.taxi.vehicles {
		p := v.p
		if p.waitingSignal != "" && p.signalWait > 30 && s.graph.Intersections[p.waitingSignal].TrafficControl.Mode == "flow" {
			t.Fatalf("taxi %d stuck at continuous-flow node %s", v.ID, p.waitingSignal)
		}
	}
}
