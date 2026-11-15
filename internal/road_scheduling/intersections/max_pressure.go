package intersections

import (
	"math"

	model "traffic/internal/infrastructure/road"
)

const PressureEpsilon = 1e-9

// LaneQueueKey identifies a queue on one directed lane. It is used for the
// downstream term of movement pressure.
type LaneQueueKey struct {
	WayID string
	Lane  int
}

// MovementQueueKey identifies vehicles waiting for one exact movement from an
// incoming lane to an outgoing way.
type MovementQueueKey struct {
	FromWayID string
	ToWayID   string
	FromLane  int
}

// TargetMovementQueueKey refines an incoming movement queue by the receiving
// lane the vehicle needs for its following route movement. ToLane is -1 when
// any connector target in the phase can serve the vehicle.
type TargetMovementQueueKey struct {
	MovementQueueKey
	ToLane int
}

// QueueSnapshot is the scheduler's read-only view of current traffic demand.
// Incoming contains movement-specific detector queues approaching stop lines.
// Outgoing contains detector queues near downstream-way entrances, where they
// represent occupancy and spillback.
type QueueSnapshot struct {
	Incoming         map[MovementQueueKey]float64
	IncomingByTarget map[TargetMovementQueueKey]float64
	Outgoing         map[LaneQueueKey]float64
}

// MovementPressure exposes the terms used to score one permitted movement.
type MovementPressure struct {
	FromWayID       string                       `json:"from_way_id"`
	ToWayID         string                       `json:"to_way_id"`
	FromLanes       []int                        `json:"from_lanes"`
	ToLanes         []int                        `json:"to_lanes"`
	LaneConnections []model.LaneConnectionConfig `json:"lane_connections"`
	IncomingQueue   float64                      `json:"incoming_queue"`
	OutgoingQueue   float64                      `json:"outgoing_queue"`
	Pressure        float64                      `json:"pressure"`
}

// PhasePressure is the complete max-pressure evaluation of one feasible
// signal phase. IncomingQueue and OutgoingQueue are sums of the corresponding
// movement terms and are included for diagnostics.
type PhasePressure struct {
	PhaseIndex    int                `json:"phase_index"`
	PhaseID       string             `json:"phase_id"`
	IncomingQueue float64            `json:"incoming_queue"`
	OutgoingQueue float64            `json:"outgoing_queue"`
	Pressure      float64            `json:"pressure"`
	Movements     []MovementPressure `json:"movements"`
}

// PressureDecision contains every phase evaluation as well as the selected
// maximum. PhaseIndex is -1 when a control has no phases.
type PressureDecision struct {
	PhaseIndex int
	Pressure   float64
	Phases     []PhasePressure
}

// MaxPressureScheduler is stateless. Keeping queue measurement outside the
// scheduler makes the policy deterministic and straightforward to validate.
type MaxPressureScheduler struct{}

// Evaluate calculates every phase and selects the maximum-pressure phase.
//
// For movement m, each incoming lane i is evaluated against only the receiving
// lanes O(i) reachable by its explicit connectors:
//
//	w(m) = sum_i max(0, Q_in[m, i] - average(Q_out[o in O(i)]))
//
// A phase pressure is the sum of w(m) for all movements it permits. Averaging
// each source lane's reachable targets models their shared receiving capacity
// without allowing congestion in an unreachable lane to distort that source's
// pressure. Negative lane-connection pressure is clamped because opening that
// movement does not force traffic into a blocked exit. All phases are evaluated,
// including the phase just served.
//
// Equal maxima use cyclic configured order beginning after previouslyServed.
// Thus ties are reproducible while zero-demand operation still visits phases
// fairly. With an invalid previouslyServed index, configured index zero wins.
func (MaxPressureScheduler) Evaluate(control model.TrafficControl, queues QueueSnapshot, previouslyServed int) PressureDecision {
	evaluations := make([]PhasePressure, len(control.Phases))
	for index, phase := range control.Phases {
		evaluation := PhasePressure{
			PhaseIndex: index,
			PhaseID:    phase.ID,
			Movements:  make([]MovementPressure, 0, len(phase.AllowedMovements)),
		}
		for _, movement := range PressureMovements(phase.AllowedMovements) {
			incoming, outgoing, pressure := MovementQueueTerms(queues, movement)
			evaluation.IncomingQueue += incoming
			evaluation.OutgoingQueue += outgoing
			evaluation.Pressure += pressure
			evaluation.Movements = append(evaluation.Movements, MovementPressure{
				FromWayID: movement.FromWayID, ToWayID: movement.ToWayID,
				FromLanes: append([]int(nil), movement.FromLanes...), ToLanes: append([]int(nil), movement.ToLanes...),
				LaneConnections: append([]model.LaneConnectionConfig(nil), movement.LaneConnections...),
				IncomingQueue:   incoming, OutgoingQueue: outgoing, Pressure: pressure,
			})
		}
		evaluations[index] = evaluation
	}

	selected := SelectMaximumPressure(evaluations, previouslyServed)
	decision := PressureDecision{PhaseIndex: selected, Phases: evaluations}
	if selected >= 0 {
		decision.Pressure = evaluations[selected].Pressure
	}
	return decision
}

