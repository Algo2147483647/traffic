package sim

import (
	"container/heap"
	"math"
	"sort"

	"traffic/internal/model"
)

type laneRouteState struct {
	wayID string
	lane  int
}

type wayCongestionLoad struct {
	occupied int
	reserved int
}

type congestionQueueItem struct {
	state    laneRouteState
	distance float64
}

type congestionQueue []congestionQueueItem

func (queue congestionQueue) Len() int { return len(queue) }
func (queue congestionQueue) Less(i, j int) bool {
	if queue[i].distance == queue[j].distance {
		return laneRouteStateLess(queue[i].state, queue[j].state)
	}
	return queue[i].distance < queue[j].distance
}
func (queue congestionQueue) Swap(i, j int) { queue[i], queue[j] = queue[j], queue[i] }
func (queue *congestionQueue) Push(value any) {
	*queue = append(*queue, value.(congestionQueueItem))
}
func (queue *congestionQueue) Pop() any {
	old := *queue
	item := old[len(old)-1]
	*queue = old[:len(old)-1]
	return item
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
	return s.congestionRouteWithLoads(startWayID, finishWayID, blockedFirstWayID, startLane, s.currentCongestionLoads())
}

func (s *Simulation) congestionRouteWithLoads(startWayID, finishWayID, blockedFirstWayID string, startLane int, loads map[string]wayCongestionLoad) ([]string, []int, bool) {
	if s.graph.Ways[startWayID] == nil || s.graph.Ways[finishWayID] == nil || startWayID == finishWayID {
		return nil, nil, false
	}
	distance := make(map[laneRouteState]float64)
	queue := make(congestionQueue, 0, s.graph.Ways[startWayID].Lanes)
	if startLane >= 0 {
		state := laneRouteState{wayID: startWayID, lane: startLane}
		distance[state] = 0
		heap.Push(&queue, congestionQueueItem{state: state})
	} else {
		for lane := 0; lane < s.graph.Ways[startWayID].Lanes; lane++ {
			state := laneRouteState{wayID: startWayID, lane: lane}
			distance[state] = 0
			heap.Push(&queue, congestionQueueItem{state: state})
		}
	}
	previous := make(map[laneRouteState]laneRouteState)
	settled := make(map[laneRouteState]bool)
	goal, found := laneRouteState{}, false
	for queue.Len() > 0 {
		item := heap.Pop(&queue).(congestionQueueItem)
		current, best := item.state, item.distance
		known, exists := distance[current]
		if settled[current] || !exists || best > known+pressureEpsilon {
			continue
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
				candidate := best + s.congestionWayCost(next, loads)
				old, exists := distance[nextState]
				if !exists || candidate < old-pressureEpsilon ||
					(math.Abs(candidate-old) <= pressureEpsilon && laneRouteStateLess(current, previous[nextState])) {
					distance[nextState], previous[nextState] = candidate, current
					heap.Push(&queue, congestionQueueItem{state: nextState, distance: candidate})
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

func laneRouteStateLess(first, second laneRouteState) bool {
	return first.wayID < second.wayID || (first.wayID == second.wayID && first.lane < second.lane)
}

func (s *Simulation) currentCongestionLoads() map[string]wayCongestionLoad {
	loads := make(map[string]wayCongestionLoad, len(s.graph.Ways))
	for _, person := range s.people {
		s.addPersonCongestionLoad(loads, person)
	}
	return loads
}

func (s *Simulation) addPersonCongestionLoad(loads map[string]wayCongestionLoad, person *personState) {
	if loads == nil || person == nil {
		return
	}
	if person.wayID != "" && (person.state == "driving" || person.state == "waiting") {
		load := loads[person.wayID]
		load.occupied++
		loads[person.wayID] = load
	} else if person.state == "turning" && person.nextWayID != "" {
		load := loads[person.nextWayID]
		load.occupied++
		loads[person.nextWayID] = load
	}
	seen := make(map[string]struct{}, len(person.route))
	for index := person.routeIndex; index >= 0 && index < len(person.route); index++ {
		wayID := person.route[index]
		if _, exists := seen[wayID]; exists {
			continue
		}
		seen[wayID] = struct{}{}
		load := loads[wayID]
		load.reserved++
		loads[wayID] = load
	}
}

func (s *Simulation) congestionWayCost(way *model.Way, loads map[string]wayCongestionLoad) float64 {
	if way == nil {
		return math.Inf(1)
	}
	load := loads[way.ID]
	usable := math.Max(0, s.stopLine(way)-math.Min(s.graph.Simulation.Intersection, way.Length))
	perLaneCapacity := math.Max(1, math.Floor(usable/s.minimumStoppedVehicleSpacing())+1)
	capacity := perLaneCapacity * float64(way.Lanes)
	committed := math.Max(float64(load.occupied), float64(load.reserved))
	occupancy := math.Min(1, committed/capacity)
	freeFlow := way.Length / math.Max(1, way.SpeedLimit)
	return freeFlow * (1 + 12*occupancy*occupancy)
}
