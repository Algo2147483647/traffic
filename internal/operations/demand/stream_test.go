package demand

import (
	"reflect"
	"testing"
)

func TestSeededSharedStreamAndConservation(t *testing.T) {
	c := Config{Rate: 20, TaxiShare: .5, Seed: 12}
	a, b := New(c, 0), New(c, 0)
	for i := 0; i < 100; i++ {
		a.Generate(float64(i), 8)
		b.Generate(float64(i), 8)
	}
	if !reflect.DeepEqual(a.Journeys, b.Journeys) {
		t.Fatal("seeded demand differs")
	}
	for _, j := range a.Journeys {
		j.State = "active"
		a.Finish(j.ID, j.ID%3 == 0)
	}
	a.Prune()
	s := a.Summary()
	if s.Generated != s.Completed+s.Cancelled || s.Cars == 0 || s.Taxis == 0 {
		t.Fatal(s)
	}
}

func TestThreeModeSeededProportions(t *testing.T) {
	a, b := New(Config{Rate: 1, TaxiShare: .3, MetroShare: .3, Seed: 85}, 0), New(Config{Rate: 1, TaxiShare: .3, MetroShare: .3, Seed: 85}, 0)
	for i := 0; i < 30000; i++ {
		x, y := a.Add(10), b.Add(10)
		if !reflect.DeepEqual(x, y) {
			t.Fatal("nondeterministic")
		}
	}
	v := a.Summary()
	for mode, want := range map[string]int{"car": 12000, "taxi": 9000, "metro": 9000} {
		got := v.Modes[mode].Generated
		if got < want-500 || got > want+500 {
			t.Fatalf("%s: %d", mode, got)
		}
	}
}
