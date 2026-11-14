package sim

import (
	"math"

	model "traffic/internal/infrastructure/road"
)

const stoppedVehicleGap = 2.5

// selectReceivingLane keeps the explicit lane connector valid while using any
// compatible downstream lane that has enough physical storage. Route preference
// wins when it is available; otherwise the clearest compatible lane is used.
func (s *Simulation) selectReceivingLane(person *personState, way *model.Way, allowed []int, preferred int, along float64) (int, bool) {
	if way == nil || len(allowed) == 0 {
		return 0, false
	}
	preferred = chooseExitLane(allowed, preferred, person.lane, s.graph.Ways[person.wayID].Lanes, way.Lanes)
	if s.laneEntryAvailable(way.ID, preferred, along, person) {
		return preferred, true
	}

	bestLane, bestClearance := -1, -1.0
	seen := make(map[int]bool, len(allowed))
	for _, lane := range allowed {
		if lane < 0 || lane >= way.Lanes || seen[lane] {
			continue
		}
		seen[lane] = true
		if !s.laneEntryAvailable(way.ID, lane, along, person) {
			continue
		}
		clearance := s.laneEntryClearance(way.ID, lane, along, person)
		if clearance > bestClearance+pressureEpsilon ||
			(math.Abs(clearance-bestClearance) <= pressureEpsilon && absInt(lane-preferred) < absInt(bestLane-preferred)) {
			bestLane, bestClearance = lane, clearance
		}
	}
	return bestLane, bestLane >= 0
}

// routeCompatibleReceivingLanes prevents a vehicle from entering an
// intermediate road in a lane that cannot serve its following route movement.
// Without this constraint, storage fallback can place vehicles in a free but
// unusable lane; once traffic becomes dense, they may never be able to merge
// back and can permanently block every lane at the next stop line.
func (s *Simulation) routeCompatibleReceivingLanes(person *personState, way *model.Way, routeIndex int, allowed []int) []int {
	// Hierarchical plans name the actual receiving lane separately from the
	// later exit lane. Follow its connector; road preparation handles any change.
	if way != nil && len(person.routeEntryLanes) == len(person.route) && routeIndex < len(person.route) {
		entry := person.routeEntryLanes[routeIndex]
		if laneIn(allowed, entry) {
			return []int{entry}
		}
		return nil
	}
	if way == nil || routeIndex+1 >= len(person.route) {
		return allowed
	}
	intersection := s.graph.Intersections[way.To]
	if intersection == nil {
		return nil
	}
	required := movementLanes(intersection.TrafficControl, way.ID, person.route[routeIndex+1])
	if len(person.routeLanes) == len(person.route) {
		required = []int{person.routeLanes[routeIndex]}
	}
	requiredSet := make(map[int]bool, len(required))
	for _, lane := range required {
		requiredSet[lane] = true
	}
	compatible := make([]int, 0, len(allowed))
	for _, lane := range allowed {
		if requiredSet[lane] {
			compatible = append(compatible, lane)
		}
	}
	// A continuous-flow entry may feed a sufficiently long preparation road.
	// Keep the legal entry connector; prepare for the next exit on that road.
	// Without this, a 2-lane branch feeding a 3-lane approach can wait forever
	// for an outer-lane connector which does not exist at the entry node.
	if len(compatible) == 0 && intersection.TrafficControl.Mode == "flow" {
		usable := s.stopLine(way) - math.Min(s.graph.Simulation.Intersection, way.Length)
		for _, entry := range allowed {
			for _, target := range required {
				needed := float64(absInt(entry-target)) * (math.Min(way.SpeedLimit, s.graph.Simulation.DriveSpeed)*laneChangeBaseDuration + s.safeGap + s.graph.Simulation.VehicleSize)
				if usable >= needed {
					compatible = append(compatible, entry)
					break
				}
			}
		}
	}
	return compatible
}

func (s *Simulation) laneEntryAvailable(wayID string, lane int, along float64, ignore *personState) bool {
	way := s.graph.Ways[wayID]
	if way == nil || lane < 0 || lane >= way.Lanes || along < 0 || along > way.Length {
		return false
	}
	if s.reservedEntry(wayID, lane, ignore) {
		return false
	}
	clearance, required := math.Inf(1), s.minimumVehicleSpacing()
	for _, other := range s.people {
		if other == ignore {
			continue
		}
		if other.state == "turning" && other.nextWayID == wayID && other.nextLane == lane {
			return false
		}
		if other.wayID != wayID || (other.state != "driving" && other.state != "waiting") || !occupiesLane(other, lane) {
			continue
		}
		if distance := math.Abs(other.along - along); distance < clearance {
			clearance = distance
			required = s.minimumVehicleSpacing()
			if other.state == "waiting" && !other.laneChanging {
				required = s.minimumStoppedVehicleSpacing()
			}
		}
	}
	return clearance+pressureEpsilon >= required
}

func (s *Simulation) laneEntryClearance(wayID string, lane int, along float64, ignore *personState) float64 {
	clearance := math.Inf(1)
	for _, other := range s.people {
		if other == ignore {
			continue
		}
		if other.state == "turning" && other.nextWayID == wayID && other.nextLane == lane {
			return 0
		}
		if other.wayID != wayID || (other.state != "driving" && other.state != "waiting") || !occupiesLane(other, lane) {
			continue
		}
		clearance = math.Min(clearance, math.Abs(other.along-along))
	}
	return clearance
}

func (s *Simulation) minimumVehicleSpacing() float64 {
	return s.safeGap + s.graph.Simulation.VehicleSize
}

func (s *Simulation) minimumStoppedVehicleSpacing() float64 {
	return math.Min(s.safeGap, stoppedVehicleGap) + s.graph.Simulation.VehicleSize
}
