package roadscheduling

import (
	"encoding/json"
	"fmt"
	"math"
	"traffic/internal/algorithms"
	"traffic/internal/operations/demand"
	"traffic/internal/operations/metro"
)

type extensionJob struct {
	cap     string
	choice  AlgorithmChoice
	payload any
}
type motionProposal struct {
	MergePriority float64 `json:"merge_priority,omitempty"`
	Entity        int     `json:"entity"`
	Speed         float64 `json:"speed_mps"`
	Lane          int     `json:"lane"`
}
type supplyProposal struct {
	Vehicle int    `json:"vehicle"`
	Stop    string `json:"stop"`
}
type demandBatch struct {
	Time  float64          `json:"time"`
	Tasks []demand.Journey `json:"tasks"`
	Stops []TaxiStop       `json:"stops"`
	Pairs [][2]int         `json:"metro_pairs"`
}

func external(c AlgorithmChoice) bool { return c.ID != "" && c.ID != "native" }
func (s *Simulation) extensionJobs() []extensionJob {
	c := s.algorithm.view.Config
	jobs := []extensionJob{}
	if external(c.Demand) && s.demandContext.Stream != nil {
		batch := demandBatch{Time: s.clock.Time, Stops: s.taxiStops(), Pairs: s.metro.ServedPairs()}
		for _, j := range s.demandContext.Stream.Journeys {
			if j.State == "queued" && j.Prepared && !j.Ready {
				batch.Tasks = append(batch.Tasks, *j)
			}
		}
		if len(batch.Tasks) > 0 {
			jobs = append(jobs, extensionJob{"demand", c.Demand, batch})
		}
	}
	if external(c.Motion) {
		tasks := []motionProposal{}
		context := []map[string]any{}
		for _, p := range s.people {
			if p.state == "driving" && !p.laneChanging {
				context = append(context, map[string]any{"entity": p.id, "way": p.wayID, "along_m": p.along, "speed_mps": p.speed, "lane": p.lane, "route": append([]string(nil), p.route...), "route_index": p.routeIndex})
				tasks = append(tasks, motionProposal{Entity: p.id, Speed: s.graph.Ways[p.wayID].SpeedLimit, Lane: p.lane})
			}
		}
		if len(tasks) > 0 {
			jobs = append(jobs, extensionJob{"motion", c.Motion, map[string]any{"tasks": tasks, "vehicles": context, "ways": s.graph.SortedWays()}})
		}
	}
	if external(c.Supply) && s.taxi != nil {
		idle := []int{}
		for _, v := range s.taxi.vehicles {
			if v.State == "Idle" && v.OrderID == 0 && !v.Retiring && v.p.state != "turning" && !v.p.laneChanging {
				idle = append(idle, v.ID)
			}
		}
		if len(idle) > 0 {
			jobs = append(jobs, extensionJob{"supply", c.Supply, map[string]any{"vehicles": idle, "stops": s.taxi.stops, "orders": s.taxiView().Orders}})
		}
	}
	if external(c.Metro) && len(s.metro.Snapshot().Trains) > 0 {
		jobs = append(jobs, extensionJob{"metro", c.Metro, s.metro.Snapshot()})
	}
	return jobs
}
func (s *Simulation) releaseDemandBatch(raw json.RawMessage) {
	var b demandBatch
	if json.Unmarshal(raw, &b) != nil || s.demandContext.Stream == nil {
		return
	}
	ids := map[int]bool{}
	for _, j := range b.Tasks {
		ids[j.ID] = true
	}
	for _, j := range s.demandContext.Stream.Journeys {
		if ids[j.ID] {
			j.Ready = true
		}
	}
}
func (s *Simulation) commitExtension(r algorithmResult) error {
	switch r.input.Capability {
	case "demand":
		var out struct {
			Journeys []demand.Journey `json:"journeys"`
		}
		if e := algorithms.Decode(r.response.Payload, &out); e != nil {
			return e
		}
		var in demandBatch
		_ = json.Unmarshal(r.input.Payload, &in)
		if len(out.Journeys) != len(in.Tasks) {
			return fmt.Errorf("incomplete demand batch")
		}
		original := map[int]demand.Journey{}
		for _, j := range in.Tasks {
			original[j.ID] = j
		}
		seen := map[int]bool{}
		for _, j := range out.Journeys {
			src, ok := original[j.ID]
			if math.IsNaN(j.DepartAt) || math.IsInf(j.DepartAt, 0) || j.DepartAt < src.DepartAt || j.DepartAt > in.Time+5 || !ok || seen[j.ID] || j.Mode != src.Mode || j.State != "queued" || j.Entity != 0 || j.Origin == j.Destination {
				return fmt.Errorf("invalid demand identity or mode")
			}
			seen[j.ID] = true
			valid := j.Origin >= 0 && j.Destination >= 0 && j.Origin < len(in.Stops) && j.Destination < len(in.Stops)
			if j.Mode == "metro" {
				valid = false
				for _, pair := range in.Pairs {
					if pair == [2]int{j.Origin, j.Destination} {
						valid = true
						break
					}
				}
			}
			if !valid {
				return fmt.Errorf("unserved demand OD")
			}
		}
		for _, v := range out.Journeys {
			for _, j := range s.demandContext.Stream.Journeys {
				if j.ID == v.ID && j.State == "queued" {
					j.Origin = v.Origin
					j.Destination = v.Destination
					j.DepartAt = v.DepartAt
					j.Ready = true
				}
			}
		}
	case "motion":
		var out struct {
			Motions []motionProposal `json:"motions"`
		}
		if e := algorithms.Decode(r.response.Payload, &out); e != nil {
			return e
		}
		seen := map[int]motionProposal{}
		people := map[int]*personState{}
		for _, p := range s.people {
			people[p.id] = p
		}
		for _, v := range out.Motions {
			p := people[v.Entity]
			_, dup := seen[v.Entity]
			if math.IsNaN(v.MergePriority) || math.IsInf(v.MergePriority, 0) || v.MergePriority < -1 || v.MergePriority > 1 || p == nil || dup || p.state != "driving" || math.IsNaN(v.Speed) || math.IsInf(v.Speed, 0) || v.Speed < 0 || v.Speed > s.graph.Ways[p.wayID].SpeedLimit || v.Lane < 0 || v.Lane >= s.graph.Ways[p.wayID].Lanes || absInt(v.Lane-p.lane) > 1 {
				return fmt.Errorf("invalid motion proposal")
			}
			seen[v.Entity] = v
		}
		s.algorithm.motion = seen
		s.algorithm.motionUntil = s.clock.Tick + 100
		for _, v := range out.Motions {
			p := people[v.Entity]
			target, distance, constrained := s.routeLaneTarget(p, s.graph.Ways[p.wayID])
			safeRoute := !constrained || v.Lane == target || distance > float64(absInt(v.Lane-target)+1)*(s.graph.Ways[p.wayID].SpeedLimit*laneChangeBaseDuration+s.minimumVehicleSpacing())
			if v.Lane != p.lane && !p.laneChanging && safeRoute && s.laneChangeSafe(p, v.Lane) {
				s.startLaneChange(p, s.graph.Ways[p.wayID], v.Lane, "algorithm")
			}
		}
	case "supply":
		var out struct {
			Targets []supplyProposal `json:"targets"`
		}
		if e := algorithms.Decode(r.response.Payload, &out); e != nil {
			return e
		}
		if s.taxi == nil {
			return fmt.Errorf("fleet unavailable")
		}
		seen := map[int]bool{}
		plans := map[int]taxiPlan{}
		vehicles := map[int]*TaxiVehicle{}
		for _, v := range s.taxi.vehicles {
			vehicles[v.ID] = v
		}
		for _, v := range out.Targets {
			car := vehicles[v.Vehicle]
			if car == nil || seen[v.Vehicle] || car.State != "Idle" || car.OrderID != 0 || car.Retiring || car.p.state == "turning" || car.p.laneChanging {
				return fmt.Errorf("taxi is not eligible")
			}
			seen[v.Vehicle] = true
			stop := s.taxi.stop(v.Stop)
			if stop.ID == "" {
				return fmt.Errorf("unknown stop")
			}
			plan, ok := s.taxiRoute(car.p, stop)
			if !ok {
				return fmt.Errorf("unreachable target")
			}
			plans[v.Vehicle] = plan
		}
		for _, v := range out.Targets {
			s.taxiSetPlan(vehicles[v.Vehicle], plans[v.Vehicle])
		}
	case "metro":
		var out struct {
			Trains []metro.Control `json:"trains"`
		}
		if e := algorithms.Decode(r.response.Payload, &out); e != nil {
			return e
		}
		return s.metro.ApplyControls(out.Trains)
	default:
		return fmt.Errorf("unsupported runtime capability")
	}
	return nil
}