// Generated phases keep lane connectors separate so geometric compatibility
// can be evaluated exactly. Pressure, however, measures one queued route choice
// once and treats its displayed receiving connectors as alternatives; otherwise
// a turn that can reach four lanes would incorrectly count every vehicle four
// times.
func PressureMovements(movements []model.TrafficMovementConfig) []model.TrafficMovementConfig {
	type movementKey struct {
		fromWayID string
		toWayID   string
		fromLane  int
	}
	result := make([]model.TrafficMovementConfig, 0, len(movements))
	indices := make(map[movementKey]int)
	pairs := make(map[movementKey]map[[2]int]bool)
	for _, movement := range movements {
		if len(movement.LaneConnections) == 0 {
			result = append(result, movement)
			continue
		}
		for _, fromLane := range movement.FromLanes {
			key := movementKey{fromWayID: movement.FromWayID, toWayID: movement.ToWayID, fromLane: fromLane}
			index, exists := indices[key]
			if !exists {
				part := movement
				part.FromLanes = []int{fromLane}
				part.ToLanes = nil
				part.LaneConnections = nil
				index = len(result)
				indices[key] = index
				pairs[key] = make(map[[2]int]bool)
				result = append(result, part)
			}
			for _, connection := range movement.ConnectionsFrom(fromLane) {
				pair := [2]int{connection.FromLane, connection.ToLane}
				if pairs[key][pair] {
					continue
				}
				pairs[key][pair] = true
				result[index].LaneConnections = append(result[index].LaneConnections, connection)
				result[index].ToLanes = append(result[index].ToLanes, connection.ToLane)
			}
		}
	}
	return result
}

func MovementQueueTerms(queues QueueSnapshot, movement model.TrafficMovementConfig) (incoming, outgoing, pressure float64) {
	// Preserve the aggregate interpretation for legacy values assembled without
	// lane connections. Graph-built controls always use the exact branch below.
	if len(movement.LaneConnections) == 0 {
		incoming = MovementIncomingQueue(queues, movement)
		outgoing = MovementOutgoingQueue(queues, movement)
		return incoming, outgoing, math.Max(0, incoming-outgoing)
	}
	seen := make(map[int]bool, len(movement.FromLanes))
	for _, lane := range movement.FromLanes {
		if lane < 0 || seen[lane] {
			continue
		}
		seen[lane] = true
		key := MovementQueueKey{FromWayID: movement.FromWayID, ToWayID: movement.ToWayID, FromLane: lane}
		laneIncoming := MovementTargetIncomingQueue(queues, key, movement.TargetLanes(lane))
		laneOutgoing := TargetLaneQueue(queues, movement.ToWayID, movement.TargetLanes(lane))
		incoming += laneIncoming
		outgoing += laneOutgoing
		pressure += math.Max(0, laneIncoming-laneOutgoing)
	}
	return incoming, outgoing, pressure
}

