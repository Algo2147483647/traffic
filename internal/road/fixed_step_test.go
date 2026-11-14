package sim

import (
	"reflect"
	"testing"
)

func TestFixedStepIndependentOfInputIntervalsAndPlayback(t *testing.T) {
	for _, mode := range []string{"traffic", "taxi"} {
		t.Run(mode, func(t *testing.T) {
			g := taxiGraph(t)
			a, b, c := New(g), New(g), New(g)
			defer a.Close()
			defer b.Close()
			defer c.Close()
			if mode == "taxi" {
				cfg := DefaultTaxiConfig()
				cfg.Fleet = 2
				cfg.Demand = 0
				for _, s := range []*Simulation{a, b, c} {
					if e := s.SetMode(mode, cfg); e != nil {
						t.Fatal(e)
					}
				}
			}
			a.Step(1)
			for i := 0; i < 100; i++ {
				b.Step(.01)
			}
			c.SetTimeScale(2)
			c.Step(.5)
			x, y, z := a.Snapshot(), b.Snapshot(), c.Snapshot()
			if x.Tick != 20 || y.Tick != 20 || z.Tick != 20 || x.Time != y.Time || x.Time != z.Time || !reflect.DeepEqual(x.People, y.People) || !reflect.DeepEqual(x.People, z.People) {
				t.Fatal("physical evolution depends on playback or input interval")
			}
		})
	}
}
