package metroscheduling

import (
	"math"
	"testing"
	track "traffic/internal/infrastructure/metro"
)

func TestPhysicalElevationConfiguration(t *testing.T) {
	l, err := track.LoadLayout("../../example/Map1/metro.json")
	if err != nil {
		t.Fatal(err)
	}
	r := NewNetwork(l, l.Defaults)
	v := r.Snapshot()
	if v.Lines[0].Elevation != -50 || v.Lines[3].Elevation != -65 {
		t.Fatal("legacy migration", v.Lines)
	}
	c := l.Defaults
	c.LineElevations = map[string]float64{"0": -70.25, "1": -70.25, "2": -90}
	c.Headway = 15
	c.MaxSpeed = 350
	c.Demand = 150
	c.Dwell = 2
	c.Cars = 13
	c.Capacity = 1500
	if err = r.Configure(c); err != nil {
		t.Fatal(err)
	}
	if r.lines[0].gate != r.lines[1].gate || r.lines[0].gate == r.lines[2].gate {
		t.Fatal("physical elevation crossing lock not respected")
	}
	if r.Snapshot().Stations[0].Elevation != -70.25 {
		t.Fatal("snapshot lost precise elevation")
	}
	c.LineElevations = map[string]float64{"0": math.NaN()}
	if r.Configure(c) == nil {
		t.Fatal("nonfinite elevation accepted")
	}
}
