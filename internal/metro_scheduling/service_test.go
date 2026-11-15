package metroscheduling

import (
	"math"
	"testing"
	track "traffic/internal/infrastructure/metro"
)

func carsOverlap(a, b CarPose, length, width float64) bool {
	dx, dy := b.Position.X-a.Position.X, b.Position.Y-a.Position.Y
	if math.Hypot(dx, dy) > length+width {
		return false
	}
	axes := [][2]float64{{math.Cos(a.Heading), math.Sin(a.Heading)}, {-math.Sin(a.Heading), math.Cos(a.Heading)}, {math.Cos(b.Heading), math.Sin(b.Heading)}, {-math.Sin(b.Heading), math.Cos(b.Heading)}}
	for _, v := range axes {
		radius := func(h float64) float64 {
			return length/2*math.Abs(v[0]*math.Cos(h)+v[1]*math.Sin(h)) + width/2*math.Abs(-v[0]*math.Sin(h)+v[1]*math.Cos(h))
		}
		if math.Abs(dx*v[0]+dy*v[1]) >= radius(a.Heading)+radius(b.Heading)-.01 {
			return false
		}
	}
	return true
}
func TestPlannedFleetTimetableAndPassengerPreservation(t *testing.T) {
	l, e := track.LoadLayout("../../example/Map3/metro.json")
	if e != nil {
		t.Fatal(e)
	}
	c := l.Defaults
	c.Demand = 0
	c.Dwell = 5
	c.Service = &ServiceConfig{Algorithm: "cycle-time", Headway: 90}
	r := NewNetwork(l, c)
	r.UseSharedDemand()
	total := 0
	for _, line := range r.lines {
		total += len(line.trains)
	}
	if total <= 8 {
		t.Fatal("still fixed two trains per line")
	}
	for i, pair := range [][2]int{{2, 7}, {7, 12}, {12, 2}, {0, 14}, {4, 10}, {16, 19}} {
		if !r.AddJourney(pair[0], pair[1], i+1) {
			t.Fatal("admission")
		}
	}
	shrunk, grown, turning, depot, departed := false, false, false, false, false
	completed := map[int]bool{}
	for tick := 0; tick < 200000; tick++ {
		r.Advance(.05)
		if tick == 1000 {
			c.Service = &ServiceConfig{Algorithm: "cycle-time", Headway: 900}
			if e = r.Configure(c); e != nil {
				t.Fatal(e)
			}
			shrunk = true
		}
		if tick == 30000 {
			c.Service = &ServiceConfig{Algorithm: "cycle-time", Headway: 120}
			if e = r.Configure(c); e != nil {
				t.Fatal(e)
			}
			grown = true
		}
		for _, id := range r.DrainCompletedJourneys() {
			if completed[id] {
				t.Fatal("duplicate completion")
			}
			completed[id] = true
		}
		if tick%200 != 0 {
			continue
		}
		v := r.Snapshot()
		if v.Generated != v.Waiting+v.Onboard+v.Completed {
			t.Fatal("passenger conservation")
		}
		if len(r.ActiveJourneys())+len(completed) != 6 {
			t.Fatal("task identities lost")
		}
		for _, s := range v.Services {
			depot = depot || s.Depot > 0 || s.Withdrawn > 0
			departed = departed || s.Completed > 0
			for _, trip := range s.Trips {
				if trip.Actual > 0 && trip.Actual+1e-6 < trip.Planned {
					t.Fatal("early departure")
				}
			}
		}
		for i, a := range v.Trains {
			turning = turning || a.State == "turning"
			if a.State == "depot" || a.State == "withdrawn" {
				if a.Passengers != 0 {
					t.Fatal("passengers parked in depot")
				}
				continue
			}
			for _, b := range v.Trains[i+1:] {
				if a.Elevation != b.Elevation || b.State == "depot" || b.State == "withdrawn" {
					continue
				}
				for _, x := range a.CarPoses {
					for _, y := range b.CarPoses {
						if carsOverlap(x, y, a.CarLength, a.CarWidth) {
							t.Fatalf("train collision %d %s / %d %s at %.1f", a.ID, a.State, b.ID, b.State, v.Time)
						}
					}
				}
			}
		}
		if tick > 30000 && len(completed) == 6 && turning && departed {
			break
		}
	}
	if !shrunk || !grown || !turning || !depot || !departed || len(completed) != 6 {
		t.Fatalf("incomplete service: done=%d turn=%v depot=%v trips=%v", len(completed), turning, depot, departed)
	}
}
func TestServicePlannerRejectsUnsafePlanAtomically(t *testing.T) {
	id := "test-invalid-service"
	_ = RegisterServicePlanner(id, func(in ServiceInput) (ServicePlan, error) {
		return ServicePlan{Line: in.Line, Fleet: 200, Headway: 1}, nil
	})
	l, _ := track.LoadLayout("../../example/Map3/metro.json")
	r := NewNetwork(l, l.Defaults)
	c := l.Defaults
	c.Service = &ServiceConfig{Algorithm: id, Headway: 100}
	c.LineElevations = map[string]float64{"0": -99}
	if r.Configure(c) == nil || r.lines[0].elevation() != -50 {
		t.Fatal("unsafe planner mutated runtime")
	}
}
