package sim

import (
	"math"
	"testing"
)

func TestTaxiLiveLoadPreservesTrips(t *testing.T) {
	s := New(taxiGraph(t))
	c := DefaultTaxiConfig()
	c.Fleet = 6
	c.Demand = 2
	_ = s.SetMode("taxi", c)
	s.Step(.05)
	beforeTime, created := s.time, s.taxi.created
	positions := map[int]float64{}
	orders := map[int]int{}
	for _, v := range s.taxi.vehicles {
		positions[v.ID] = v.p.along
		orders[v.ID] = v.OrderID
	}
	if err := s.SetTaxiLoad(9, 0); err != nil {
		t.Fatal(err)
	}
	if s.time != beforeTime || s.taxi.created != created || len(s.taxi.vehicles) != 9 || !math.IsInf(s.taxi.nextArrival, 1) {
		t.Fatal("live load reset state or did not apply")
	}
	for _, v := range s.taxi.vehicles {
		if old, ok := positions[v.ID]; ok && (old != v.p.along || orders[v.ID] != v.OrderID) {
			t.Fatal("active taxi changed")
		}
	}
	if err := s.SetTaxiLoad(3, 0); err != nil {
		t.Fatal(err)
	}
	if len(s.taxi.vehicles) < 6 {
		t.Fatal("busy taxis removed before finishing")
	}
	for _, v := range s.taxi.vehicles {
		if v.ID > 3 && !v.Retiring {
			t.Fatal("excess taxi not scheduled to retire")
		}
	}
	for i := 0; i < 30000 && len(s.taxi.vehicles) > 3; i++ {
		s.Step(.05)
	}
	if len(s.taxi.vehicles) != 3 {
		t.Fatal("retiring taxis did not finish", len(s.taxi.vehicles))
	}
	if s.taxi.created != created {
		t.Fatal("zero demand generated new orders")
	}
	if s.taxi.completed == 0 {
		t.Fatal("retirement interrupted all service")
	}
	if err := s.SetTaxiLoad(3, 30); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 400; i++ {
		s.Step(.05)
	}
	if s.taxi.created <= created {
		t.Fatal("new demand frequency not active")
	}
	if err := s.SetTaxiLoad(0, 2); err == nil {
		t.Fatal("invalid fleet accepted")
	}
}
