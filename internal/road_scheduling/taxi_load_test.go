package roadscheduling

import (
	"testing"
)

func TestTaxiSustainedService(t *testing.T) {
	s := New(taxiGraph(t))
	c := DefaultTaxiConfig()
	// Fixed stress workload, independent of interactive defaults.
	c.Fleet, c.Demand, c.Dwell = 16, 8, 4
	// Preserve offered trips per free-flow journey after km/h-to-m/s calibration.
	c.Demand /= 3.6
	c.MaxWait *= 3.6
	_ = s.SetMode("taxi", c)
	half := 0
	for i := 0; i < 43200; i++ {
		s.Step(.05)
		if i == 21600 {
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
