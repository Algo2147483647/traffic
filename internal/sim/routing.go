package sim

import (
	"math"
	"sort"

	"traffic/internal/model"
)

type laneRouteState struct {
	wayID string
	lane  int
}

// rerouteAroundCongestion replaces the remaining route when its immediate
// downstream way has no storage. Only the first blocked way is excluded; later
// edges receive a smooth occupancy penalty so traffic can use spare network
// capacity without oscillating between routes on every tick.
func (s *Simulation) rerouteAroundCongestion(person *personState, blockedWayID string) bool {
	if person == nil || person.routeIndex < 0 || person.routeIndex >= len(person.route)-1 {
		return false
	}
	currentWayID := person.route[person.routeIndex]
	finishWayID := person.route[len(person.route)-1]
	path, lanes, ok := s.congestionRoute(currentWayID, finishWayID, blockedWayID, person.lane)
	if !ok || len(path) < 2 || path[1] == blockedWayID {
		return false
	}
	prefix := append([]string(nil), person.route[:person.routeIndex]...)
	lanePrefix := make([]int, person.routeIndex)
	if len(person.routeLanes) >= person.routeIndex {
		copy(lanePrefix, person.routeLanes[:person.routeIndex])
	}
	person.route = append(prefix, path...)
	person.routeLanes = append(lanePrefix, lanes...)
	return true
}

func (s *Simulation) congestionRoute(startWayID, finishWayID, blockedFirstWayID string, startLane int) ([]string, []int, bool) {
	if s.graph.Ways[startWayID] == nil || s.graph.Ways[finishWayID] == nil || startWayID == finishWayID {
		return nil, nil, false
	}
	distance := make(map[laneRouteState]float64)
	if startLane >= 0 {
		distance[laneRouteState{wayID: startWayID, lane: startLane}] = 0
	} else {
		for lane := 0; lane < s.graph.Ways[startWayID].Lanes; lane++ {
			distance[laneRouteState{wayID: startWayID, lane: lane}] = 0
		}
	}
	previous := make(map[laneRouteState]laneRouteState)
	settled := make(map[laneRouteState]bool)
	goal, found := laneRouteState{}, false
	for {
		current, best, ok := nearestCongestionState(distance, settled)
		if !ok || math.IsInf(best, 1) {
			break
		}
		settled[current] = true
		if current.wayID == finishWayID {
			goal, found = current, true
			break
		}
		from := s.graph.Ways[current.wayID]
		intersection := s.graph.Intersections[from.To]
		if intersection == nil {
			continue
		}
		outgoing := append([]*model.Way(nil), s.graph.Outgoing[from.To]...)
		sort.Slice(outgoing, func(i, j int) bool { return outgoing[i].ID < outgoing[j].ID })
		for _, next := range outgoing {
			if current.wayID == startWayID && next.ID == blockedFirstWayID {
				continue
			}
			movement, allowed := intersection.TrafficControl.Movement(from.ID, next.ID, current.lane)
			if !allowed {
				continue
			}
			for _, connection := range movement.ConnectionsFrom(current.lane) {
				nextState := laneRouteState{wayID: next.ID, lane: connection.ToLane}
				candidate := best + s.congestionWayCost(next)
				old, exists := distance[nextState]
				if !exists || candidate < old-pressureEpsilon ||
					(math.Abs(candidate-old) <= pressureEpsilon && laneRouteStateLess(current, previous[nextState])) {
					distance[nextState], previous[nextState] = candidate, current
				}
			}
		}
	}
	if !found {
		return nil, nil, false
	}
	var reversed []laneRouteState
	for cursor := goal; ; {
		reversed = append(reversed, cursor)
		prior, ok := previous[cursor]
		if !ok {
			break
		}
		cursor = prior
	}
	if reversed[len(reversed)-1].wayID != startWayID {
		return nil, nil, false
	}
	path, lanes := make([]string, len(reversed)), make([]int, len(reversed))
	for index := range reversed {
		state := reversed[index]
		path[len(reversed)-1-index] = state.wayID
		lanes[len(reversed)-1-index] = state.lane
	}
	return path, lanes, true
}

func nearestCongestionState(distance map[laneRouteState]float64, settled map[laneRouteState]bool) (laneRouteState, float64, bool) {
	current, best, found := laneRouteState{}, math.Inf(1), false
	for state, candidate := range distance {
		if settled[state] || candidate > best+pressureEpsilon ||
			(math.Abs(candidate-best) <= pressureEpsilon && found && !laneRouteStateLess(state, current)) {
			continue
		}
		current, best, found = state, candidate, true
	}
	return current, best, found
}

func laneRouteStateLess(first, second laneRouteState) bool {
	return first.wayID < second.wayID || (first.wayID == second.wayID && first.lane < second.lane)
}

func (s *Simulation) congestionWayCost(way *model.Way) float64 {
	if way == nil {
		return math.Inf(1)
	}
	occupied, reserved := 0, 0
	for _, person := range s.people {
		if person.wayID == way.ID && (person.state == "driving" || person.state == "waiting") {
			occupied++
		} else if person.state == "turning" && person.nextWayID == way.ID {
			occupied++
		}
		for index := person.routeIndex; index >= 0 && index < len(person.route); index++ {
			if person.route[index] == way.ID {
				reserved++
				break
			}
		}
	}
	usable := math.Max(0, s.stopLine(way)-math.Min(s.graph.Simulation.Intersection, way.Length))
	perLaneCapacity := math.Max(1, math.Floor(usable/s.minimumStoppedVehicleSpacing())+1)
	capacity := perLaneCapacity * float64(way.Lanes)
	committed := math.Max(float64(occupied), float64(reserved))
	occupancy := math.Min(1, committed/capacity)
	freeFlow := way.Length / math.Max(1, way.SpeedLimit)
	return freeFlow * (1 + 12*occupancy*occupancy)
}
