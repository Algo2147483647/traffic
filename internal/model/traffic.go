package model

import (
	"math"
	"sort"
)

type wayRouteState struct {
	wayID    string
	departed bool
}

type ActivePhase struct {
	Index     int
	Phase     TrafficPhaseConfig
	Stage     string
	Remaining float64
}

func (control TrafficControl) Active(simulationTime float64) ActivePhase {
	if len(control.Phases) == 0 || control.CycleSeconds <= 0 {
		return ActivePhase{Index: -1}
	}
	position := math.Mod(simulationTime+control.OffsetSeconds, control.CycleSeconds)
	if position < 0 {
		position += control.CycleSeconds
	}
	elapsed := 0.0
	for index, phase := range control.Phases {
		greenEnd := elapsed + phase.DurationSeconds
		if position < greenEnd {
			return ActivePhase{Index: index, Phase: phase, Stage: "green", Remaining: greenEnd - position}
		}
		amberEnd := greenEnd + phase.AmberSeconds
		if position < amberEnd {
			return ActivePhase{Index: index, Phase: phase, Stage: "amber", Remaining: amberEnd - position}
		}
		redEnd := amberEnd + phase.AllRedSeconds
		if position < redEnd {
			return ActivePhase{Index: index, Phase: phase, Stage: "all_red", Remaining: redEnd - position}
		}
		elapsed = redEnd
	}
	last := control.Phases[len(control.Phases)-1]
	return ActivePhase{Index: len(control.Phases) - 1, Phase: last, Stage: "all_red", Remaining: last.AllRedSeconds}
}

func (control TrafficControl) Allows(incomingWayID string, simulationTime float64) bool {
	active := control.Active(simulationTime)
	if active.Index < 0 {
		return true
	}
	if active.Stage != "green" {
		return false
	}
	for _, wayID := range active.Phase.AllowedIncoming {
		if wayID == incomingWayID {
			return true
		}
	}
	return false
}

func (control TrafficControl) AllowsMovement(fromWayID, toWayID string, lane int, simulationTime float64) bool {
	active := control.Active(simulationTime)
	if active.Index < 0 {
		return true
	}
	if active.Stage != "green" {
		return false
	}
	for _, movement := range active.Phase.AllowedMovements {
		if movement.FromWayID == fromWayID && movement.ToWayID == toWayID && len(movement.TargetLanes(lane)) > 0 {
			return true
		}
	}
	return false
}

type LaneProjection struct {
	Way      *Way
	Lane     int
	Position Vec2
	Distance float64
	Along    float64
	Tangent  Vec2
}

func (graph *Graph) NearestLane(point Vec2) LaneProjection {
	best := LaneProjection{Distance: math.Inf(1)}
	for _, way := range graph.SortedWays() {
		if way.Structure == "bridge" || way.Structure == "ramp" {
			continue
		}
		projection := way.Project(point)
		for lane := 0; lane < way.Lanes; lane++ {
			position, tangent := way.Sample(projection.Along, lane)
			distance := point.Sub(position).Length()
			if distance < best.Distance {
				best = LaneProjection{Way: way, Lane: lane, Position: position, Distance: distance, Along: projection.Along, Tangent: tangent}
			}
		}
	}
	return best
}

// ShortestPath returns directed ways from one intersection to another using
// free-flow travel time as cost. Unlike node-only routing, consecutive ways
// must have at least one lane connector through their shared intersection.
// Edge weight remains the lane count in Graph.
func (graph *Graph) ShortestPath(from, to string) ([]*Way, bool) {
	if from == to {
		return nil, true
	}
	if graph.Intersections[from] == nil || graph.Intersections[to] == nil {
		return nil, false
	}
	distance, previous, settled := make(map[string]float64), make(map[string]string), make(map[string]bool)
	for _, way := range sortedWays(graph.Outgoing[from]) {
		distance[way.ID] = wayTravelTime(way)
		previous[way.ID] = ""
	}
	goal := ""
	for {
		current, best := nearestUnsettledWay(distance, settled)
		if current == "" || math.IsInf(best, 1) {
			break
		}
		settled[current] = true
		way := graph.Ways[current]
		if way.To == to {
			goal = current
			break
		}
		for _, next := range graph.connectedOutgoing(way) {
			candidate := best + wayTravelTime(next)
			old, exists := distance[next.ID]
			if !exists || candidate < old-1e-12 || (math.Abs(candidate-old) <= 1e-12 && current < previous[next.ID]) {
				distance[next.ID], previous[next.ID] = candidate, current
			}
		}
	}
	if goal == "" {
		return nil, false
	}
	return reconstructWayPath(graph, goal, previous), true
}

