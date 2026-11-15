package simulation

import (
	"math"
	"testing"
	model "traffic/internal/infrastructure/road"
	"traffic/internal/operations/demand"
)

func TestWorldCoordinatesModesAndPreservesTasks(t *testing.T) {
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
	for i := 0; i < 400; i++ {
		w.Step(.05)
	}
	before := w.Snapshot()
	if math.Abs(before.Time-before.Metro.Time) > 1e-8 || before.Tick != 400 || before.Demand == nil || !before.Demand.Conserved {
		t.Fatalf("world time/accounting mismatch: time=%v metro=%v tick=%v demand=%+v", before.Time, before.Metro.Time, before.Tick, before.Demand)
	}
	mc := before.Metro.Config
	mc.Headway = 900
	mc.Service = nil
	if e = w.Execute(Command{Kind: "set_metro", Metro: mc}); e != nil {
		t.Fatal(e)
	}
	after := w.Snapshot()
	if after.Metro.Generated != before.Metro.Generated || !after.Demand.Conserved {
		t.Fatal("fleet configuration replaced passenger tasks")
	}
	w.Reset(10)
	reset := w.Snapshot()
	if reset.Time != 0 || reset.Metro.Time != 0 || reset.Tick != 0 {
		t.Fatal("world reset did not reset both clocks")
	}
}
