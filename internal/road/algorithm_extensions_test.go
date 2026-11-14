package sim

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
	"traffic/internal/algorithms"
	"traffic/internal/operations/demand"
)

func TestLiveExtensionWorkers(t *testing.T) {
	python, e := exec.LookPath("python")
	if e != nil {
		t.Fatal("Python example runtime required")
	}
	dir, _ := filepath.Abs("../../experiments/algorithms")
	for _, cap := range []string{"demand", "motion", "metro", "supply"} {
		t.Run(cap, func(t *testing.T) {
			s := New(taxiGraph(t))
			defer s.Close()
			s.people = nil
			cfg := DefaultTaxiConfig()
			cfg.Fleet = 4
			s.startTaxiLive(cfg)
			if e := s.setDemand(demand.Config{Rate: 0, TaxiShare: .3, MetroShare: .3, Seed: 11}); e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 12; i++ {
				s.demandStream.Add(len(s.taxiStops()))
			}
			choice := AlgorithmChoice{ID: t.Name()}
			if e := algorithms.Register(algorithms.Manifest{ID: choice.ID, Capability: cap, AlgorithmVersion: "1", ContractVersion: 1, Command: []string{python, "extensions.py"}, Directory: dir, Timeout: 1500}); e != nil {
				t.Fatal(e)
			}
			var c AlgorithmConfig
			switch cap {
			case "demand":
				c.Demand = choice
			case "motion":
				c.Motion = choice
			case "metro":
				c.Metro = choice
			case "supply":
				c.Supply = choice
			}
			if e := s.setAlgorithms(c); e != nil {
				t.Fatal(e)
			}
			s.advanceSharedDemand()
			finishAlgorithms(t, s)
			if s.algorithm.view.Applied != 1 || s.algorithm.view.Fallbacks != 0 {
				t.Fatalf("not applied: %+v", s.algorithm.view)
			}
			s.advanceSharedDemand()
			if !s.auditDemand().Conserved {
				t.Fatal(s.auditDemand())
			}
			if cap == "motion" && len(s.algorithm.motion) == 0 {
				t.Fatal("no actual motion controls")
			}
		})
	}
}

func TestExtensionsRejectUnsafeProposalsAtomically(t *testing.T) {
	s := New(taxiGraph(t))
	defer s.Close()
	s.people = nil
	cfg := DefaultTaxiConfig()
	cfg.Fleet = 2
	s.startTaxiLive(cfg)
	vehicle := s.taxi.vehicles[0]
	vehicle.OrderID = 99
	vehicle.State = "ToPickup"
	for _, test := range []struct {
		cap     string
		payload any
	}{
		{"supply", map[string]any{"targets": []supplyProposal{{vehicle.ID, s.taxi.stops[0].ID}}}},
		{"metro", map[string]any{"trains": []map[string]any{{"train": s.metro.Snapshot().Trains[0].ID, "speed_mps": 1000, "depart_after_s": 0}}}},
		{"motion", map[string]any{"motions": []motionProposal{{Entity: -1, Speed: 1, Lane: 0}}}},
	} {
		raw, _ := json.Marshal(test.payload)
		if e := s.commitExtension(algorithmResult{input: algorithms.Request{Capability: test.cap}, response: algorithms.Response{Payload: raw}}); e == nil {
			t.Fatalf("accepted unsafe %s", test.cap)
		}
	}
	if vehicle.OrderID != 99 || vehicle.State != "ToPickup" {
		t.Fatal("busy taxi changed")
	}
}

func TestExtensionFailuresUseNativeFallback(t *testing.T) {
	for _, cap := range []string{"demand", "motion", "metro", "supply"} {
		for _, failure := range []string{"timeout", "crash", "malformed", "stale"} {
			t.Run(cap+"/"+failure, func(t *testing.T) {
				s := New(taxiGraph(t))
				defer s.Close()
				s.people = nil
				c := DefaultTaxiConfig()
				c.Fleet = 2
				s.startTaxiLive(c)
				if e := s.setDemand(demand.Config{TaxiShare: .3, MetroShare: .3, Seed: 11}); e != nil {
					t.Fatal(e)
				}
				for i := 0; i < 10; i++ {
					s.demandStream.Add(len(s.taxiStops()))
				}
				choice := AlgorithmChoice{ID: registerAlgorithmHelper(t, cap, failure)}
				var config AlgorithmConfig
				switch cap {
				case "demand":
					config.Demand = choice
				case "motion":
					config.Motion = choice
				case "metro":
					config.Metro = choice
				case "supply":
					config.Supply = choice
				}
				if e := s.setAlgorithms(config); e != nil {
					t.Fatal(e)
				}
				s.advanceSharedDemand()
				finishAlgorithms(t, s)
				if s.algorithm.view.Fallbacks != 1 {
					t.Fatalf("missing fallback: %+v", s.algorithm.view)
				}
				s.advanceSharedDemand()
				if !s.auditDemand().Conserved {
					t.Fatal(s.auditDemand())
				}
			})
		}
	}
}