// RouteBetweenWays returns a fully connected route including both endpoint
// ways. allowDirect permits a zero-intersection route when both endpoints are
// on the same way; when false, the search must leave and reconnect to that way.
func (graph *Graph) RouteBetweenWays(startWayID, finishWayID string, allowDirect bool) ([]*Way, bool) {
	if graph.Ways[startWayID] == nil || graph.Ways[finishWayID] == nil {
		return nil, false
	}
	if allowDirect && startWayID == finishWayID {
		return []*Way{graph.Ways[startWayID]}, true
	}
	start := wayRouteState{wayID: startWayID}
	distance := map[wayRouteState]float64{start: 0}
	previous := make(map[wayRouteState]wayRouteState)
	settled := make(map[wayRouteState]bool)
	goal := wayRouteState{}
	found := false
	for {
		current, best, ok := nearestRouteState(distance, settled)
		if !ok {
			break
		}
		settled[current] = true
		if current.departed && current.wayID == finishWayID {
			goal, found = current, true
			break
		}
		for _, next := range graph.connectedOutgoing(graph.Ways[current.wayID]) {
			candidateState := wayRouteState{wayID: next.ID, departed: true}
			candidate := best + wayTravelTime(next)
			old, exists := distance[candidateState]
			if !exists || candidate < old-1e-12 {
				distance[candidateState], previous[candidateState] = candidate, current
			}
		}
	}
	if !found {
		return nil, false
	}
	var reversed []*Way
	for cursor := goal; ; cursor = previous[cursor] {
		reversed = append(reversed, graph.Ways[cursor.wayID])
		if cursor == start {
			break
		}
	}
	path := make([]*Way, len(reversed))
	for index := range reversed {
		path[len(reversed)-1-index] = reversed[index]
	}
	return path, true
}

func (graph *Graph) connectedOutgoing(from *Way) []*Way {
	if from == nil {
		return nil
	}
	intersection := graph.Intersections[from.To]
	if intersection == nil {
		return nil
	}
	var result []*Way
	for _, to := range sortedWays(graph.Outgoing[from.To]) {
		if intersection.TrafficControl.HasMovement(from.ID, to.ID) {
			result = append(result, to)
		}
	}
	return result
}

func sortedWays(ways []*Way) []*Way {
	result := append([]*Way(nil), ways...)
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func wayTravelTime(way *Way) float64 {
	return way.Length / math.Max(1, way.SpeedLimit)
}

func nearestUnsettledWay(distance map[string]float64, settled map[string]bool) (string, float64) {
	current, best := "", math.Inf(1)
	for wayID, candidate := range distance {
		if settled[wayID] || candidate > best || (candidate == best && current != "" && wayID > current) {
			continue
		}
		current, best = wayID, candidate
	}
	return current, best
}

func nearestRouteState(distance map[wayRouteState]float64, settled map[wayRouteState]bool) (wayRouteState, float64, bool) {
	var current wayRouteState
	best, found := math.Inf(1), false
	for state, candidate := range distance {
		laterTie := candidate == best && found && (state.wayID > current.wayID || (state.wayID == current.wayID && state.departed && !current.departed))
		if settled[state] || candidate > best || laterTie {
			continue
		}
		current, best, found = state, candidate, true
	}
	return current, best, found
}

func reconstructWayPath(graph *Graph, goal string, previous map[string]string) []*Way {
	var reversed []*Way
	for cursor := goal; cursor != ""; cursor = previous[cursor] {
		reversed = append(reversed, graph.Ways[cursor])
	}
	path := make([]*Way, len(reversed))
	for index := range reversed {
		path[len(reversed)-1-index] = reversed[index]
	}
	return path
}
