package roadscheduling

import (
	"reflect"
	"testing"
)

func TestMetroFixedStepIntegrationAndReset(t *testing.T) {
	a, b := New(taxiGraph(t)), New(taxiGraph(t))
	a.people = nil
	b.people = nil
	a.Step(200)
	for i := 0; i < 400; i++ {
		b.Step(.5)
	}
	x, y := a.Snapshot(), b.Snapshot()
	if !reflect.DeepEqual(x.Metro, y.Metro) || x.Metro.Time != x.Time || x.Metro.Departures == 0 {
		t.Fatal("metro clock or fixed-step integration failed")
	}
	c := x.Metro.Config
	c.Capacity = 500
	if err := a.Execute(Command{Kind: "set_metro", Metro: c}); err != nil {
		t.Fatal(err)
	}
	if a.clock.Time != x.Time {
		t.Fatal("settings reset world")
	}
	a.Reset(1)
	if a.metro.Snapshot().Time != 0 || a.metro.Snapshot().Config.Capacity != 500 {
		t.Fatal("metro reset lost settings")
	}
}
