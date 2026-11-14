package sim

import (
	"fmt"
	"math"
	"sort"

	model "traffic/internal/infrastructure/road"
)

const (
	laneChangeBaseDuration = 1.4
	laneChangeCooldown     = 2.5
	routePreparationBase   = 220.0
	overtakeLookaheadBase  = 90.0
)

type laneSpeedConstraint struct {
	speed    float64
	leaderID int
}

func (s *Simulation) advanceLaneChangesLocked(dt float64) {
	for _, person := range s.people {
		if !person.laneChanging {
			if person.state == "driving" || person.state == "waiting" {
				person.lanePosition = float64(person.lane)
			}
			continue
		}
		person.laneChangeProgress = math.Min(1, person.laneChangeProgress+dt/person.laneChangeDuration)
		smooth := person.laneChangeProgress * person.laneChangeProgress * (3 - 2*person.laneChangeProgress)
		person.lanePosition = float64(person.laneChangeFrom) + float64(person.laneChangeTo-person.laneChangeFrom)*smooth
		if person.laneChangeProgress < 1 {
			continue
		}
		person.lane = person.laneChangeTo
		person.lanePosition = float64(person.lane)
		person.laneChanging = false
		person.laneChangeProgress = 0
		person.lastLaneChangeAt = s.clock.Time
	}
}

func (s *Simulation) planLaneChangesLocked() {
	people := append([]*personState(nil), s.people...)
	sort.Slice(people, func(i, j int) bool { return people[i].id < people[j].id })
	for _, person := range people {
		if person.laneChanging || (person.state != "driving" && person.state != "waiting") || person.wayID == "" {
			continue
		}
		way := s.graph.Ways[person.wayID]
		if way == nil || way.Lanes < 2 {
			continue
		}

		routeLane, routeDistance, constrained := s.routeLaneTarget(person, way)
		difference := absInt(routeLane - person.lane)
		desiredSpeed := math.Min(s.graph.Simulation.DriveSpeed, way.SpeedLimit)
		preparationDistance := math.Max(routePreparationBase, float64(difference)*(desiredSpeed*laneChangeBaseDuration+s.safeGap+s.graph.Simulation.VehicleSize))
		if constrained && difference > 0 && routeDistance <= preparationDistance {
			target := person.lane
			if routeLane < person.lane {
				target--
			} else {
				target++
			}
			if s.laneChangeSafe(person, target) {
				s.startLaneChange(person, way, target, "route preparation")
			}
			continue
		}
		// Once a vehicle is in a compatible lane near its turn or destination,
		// do not let opportunistic passing pull it back out again.
		if constrained && routeDistance <= preparationDistance {
			continue
		}
		if person.lastLaneChangeAt > 0 && s.clock.Time-person.lastLaneChangeAt < laneChangeCooldown {
			continue
		}
		leader, currentClearance := s.nearestLeader(person, person.lane)
		lookahead := math.Max(overtakeLookaheadBase, desiredSpeed*2)
		if leader == nil || currentClearance > lookahead || (leader.state != "waiting" && leader.speed >= desiredSpeed*.65) {
			continue
		}

		candidates := []int{person.lane - 1, person.lane + 1}
		bestLane, bestScore := -1, currentClearance+s.graph.Simulation.VehicleSize
		passingLane := person.lane - 1
		if s.graph.DrivingSide == model.DrivingSideLeft {
			passingLane = person.lane + 1
		}
		var routeLanes []int
		if len(person.routeLanes) == len(person.route) {
			routeLanes = []int{person.routeLanes[person.routeIndex]}
		} else if person.routeIndex+1 < len(person.route) {
			routeLanes = movementLanes(s.graph.Intersections[way.To].TrafficControl, way.ID, person.route[person.routeIndex+1])
		}
		for _, candidate := range candidates {
			if candidate < 0 || candidate >= way.Lanes || !s.laneChangeSafe(person, candidate) {
				continue
			}
			// An intermediate road lane that cannot serve the next route
			// movement is not a safe overtaking lane: a dense queue may make
			// the required merge back impossible before the stop line.
			if len(routeLanes) > 0 && !laneIn(routeLanes, candidate) {
				continue
			}
			if constrained && routeDistance <= preparationDistance*1.5 && absInt(candidate-routeLane) > difference {
				continue
			}
			score := s.laneClearanceAhead(person, candidate)
			if candidate == passingLane {
				score += way.LaneWidth
			}
			if score > bestScore {
				bestLane, bestScore = candidate, score
			}
		}
		if bestLane >= 0 && s.policies.LaneChoice != nil {
			candidate := s.policies.LaneChoice(person.lane, bestLane, way.Lanes)
			if candidate >= 0 && candidate < way.Lanes && absInt(candidate-person.lane) == 1 && s.laneChangeSafe(person, candidate) {
				bestLane = candidate
			}
		}
		if bestLane >= 0 {
			s.startLaneChange(person, way, bestLane, "overtake")
		}
	}
}

