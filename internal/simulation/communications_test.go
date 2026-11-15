package simulation

import (
	"math"
	"testing"
	"traffic/internal/foundation"
	infra "traffic/internal/infrastructure/communications"
	model "traffic/internal/infrastructure/road"
	"traffic/internal/operations/demand"
	rail "traffic/internal/operations/metro"
	road "traffic/internal/road_scheduling"
)

func TestRadioCarrierBinding(t *testing.T) {
	s := road.Snapshot{Kernel: foundation.Stamp{Run: 3, Topology: 2}, RadioLayout: &infra.Layout{Profile: infra.Profile{ReceiverHeight: 1.5}}, Metro: rail.View{Journeys: []rail.NetworkJourney{{ID: 10, Train: 101, Car: 2, Position: rail.Point{X: 20, Y: 30}, Elevation: -55, Speed: 5}}}}
	u := mobility(s)
	if len(u) != 1 || u[0].ID != "3:2:metro-passenger:10" || u[0].Carrier != "3:2:train:101/car:2" || u[0].Position.Z != -53.5 || !u[0].Underground {
		t.Fatal("carrier binding", u)
	}
	s.Metro.Journeys[0].Train = 0
	s.Metro.Journeys[0].State = "transferring"
	if mobility(s)[0].ID != u[0].ID {
		t.Fatal("transfer changed identity")
	}
}
func TestMap1RadioObservationsAndMapSwitch(t *testing.T) {
	c, e := model.LoadConfig("../../example/Map1/road.json")
	if e != nil {
		t.Fatal(e)
	}
	g, e := model.BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	w := New(g)
	defer w.Close()
	if e = w.Execute(Command{Kind: "set_demand", Demand: demand.Config{Rate: 60, TaxiShare: .3, MetroShare: .4, Seed: 71429}}); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 100; i++ {
		w.Step(.05)
	}
	v := w.Snapshot()
	r := v.Communications
	if r == nil || len(r.Cells) != 2100 || r.Connected+r.Outage+r.Unsupported != len(r.Terminals) || math.Abs(float64(r.TimeUS)/1e6-v.Time) > 1e-6 || !v.Demand.Conserved {
		t.Fatal("radio/world boundary")
	}
	seen := map[string]bool{}
	for _, u := range r.Terminals {
		if seen[u.ID] {
			t.Fatal("duplicate UE", u.ID)
		}
		seen[u.ID] = true
	}
	if v.Metro.Generated != v.Metro.Waiting+v.Metro.Onboard+v.Metro.Completed {
		t.Fatal("passengers changed")
	}
	old := r.Terminals[0].ID
	w.Reset(10)
	reset := w.Snapshot()
	for _, u := range reset.Communications.Terminals {
		if u.ID == old {
			t.Fatal("reset reused UE identity")
		}
	}
	presets, e := model.LoadPresetDirectory("../../example")
	if e != nil || len(presets) != 2 {
		t.Fatalf("preset migration: %v / %d", e, len(presets))
	}
	for _, p := range presets {
		if p.ID == "radial-grid-cloverleaf" {
			t.Fatal("old map ID remains")
		}
	}
	c2, e := model.LoadConfig("../../example/Map2/road.json")
	if e != nil {
		t.Fatal(e)
	}
	g2, e := model.BuildGraph(c2)
	if e != nil {
		t.Fatal(e)
	}
	w.ReplaceGraph(g2)
	if w.Snapshot().Communications != nil {
		t.Fatal("radio leaked into another map")
	}
}
