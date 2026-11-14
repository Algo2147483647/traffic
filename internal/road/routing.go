package sim

import (
	"math"

	"traffic/internal/infrastructure/interchange"
	model "traffic/internal/infrastructure/road"
)

type wayCongestionLoad struct {
	occupied int
	reserved int
}

// rerouteAroundCongestion replaces the remaining route when its immediate
// downstream way has no storage. Only the first blocked way is excluded; later
// edges receive a smooth occupancy penalty so traffic can use spare network
// capacity without oscillating between routes on every tick.
func (s *Simulation) rerouteAroundCongestion(person *personState, blockedWayID string) bool {
	if person == nil || person.routeIndex < 0 || person.routeIndex >= len(person.route)-1 {
		return false
	}
	plan, ok := s.congestionRoute(person.route[person.routeIndex], person.route[len(person.route)-1], blockedWayID, person.lane, person.along)
	if !ok || len(plan.Steps) < 2 || plan.Steps[1].WayID == blockedWayID {
		return false
	}
	path, exits, entries := plan.Arrays()
	prefix := append([]string(nil), person.route[:person.routeIndex]...)
	exitPrefix, entryPrefix := make([]int, person.routeIndex), make([]int, person.routeIndex)
	if len(person.routeLanes) >= person.routeIndex {
		copy(exitPrefix, person.routeLanes[:person.routeIndex])
	}
	if len(person.routeEntryLanes) >= person.routeIndex {
		copy(entryPrefix, person.routeEntryLanes[:person.routeIndex])
	}
	person.route = append(prefix, path...)
	person.routeLanes = append(exitPrefix, exits...)
	person.routeEntryLanes = append(entryPrefix, entries...)
	return true
}

func (s *Simulation) congestionRoute(start, finish, blocked string, lane int, along float64) (interchange.Route, bool) {
	return s.congestionRouteWithLoads(start, finish, blocked, lane, along, s.currentCongestionLoads())
}

func (s *Simulation) congestionRouteWithLoads(start, finish, blocked string, lane int, along float64, loads map[string]wayCongestionLoad) (interchange.Route, bool) {
	if s.router == nil {
		s.router = interchange.NewPlanner(s.graph)
	}
	// Freeze weights for both searches. Native experimental objectives participate
	// in macro transfer costs as well as ordinary roads.
	costs := make(map[string]float64, len(s.graph.Ways))
	for _, w := range s.graph.SortedWays() {
		costs[w.ID] = s.congestionWayCost(w, loads)
	}
	req := interchange.Request{Start: start, Finish: finish, StartLane: lane, BlockedFirst: blocked,
		Cost: func(w *model.Way) float64 { return costs[w.ID] }}
	// Prefer connector-continuous paths. When none exists, retain explicit entry
	// and exit lanes and reserve sufficient road length for physical preparation.
	if result, ok := s.router.Route(req); ok {
		return result, true
	}
	req.CanChange = func(w *model.Way, entry, exit int, initial bool) bool {
		begin := math.Min(s.graph.Simulation.Intersection, w.Length)
		if initial {
			begin = along
		}
		duration := math.Max(laneChangeBaseDuration, math.Min(2.2, w.LaneWidth/5))
		needed := float64(absInt(entry-exit)) * (w.SpeedLimit*duration + s.safeGap + s.graph.Simulation.VehicleSize)
		return s.stopLine(w)-begin >= needed
	}
	return s.router.Route(req)
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
	cost := freeFlow * (1 + 12*occupancy*occupancy)
	if s.policies.RouteCost != nil {
		v := s.policies.RouteCost(RoadCost{way.ID, way.Length, way.SpeedLimit, cost})
		if v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
			return v
		}
	}
	return cost
}