func MovementTargetIncomingQueue(queues QueueSnapshot, key MovementQueueKey, targetLanes []int) float64 {
	if queues.IncomingByTarget == nil {
		return NonNegativeFinite(queues.Incoming[key])
	}
	queue := NonNegativeFinite(queues.IncomingByTarget[TargetMovementQueueKey{MovementQueueKey: key, ToLane: -1}])
	seen := make(map[int]bool, len(targetLanes))
	for _, lane := range targetLanes {
		if lane < 0 || seen[lane] {
			continue
		}
		seen[lane] = true
		queue += NonNegativeFinite(queues.IncomingByTarget[TargetMovementQueueKey{MovementQueueKey: key, ToLane: lane}])
	}
	return queue
}

func TargetLaneQueue(queues QueueSnapshot, wayID string, targets []int) float64 {
	queue, count := 0.0, 0
	seen := make(map[int]bool, len(targets))
	for _, lane := range targets {
		if lane < 0 || seen[lane] {
			continue
		}
		seen[lane] = true
		queue += NonNegativeFinite(queues.Outgoing[LaneQueueKey{WayID: wayID, Lane: lane}])
		count++
	}
	if count == 0 {
		return 0
	}
	return queue / float64(count)
}

func MovementIncomingQueue(queues QueueSnapshot, movement model.TrafficMovementConfig) float64 {
	queue := 0.0
	seen := make(map[int]bool, len(movement.FromLanes))
	for _, lane := range movement.FromLanes {
		if lane < 0 || seen[lane] {
			continue
		}
		seen[lane] = true
		queue += NonNegativeFinite(queues.Incoming[MovementQueueKey{
			FromWayID: movement.FromWayID, ToWayID: movement.ToWayID, FromLane: lane,
		}])
	}
	return queue
}

func MovementOutgoingQueue(queues QueueSnapshot, movement model.TrafficMovementConfig) float64 {
	queue := 0.0
	lanes := 0
	seen := make(map[int]bool, len(movement.ToLanes))
	for _, lane := range movement.ToLanes {
		if lane < 0 || seen[lane] {
			continue
		}
		seen[lane] = true
		queue += NonNegativeFinite(queues.Outgoing[LaneQueueKey{WayID: movement.ToWayID, Lane: lane}])
		lanes++
	}
	if lanes == 0 {
		return 0
	}
	return queue / float64(lanes)
}

func NonNegativeFinite(value float64) float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func SelectMaximumPressure(evaluations []PhasePressure, previouslyServed int) int {
	if len(evaluations) == 0 {
		return -1
	}
	selected := 0
	best := NonNegativeFinite(evaluations[0].Pressure)
	for index := 1; index < len(evaluations); index++ {
		candidate := NonNegativeFinite(evaluations[index].Pressure)
		if candidate > best+PressureEpsilon ||
			(math.Abs(candidate-best) <= PressureEpsilon && TieRank(index, previouslyServed, len(evaluations)) < TieRank(selected, previouslyServed, len(evaluations))) {
			selected, best = index, candidate
		}
	}
	return selected
}

// SelectNextMaximumPressure chooses the phase to serve after a completed green.
// The phase that just served is ineligible when alternatives exist; otherwise
// a dominant queue could repeatedly clear and immediately select itself,
// bypassing maximum green and starving the rest of the intersection.
func SelectNextMaximumPressure(evaluations []PhasePressure, justServed int) int {
	if len(evaluations) == 0 {
		return -1
	}
	if len(evaluations) == 1 {
		return 0
	}
	selected, best := -1, 0.0
	for index := range evaluations {
		if index == justServed {
			continue
		}
		candidate := NonNegativeFinite(evaluations[index].Pressure)
		if selected < 0 || candidate > best+PressureEpsilon ||
			(math.Abs(candidate-best) <= PressureEpsilon && TieRank(index, justServed, len(evaluations)) < TieRank(selected, justServed, len(evaluations))) {
			selected, best = index, candidate
		}
	}
	return selected
}

func TieRank(index, previouslyServed, phaseCount int) int {
	if previouslyServed < 0 || previouslyServed >= phaseCount {
		return index
	}
	distance := (index - previouslyServed + phaseCount) % phaseCount
	if distance == 0 {
		return phaseCount
	}
	return distance - 1
}