func (s *Simulation) startLaneChange(person *personState, way *model.Way, target int, reason string) {
	if target < 0 || target >= way.Lanes || absInt(target-person.lane) != 1 {
		return
	}
	person.laneChanging = true
	person.laneChangeFrom = person.lane
	person.laneChangeTo = target
	person.laneChangeProgress = 0
	person.laneChangeDuration = math.Max(laneChangeBaseDuration, math.Min(2.2, way.LaneWidth/5))
	person.laneChangeReason = reason
}

func (s *Simulation) routeLaneTarget(person *personState, way *model.Way) (lane int, distance float64, constrained bool) {
	if len(person.route) == 0 || person.routeIndex < 0 || person.routeIndex >= len(person.route) {
		return person.lane, math.Inf(1), false
	}
	if person.routeIndex == len(person.route)-1 {
		return max(0, min(way.Lanes-1, person.destinationLane)), math.Max(0, person.destinationAlong-person.along), true
	}
	if len(person.routeLanes) == len(person.route) {
		return person.routeLanes[person.routeIndex], math.Max(0, s.stopLine(way)-person.along), true
	}
	nextWayID := person.route[person.routeIndex+1]
	control := s.graph.Intersections[way.To].TrafficControl
	available := movementLanes(control, way.ID, nextWayID)
	// Prepare for the receiving road's next movement before entering a short link.
	// This prevents permanent waits at green flow nodes with lane-count changes.
	viable := make([]int, 0, len(available))
	for _, fromLane := range available {
		movement, ok := control.Movement(way.ID, nextWayID, fromLane)
		if ok && len(s.routeCompatibleReceivingLanes(person, s.graph.Ways[nextWayID], person.routeIndex+1, movement.TargetLanes(fromLane))) > 0 {
			viable = append(viable, fromLane)
		}
	}
	if len(viable) > 0 {
		available = viable
	}

	if len(available) == 0 {
		return person.lane, math.Max(0, s.stopLine(way)-person.along), false
	}
	best, bestDistance := available[0], math.MaxInt
	for _, candidate := range available {
		laneDistance := absInt(candidate - person.lane)
		preferTie := laneDistance == bestDistance && ((person.id%2 == 0 && candidate < best) || (person.id%2 != 0 && candidate > best))
		if laneDistance < bestDistance || preferTie {
			best, bestDistance = candidate, laneDistance
		}
	}
	return best, math.Max(0, s.stopLine(way)-person.along), true
}

func movementLanes(control model.TrafficControl, fromWayID, toWayID string) []int {
	seen := make(map[int]bool)
	var lanes []int
	for _, phase := range control.Phases {
		for _, movement := range phase.AllowedMovements {
			if movement.FromWayID != fromWayID || movement.ToWayID != toWayID {
				continue
			}
			for _, lane := range movement.FromLanes {
				if !seen[lane] {
					seen[lane] = true
					lanes = append(lanes, lane)
				}
			}
		}
	}
	sort.Ints(lanes)
	return lanes
}

func laneIn(lanes []int, target int) bool {
	for _, lane := range lanes {
		if lane == target {
			return true
		}
	}
	return false
}

func (s *Simulation) laneChangeSafe(person *personState, targetLane int) bool {
	way := s.graph.Ways[person.wayID]
	if way == nil || targetLane < 0 || targetLane >= way.Lanes || absInt(targetLane-person.lane) != 1 {
		return false
	}
	minimumFront := s.safeGap + s.graph.Simulation.VehicleSize
	minimumRear := minimumFront
	for _, other := range s.people {
		if other == person || other.wayID != person.wayID || (other.state != "driving" && other.state != "waiting") || !occupiesLane(other, targetLane) {
			continue
		}
		delta := other.along - person.along
		if delta >= 0 {
			if delta < minimumFront {
				return false
			}
			continue
		}
		rearRequirement := minimumRear + math.Max(0, other.speed-person.speed)*laneChangeBaseDuration
		if -delta < rearRequirement {
			return false
		}
	}
	return true
}

