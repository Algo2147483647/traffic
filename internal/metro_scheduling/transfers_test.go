package metroscheduling

import (
	"reflect"
	"testing"
	track "traffic/internal/infrastructure/metro"
)

func TestRadialMap1RealTransfersAndConservation(t *testing.T) {
	l, e := track.LoadLayout("../../example/Map1/metro.json")
	if e != nil {
		t.Fatal(e)
	}
	if len(l.Transfers) != 6 {
		t.Fatal("six exchanges required")
	}
	c := l.Defaults
	c.Demand = 0
	c.Dwell = 5
	c.Headway = 30
	c.Capacity = 3
	r := NewNetwork(l, c)
	r.UseSharedDemand()
	expected := 0
	add := func(a, b int) {
		expected++
		if !r.AddJourney(a, b, expected) {
			t.Fatal("unserved journey", a, b)
		}
	}
	for i, x := range l.Transfers {
		a := r.globalStation(x.A)
		other := r.globalStation([2]int{3, (i + 1) % 6})
		add(a, other)
		add(other, a)
	}
	add(2, 7)
	add(7, 12)
	add(12, 2)
	if len(r.PlanJourney(2, 7)) < 5 {
		t.Fatal("missing multi-line itinerary")
	}
	done := map[int]bool{}
	walking := false
	for tick := 0; tick < 240000 && len(done) < expected; tick++ {
		r.Advance(.05)
		for _, id := range r.DrainCompletedJourneys() {
			if done[id] {
				t.Fatal("duplicate completion")
			}
			done[id] = true
		}
		if tick%100 == 0 {
			v := r.Snapshot()
			for _, j := range v.Journeys {
				if j.State == "onboard" {
					found := false
					for _, tr := range v.Trains {
						if tr.ID == j.Train && j.Car >= 0 && j.Car < len(tr.CarPoses) {
							found = true
							if j.Position != tr.CarPoses[j.Car].Position || j.Elevation != tr.Elevation {
								t.Fatal("passenger not on assigned car")
							}
						}
					}
					if !found {
						t.Fatal("missing passenger carrier")
					}
				}
			}
			walking = walking || v.Transferring > 0
			if v.Generated != v.Waiting+v.Onboard+v.Completed {
				t.Fatalf("passengers lost: generated=%d waiting=%d aboard=%d completed=%d", v.Generated, v.Waiting, v.Onboard, v.Completed)
			}
			active := r.ActiveJourneys()
			if len(active)+len(done) != expected {
				t.Fatal("journey identities lost")
			}
			for _, n := range active {
				if n != 1 {
					t.Fatal("duplicate active passenger")
				}
			}
			for _, tr := range v.Trains {
				if tr.Passengers > tr.Capacity {
					t.Fatal("capacity bypassed")
				}
			}
		}
	}
	if len(done) != expected || !walking || r.transferCount < 18 {
		t.Fatalf("transfers incomplete: %d/%d transfers=%d", len(done), expected, r.transferCount)
	}
}
func TestTransferWalkingPauseAndPlanIsolation(t *testing.T) {
	l, e := track.LoadLayout("../../example/Map1/metro.json")
	if e != nil {
		t.Fatal(e)
	}
	c := l.Defaults
	c.Demand = 0
	r := NewNetwork(l, c)
	x := l.Transfers[0]
	a, b := r.globalStation(x.A), r.globalStation(x.B)
	if !r.AddJourney(a, b, 7) {
		t.Fatal("transfer-only admission")
	}
	before := r.PlanJourney(a, b)
	copy := r.PlanJourney(a, b)
	copy[0].To = -1
	if !reflect.DeepEqual(before, r.PlanJourney(a, b)) {
		t.Fatal("mutable itinerary")
	}
	c.Enabled = false
	if e = r.Configure(c); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2000; i++ {
		r.Advance(.05)
	}
	if len(r.DrainCompletedJourneys()) > 0 {
		t.Fatal("paused transfer completed")
	}
	c.Enabled = true
	r.Configure(c)
	for i := 0; i < 1300; i++ {
		r.Advance(.05)
	}
	if r.completed != 1 {
		t.Fatal("transfer never resumed")
	}
}
