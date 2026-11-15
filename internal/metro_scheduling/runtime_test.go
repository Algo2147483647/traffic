package metroscheduling

import (
	"math"
	"testing"
)

func fixture() *Runtime {
	c := DefaultConfig()
	c.Demand = 0
	c.Capacity = 3
	c.Headway = 30
	c.Dwell = 5
	return New([]Point{{X: 0, Y: 0}, {X: 600, Y: 0}, {X: 1200, Y: 0}, {X: 1800, Y: 0}}, c)
}
func TestMetroServiceConservationAndInterlocking(t *testing.T) {
	r := fixture()
	for i := 0; i < 12; i++ {
		r.AddPassenger(0, 3)
		r.AddPassenger(3, 0)
	}
	sawFull, sawTail, sawReverse := false, false, false
	previous := map[int]float64{}
	for i := 0; i < 80000; i++ {
		r.Advance(.05)
		v := r.Snapshot()
		if v.Generated != v.Waiting+v.Onboard+v.Completed {
			t.Fatal("passenger conservation", v)
		}
		for _, tr := range r.trains {
			if len(tr.onboard) > tr.Capacity {
				t.Fatal("capacity exceeded")
			}
			sawFull = sawFull || len(tr.onboard) == tr.Capacity
			sawReverse = sawReverse || (tr.ID == 1 && tr.Direction < 0)
			if tr.State == "moving" {
				if tr.Speed > 300.0/3.6+1e-8 || tr.Distance > tr.Authority+1e-8 {
					t.Fatal("authority or speed violation")
				}
				if tr.Distance > r.lengths[tr.segment]-tr.Length && tr.Distance < r.lengths[tr.segment]-.01 {
					sawTail = true
					if r.segments[tr.segment] != tr.ID {
						t.Fatal("head exit released tail")
					}
				}
			}
			if old, ok := previous[tr.ID]; ok && math.Abs(tr.Speed-old) > .05001 {
				t.Fatal("unbounded acceleration", tr.Speed, old)
			}
			previous[tr.ID] = tr.Speed
		}
		for _, b := range v.Blocks {
			if b.Occupied != 0 && b.Owner != b.Occupied {
				t.Fatal("unreserved block occupancy")
			}
		}
		for a := 0; a < len(r.trains); a++ {
			for b := a + 1; b < len(r.trains); b++ {
				if r.trains[a].segment >= 0 && r.trains[a].segment == r.trains[b].segment {
					t.Fatal("opposing section entry")
				}
			}
		}
		if v.Completed == 24 {
			break
		}
	}
	v := r.Snapshot()
	if v.Completed != 24 || !sawFull || !sawTail || !sawReverse {
		t.Fatalf("incomplete service: completed=%d full=%v tail=%v reverse=%v", v.Completed, sawFull, sawTail, sawReverse)
	}
}
func TestFIFOAndCapacityPreserveWaitingIdentity(t *testing.T) {
	r := fixture()
	for i := 0; i < 8; i++ {
		r.AddPassenger(0, 3)
	}
	for i := 0; i < 110; i++ {
		r.Advance(.05)
	}
	tr := r.trains[0]
	if len(tr.onboard) != 3 || tr.onboard[0].ID != 1 || tr.onboard[2].ID != 3 || r.queues[0][0][0].ID != 4 {
		t.Fatal("FIFO or full train changed waiting identities")
	}
	c := r.config
	c.Capacity = 2
	if r.Configure(c) == nil {
		t.Fatal("capacity lowered below occupancy")
	}
}
func TestSnapshotDetachedAndDisabledClock(t *testing.T) {
	r := fixture()
	v := r.Snapshot()
	v.Stations[0].Name = "changed"
	if r.Snapshot().Stations[0].Name == "changed" {
		t.Fatal("snapshot alias")
	}
	c := r.config
	c.Enabled = false
	_ = r.Configure(c)
	r.Advance(10)
	if r.time != 10 || r.trains[0].Distance != 0 {
		t.Fatal("disabled service changed position or clock")
	}
}
