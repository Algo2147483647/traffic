package metroscheduling

import (
	"fmt"
	"math"
	"sort"
	"sync"
)

type ServiceInput struct {
	Line                    int
	Name                    string
	Circular                bool
	Stations                int
	Cycle, Minimum, Headway float64
}
type ServicePlanner func(ServiceInput) (ServicePlan, error)

var serviceMu sync.RWMutex
var servicePlanners = map[string]ServicePlanner{"cycle-time": func(in ServiceInput) (ServicePlan, error) {
	h := math.Max(in.Headway, in.Minimum)
	limit := min(98, max(2, 2*(in.Stations-1)))
	fleet := int(math.Ceil(in.Cycle / h))
	if in.Circular {
		fleet *= 2
	}
	fleet = max(2, fleet)
	if fleet%2 != 0 {
		fleet++
	}
	if fleet > limit {
		fleet = limit
		h = math.Max(h, in.Cycle/float64(fleet))
		if in.Circular {
			h = math.Max(h, 2*in.Cycle/float64(fleet))
		}
	}
	return ServicePlan{Line: in.Line, Name: in.Name, Fleet: fleet, Headway: h, Requested: in.Headway, Cycle: in.Cycle, Minimum: in.Minimum, Limit: limit}, nil
}}

func RegisterServicePlanner(id string, p ServicePlanner) error {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	if id == "" || p == nil || servicePlanners[id] != nil {
		return fmt.Errorf("invalid or duplicate service planner")
	}
	servicePlanners[id] = p
	return nil
}
func ServicePlannerIDs() []string {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	ids := []string{}
	for id := range servicePlanners {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

type serviceRuntime struct {
	last                        [2]float64
	plan                        ServicePlan
	next                        [2]float64
	trips                       []*ServiceTrip
	serial, departed, completed int
}

func (r *Runtime) servicePlans(c Config) ([]ServicePlan, error) {
	id := "cycle-time"
	headway := c.Headway
	if c.Service != nil {
		if c.Service.Algorithm != "" {
			id = c.Service.Algorithm
		}
		if c.Service.Headway > 0 {
			headway = c.Service.Headway
		}
		if math.IsNaN(c.Service.Headway) || math.IsInf(c.Service.Headway, 0) || c.Service.Headway < 0 {
			return nil, fmt.Errorf("invalid service headway")
		}
	}
	serviceMu.RLock()
	planner := servicePlanners[id]
	serviceMu.RUnlock()
	if planner == nil {
		return nil, fmt.Errorf("unknown metro service planner %s", id)
	}
	plans := []ServicePlan{}
	for _, line := range r.lines {
		cycle, minimum := 0.0, 0.0
		for i, d := range line.lengths {
			if (line.circular() && i >= len(line.stations)) || (!line.circular() && line.spec != nil && i >= len(line.stations)-1) {
				break
			}
			run := travelEstimate(d, 0, c.MaxSpeed/3.6) + c.Dwell
			cycle += run
			minimum = math.Max(minimum, run)
		}
		if !line.circular() {
			cycle = 2 * cycle
			if line.spec != nil {
				total := line.chain[len(line.chain)-1]
				for _, st := range []int{0, len(line.stations) - 1} {
					loop := math.Abs(line.spec.Stops[st][1] - line.spec.Stops[st][0])
					loop = math.Min(loop, total-loop)
					cycle += loop / (line.spec.LoopSpeed / 3.6)
				}
			}
		}
		name := fmt.Sprintf("Line %d", line.lineID+1)
		if line.spec != nil {
			name = line.spec.Name
		}
		in := ServiceInput{line.lineID, name, line.circular(), len(line.stations), cycle, minimum, headway}
		p, err := planner(in)
		if err != nil {
			return nil, err
		}
		if p.Line != in.Line || p.Fleet < 2 || p.Fleet > min(98, 2*(in.Stations-1)) || p.Fleet%2 != 0 || math.IsNaN(p.Headway) || math.IsInf(p.Headway, 0) || p.Headway < minimum {
			return nil, fmt.Errorf("unsafe service plan for line %d", in.Line)
		}
		plans = append(plans, p)
	}
	return plans, nil
}
func (r *Runtime) applyServicePlans(plans []ServicePlan) {
	for i, p := range plans {
		line := r.lines[i]
		if line.service == nil {
			line.service = &serviceRuntime{plan: p, next: [2]float64{line.time, line.time}, last: [2]float64{math.Inf(-1), math.Inf(-1)}}
			line.trains = nil
			for j := range line.platforms {
				line.platforms[j] = [2]int{}
			}
		} else {
			line.service.plan = p
			for d := range line.service.next {
				line.service.next[d] = math.Max(line.time, line.service.next[d])
			}
		}
		for len(line.trains) < p.Fleet {
			n := len(line.trains)
			st, dir := 0, 1
			if n%2 == 1 {
				dir = -1
				if !line.circular() {
					st = len(line.stations) - 1
				}
			}
			t := &train{TrainView: TrainView{ID: n + 1, Station: st, Next: st + dir, Direction: dir, State: "depot", Length: float64(line.config.Cars) * line.carLength(), Capacity: line.config.Capacity}, carCount: line.config.Cars, segment: -1}
			if line.circular() {
				t.Next = (t.Next + len(line.stations)) % len(line.stations)
			}
			t.routeDistance = line.spec.Stops[st][directionIndex(dir)]
			t.Position = line.directionPose(dir, t.routeDistance).Position
			line.trains = append(line.trains, t)
		}
		for j, t := range line.trains {
			t.retiring = j >= p.Fleet
			if t.retiring && t.State == "depot" {
				t.State = "withdrawn"
			}
			if !t.retiring && t.State == "withdrawn" {
				t.State = "depot"
			}
		}
	}
}
func (r *Runtime) depotStation(dir int) int {
	if r.circular() || dir > 0 {
		return 0
	}
	return len(r.stations) - 1
}
func (r *Runtime) prepareServices() {
	s := r.service
	if s == nil {
		return
	}
	for d, dir := range []int{1, -1} {
		pending := false
		for _, trip := range s.trips {
			if trip.Direction == dir && trip.State == "queued" {
				pending = true
				break
			}
		}
		if !pending && r.time >= s.next[d] {
			s.serial++
			s.trips = append(s.trips, &ServiceTrip{ID: s.serial, Direction: dir, Origin: r.depotStation(dir), Planned: s.next[d], State: "queued"})
			s.next[d] += s.plan.Headway
		}
	}
	for _, trip := range s.trips {
		if trip.State != "queued" {
			continue
		}
		st, dir := trip.Origin, trip.Direction
		p := r.platform(st, dir)
		if r.platforms[st][p] != 0 {
			continue
		}
		for _, t := range r.trains {
			if t.State != "depot" || t.retiring || t.Station != st || (r.circular() && t.Direction != dir) {
				continue
			}
			t.Direction = dir
			t.Next = st + dir
			if r.circular() {
				t.Next = (t.Next + len(r.stations)) % len(r.stations)
			}
			t.State = "dwelling"
			t.arrived = r.time
			t.scheduled = math.Max(math.Max(r.time+r.config.Dwell, trip.Planned), s.last[directionIndex(dir)]+s.plan.Headway)
			t.service = 0
			t.needsLoop = false
			t.trip = trip.ID
			t.tripOrigin = st
			t.tripStops = 0
			t.tripFinishing = false
			t.routeDistance = r.spec.Stops[st][directionIndex(dir)]
			t.Position = r.directionPose(dir, t.routeDistance).Position
			r.platforms[st][p] = t.ID
			trip.Train = t.ID
			trip.State = "boarding"
			break
		}
	}
	// Keep active and queued jobs plus a bounded display history.
	if len(s.trips) > 160 {
		keep := []*ServiceTrip{}
		for i, t := range s.trips {
			if t.State != "completed" || i >= len(s.trips)-120 {
				keep = append(keep, t)
			}
		}
		s.trips = keep
	}
}
func (r *Runtime) serviceDeparture(t *train) {
	if r.service == nil || t.trip == 0 {
		return
	}
	for _, trip := range r.service.trips {
		if trip.ID == t.trip && trip.State == "boarding" {
			trip.State = "running"
			trip.Actual = r.time
			r.service.departed++
			r.service.last[directionIndex(trip.Direction)] = r.time
			d := directionIndex(trip.Direction)
			r.service.next[d] = math.Max(r.service.next[d], r.time+r.service.plan.Headway)
			break
		}
	}
}
func (r *Runtime) serviceArrival(t *train) {
	if r.service == nil {
		return
	}
	t.tripStops++
	if (!r.circular() && (t.Station == 0 || t.Station == len(r.stations)-1)) || (r.circular() && t.tripStops >= len(r.stations) && t.Station == t.tripOrigin) {
		t.tripFinishing = true
	}
}
func (r *Runtime) parkService(t *train) bool {
	if r.service == nil || !t.tripFinishing || len(t.onboard) > 0 || t.needsLoop || t.State == "turning" || t.State == "moving" || t.Station != r.depotStation(t.Direction) {
		return false
	}
	for _, trip := range r.service.trips {
		if trip.ID == t.trip {
			trip.State = "completed"
			trip.Finished = r.time
			r.service.completed++
			break
		}
	}
	for i := range r.platforms {
		for d := range r.platforms[i] {
			if r.platforms[i][d] == t.ID {
				r.platforms[i][d] = 0
			}
		}
		if r.routes[i] == t.ID {
			r.routes[i] = 0
		}
	}
	t.trip = 0
	t.State = "depot"
	if t.retiring {
		t.State = "withdrawn"
	}
	t.Speed = 0
	t.Passengers = 0
	return true
}
func (r *Runtime) serviceView() ServiceView {
	s := r.service
	v := ServiceView{Plan: s.plan, Departed: s.departed, Completed: s.completed, Depots: []int{0}}
	if !r.circular() {
		v.Depots = append(v.Depots, len(r.stations)-1)
	}
	for _, t := range r.trains {
		switch t.State {
		case "depot":
			v.Depot++
		case "withdrawn":
			v.Withdrawn++
		default:
			v.Active++
			if t.retiring {
				v.Retiring++
			}
		}
	}
	for _, trip := range s.trips {
		x := *trip
		if x.Train > 0 {
			x.Train += r.lineID * 100
		}
		v.Trips = append(v.Trips, x)
	}
	return v
}