func occupiesLane(person *personState, lane int) bool {
	return person.lane == lane || (person.laneChanging && person.laneChangeTo == lane)
}

func (s *Simulation) nearestLeader(person *personState, lane int) (*personState, float64) {
	var leader *personState
	distance := math.Inf(1)
	for _, other := range s.people {
		if other == person || other.wayID != person.wayID || !occupiesLane(other, lane) || (other.state != "driving" && other.state != "waiting") {
			continue
		}
		if delta := other.along - person.along; delta > 0 && delta < distance {
			leader, distance = other, delta
		}
	}
	return leader, distance
}

func (s *Simulation) laneClearanceAhead(person *personState, lane int) float64 {
	_, distance := s.nearestLeader(person, lane)
	if math.IsInf(distance, 1) {
		return s.graph.Ways[person.wayID].Length - person.along
	}
	return distance
}

// advanceRoadVehiclesLocked applies car-following constraints once per vehicle.
// A vehicle changing lanes occupies both its source and target lanes, so leaders
// and followers in either lane reserve space for the complete maneuver.
func (s *Simulation) advanceRoadVehiclesLocked(dt float64) {
	groups := make(map[string][]*personState)
	constraints := make(map[*personState]laneSpeedConstraint)
	var active []*personState
	for _, person := range s.people {
		if person.state != "driving" && person.state != "waiting" {
			continue
		}
		way := s.graph.Ways[person.wayID]
		active = append(active, person)
		constraints[person] = laneSpeedConstraint{speed: s.policySpeed(math.Min(s.graph.Simulation.DriveSpeed, way.SpeedLimit))}
		for lane := 0; lane < way.Lanes; lane++ {
			if occupiesLane(person, lane) {
				key := fmt.Sprintf("%s:%d", person.wayID, lane)
				groups[key] = append(groups[key], person)
			}
		}
	}
	for _, people := range groups {
		sort.Slice(people, func(i, j int) bool {
			if people[i].along == people[j].along {
				return people[i].id < people[j].id
			}
			return people[i].along > people[j].along
		})
		for index := 1; index < len(people); index++ {
			leader, follower := people[index-1], people[index]
			spacing := s.minimumVehicleSpacing()
			if leader.state == "waiting" && !leader.laneChanging {
				spacing = s.minimumStoppedVehicleSpacing()
			}
			freeDistance := leader.along - follower.along - spacing
			allowed := math.Max(0, leader.speed+freeDistance/dt)
			constraint := constraints[follower]
			if allowed < constraint.speed-1e-9 || (math.Abs(allowed-constraint.speed) <= 1e-9 && (constraint.leaderID == 0 || leader.id < constraint.leaderID)) {
				constraint.speed, constraint.leaderID = allowed, leader.id
				constraints[follower] = constraint
			}
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].id < active[j].id })
	parallelFor(len(active), 256, func(start, end int) {
		for index := start; index < end; index++ {
			person := active[index]
			constraint := constraints[person]
			way := s.graph.Ways[person.wayID]
			desiredSpeed := math.Min(s.graph.Simulation.DriveSpeed, way.SpeedLimit)
			target := s.roadTarget(person, way)
			allowedSpeed := math.Min(constraint.speed, math.Max(0, (target-person.along)/dt))
			person.speed, person.waitingFor = allowedSpeed, 0
			if allowedSpeed < desiredSpeed*.08 {
				person.state = "waiting"
				person.waitingFor = constraint.leaderID
				if constraint.leaderID != 0 {
					person.waitingReason = "leader queue"
				} else if person.waitingSignal == "" && person.waitingReason == "" {
					person.waitingReason = "road end"
				}
			} else {
				person.state, person.waitingReason = "driving", ""
				if person.along < s.stopLine(way)-0.001 {
					person.waitingSignal = ""
					person.signalWait = 0
				}
			}
			person.along = math.Min(target, person.along+allowedSpeed*dt)
			person.position, person.heading = way.SampleLane(person.along, person.lanePosition)
		}
	})
}
