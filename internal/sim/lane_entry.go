package sim

import (
	"math"

	"traffic/internal/model"
)

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
		clearance := s.laneEntryClearance(way.ID, lane, along, person)
		if clearance+pressureEpsilon < s.minimumVehicleSpacing() {
			continue
		}
		if clearance > bestClearance+pressureEpsilon ||
			(math.Abs(clearance-bestClearance) <= pressureEpsilon && absInt(lane-preferred) < absInt(bestLane-preferred)) {
			bestLane, bestClearance = lane, clearance
		}
	}
	return bestLane, bestLane >= 0
}

func (s *Simulation) laneEntryAvailable(wayID string, lane int, along float64, ignore *personState) bool {
	way := s.graph.Ways[wayID]
	if way == nil || lane < 0 || lane >= way.Lanes || along < 0 || along > way.Length {
		return false
	}
	return s.laneEntryClearance(wayID, lane, along, ignore)+pressureEpsilon >= s.minimumVehicleSpacing()
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
