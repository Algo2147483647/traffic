package sim

import (
	"math"

	"traffic/internal/model"
)

const (
	signalDetectorDistance = 180.0
	greenExtensionStep     = 4.0
)

// signalRuntime is deliberately independent for each intersection: every
// controller may extend or terminate a green without disturbing the others.
type signalRuntime struct {
	PhaseIndex        int
	LastServedPhase   int
	SelectedPhase     int
	Stage             string
	StageElapsed      float64
	GreenTarget       float64
	PhasePressures    []PhasePressure
	ActivePressure    float64
	CompetingPressure float64
	Decision          string
}

func (s *Simulation) initSignalsLocked() {
	s.signals = make(map[string]*signalRuntime)
	if !s.adaptive {
		return
	}
	for id, intersection := range s.graph.Intersections {
		control := intersection.TrafficControl
		if !control.Generated || !control.Adaptive || len(control.Phases) == 0 {
			continue
		}
		active := control.Active(0)
		if active.Index < 0 {
			continue
		}
		elapsed := active.Phase.DurationSeconds - active.Remaining
		switch active.Stage {
		case "amber":
			elapsed = active.Phase.AmberSeconds - active.Remaining
		case "all_red":
			elapsed = active.Phase.AllRedSeconds - active.Remaining
		}
		runtime := &signalRuntime{
			PhaseIndex: active.Index, LastServedPhase: active.Index, Stage: active.Stage, StageElapsed: math.Max(0, elapsed),
			GreenTarget: active.Phase.DurationSeconds, SelectedPhase: (active.Index + 1) % len(control.Phases),
			PhasePressures: make([]PhasePressure, len(control.Phases)), Decision: "design green",
		}
		s.signals[id] = runtime
	}
}

func (s *Simulation) updateSignalsLocked(dt float64) {
	if !s.adaptive || len(s.signals) == 0 {
		return
	}
	queues := s.signalQueuesLocked()
	scheduler := MaxPressureScheduler{}
	for id, runtime := range s.signals {
		control := s.graph.Intersections[id].TrafficControl
		decision := scheduler.Evaluate(control, queues, runtime.LastServedPhase)
		runtime.PhasePressures = decision.Phases
		s.advanceSignalLocked(control, runtime, dt)
	}
}

func (s *Simulation) advanceSignalLocked(control model.TrafficControl, runtime *signalRuntime, dt float64) {
	if runtime == nil || len(control.Phases) == 0 || runtime.PhaseIndex < 0 || runtime.PhaseIndex >= len(control.Phases) {
		return
	}
	s.refreshSignalSelection(runtime)
	for dt > 0 {
		phase := control.Phases[runtime.PhaseIndex]
		if runtime.Stage == "green" {
			s.setAdaptiveGreenTarget(control, runtime, phase)
		}
		duration := runtime.GreenTarget
		switch runtime.Stage {
		case "amber":
			duration = phase.AmberSeconds
		case "all_red":
			duration = phase.AllRedSeconds
		}
		remaining := math.Max(0, duration-runtime.StageElapsed)
		if remaining > dt {
			runtime.StageElapsed += dt
			return
		}
		dt -= remaining
		runtime.StageElapsed = 0
		switch runtime.Stage {
		case "green":
			runtime.LastServedPhase = runtime.PhaseIndex
			runtime.Stage = "amber"
			runtime.Decision = "clearance after " + runtime.Decision
		case "amber":
			runtime.Stage = "all_red"
			runtime.Decision = "conflict clearance"
		case "all_red":
			nextPhase := runtime.SelectedPhase
			if nextPhase < 0 || nextPhase >= len(control.Phases) || (len(control.Phases) > 1 && nextPhase == runtime.LastServedPhase) {
				nextPhase = selectNextMaximumPressure(runtime.PhasePressures, runtime.LastServedPhase)
			}
			runtime.PhaseIndex = nextPhase
			runtime.Stage = "green"
			next := control.Phases[runtime.PhaseIndex]
			runtime.GreenTarget = next.DurationSeconds
			runtime.Decision = "pressure-selected green"
		}
		s.refreshSignalSelection(runtime)
	}
}

func (s *Simulation) refreshSignalSelection(runtime *signalRuntime) {
	runtime.ActivePressure = phasePressureAt(runtime.PhasePressures, runtime.PhaseIndex)
	runtime.CompetingPressure = maxOtherPressure(runtime.PhasePressures, runtime.PhaseIndex)
	served := runtime.PhaseIndex
	if runtime.Stage != "green" {
		served = runtime.LastServedPhase
	}
	runtime.SelectedPhase = selectNextMaximumPressure(runtime.PhasePressures, served)
}

func (s *Simulation) setAdaptiveGreenTarget(control model.TrafficControl, runtime *signalRuntime, phase model.TrafficPhaseConfig) {
	minimum := phase.MinGreenSeconds
	if minimum <= 0 {
		minimum = phase.DurationSeconds
	}
	maximum := phase.MaxGreenSeconds
	if maximum < minimum {
		maximum = phase.DurationSeconds
	}
	base := phase.DurationSeconds
	elapsed := runtime.StageElapsed
	if elapsed < minimum {
		runtime.GreenTarget = math.Max(runtime.GreenTarget, minimum)
		runtime.Decision = "minimum green"
		return
	}
	if runtime.ActivePressure <= pressureEpsilon && runtime.CompetingPressure > pressureEpsilon {
		runtime.GreenTarget = elapsed
		runtime.Decision = "gap out"
		return
	}
	if elapsed < base {
		runtime.GreenTarget = math.Max(runtime.GreenTarget, base)
		runtime.Decision = "design green"
		return
	}
	if runtime.ActivePressure > runtime.CompetingPressure+pressureEpsilon && elapsed < maximum {
		runtime.GreenTarget = math.Min(maximum, math.Max(runtime.GreenTarget, elapsed+greenExtensionStep))
		runtime.Decision = "queue extension"
		return
	}
	if elapsed >= maximum-.001 {
		runtime.Decision = "maximum green"
	} else {
		runtime.Decision = "pressure served"
	}
	runtime.GreenTarget = math.Max(elapsed, math.Min(runtime.GreenTarget, maximum))
}

func maxOtherPressure(pressures []PhasePressure, active int) float64 {
	maximum := 0.0
	for index, evaluation := range pressures {
		if index != active && evaluation.Pressure > maximum {
			maximum = evaluation.Pressure
		}
	}
	return maximum
}

func phasePressureAt(pressures []PhasePressure, index int) float64 {
	if index < 0 || index >= len(pressures) {
		return 0
	}
	return pressures[index].Pressure
}

// signalQueuesLocked takes one consistent queue snapshot for all intersection
// controllers. Queue length is detector occupancy: each driving or stopped
// vehicle inside the zone contributes one vehicle. Incoming queues retain the
// vehicle's next movement; outgoing queues near a way's entrance represent
// receiving occupancy and spillback. Including approaching vehicles lets the
// controller react before they reach a red indication.
func (s *Simulation) signalQueuesLocked() QueueSnapshot {
	queues := QueueSnapshot{
		Incoming: make(map[MovementQueueKey]float64),
		Outgoing: make(map[LaneQueueKey]float64),
	}
	for _, person := range s.people {
		if (person.state != "driving" && person.state != "waiting") || person.wayID == "" {
			continue
		}
		if person.along >= -pressureEpsilon && person.along <= signalDetectorDistance+pressureEpsilon {
			queues.Outgoing[LaneQueueKey{WayID: person.wayID, Lane: person.lane}]++
		}
		if person.routeIndex+1 >= len(person.route) {
			continue
		}
		from := s.graph.Ways[person.wayID]
		if from == nil || s.signals[from.To] == nil {
			continue
		}
		distance := s.stopLine(from) - person.along
		if distance < -0.001 || distance > signalDetectorDistance {
			continue
		}
		toWayID := person.route[person.routeIndex+1]
		queues.Incoming[MovementQueueKey{FromWayID: from.ID, ToWayID: toWayID, FromLane: person.lane}]++
	}
	return queues
}

func (s *Simulation) signalAllowsMovement(intersectionID, fromWayID, toWayID string, lane int) bool {
	_, allowed := s.signalMovement(intersectionID, fromWayID, toWayID, lane)
	return allowed
}

// signalMovement returns only the lane connectors displayed by the active
// green. Route planning uses the union across all phases, but a vehicle may not
// select a receiving lane whose connector belongs to a different phase.
func (s *Simulation) signalMovement(intersectionID, fromWayID, toWayID string, lane int) (model.TrafficMovementConfig, bool) {
	intersection := s.graph.Intersections[intersectionID]
	if intersection == nil {
		return model.TrafficMovementConfig{}, false
	}
	runtime := s.signals[intersectionID]
	if runtime == nil {
		active := intersection.TrafficControl.Active(s.time)
		if active.Stage != "green" {
			return model.TrafficMovementConfig{}, false
		}
		return movementForLaneInPhase(active.Phase, fromWayID, toWayID, lane)
	}
	if runtime.Stage != "green" {
		return model.TrafficMovementConfig{}, false
	}
	return movementForLaneInPhase(intersection.TrafficControl.Phases[runtime.PhaseIndex], fromWayID, toWayID, lane)
}

func phaseForMovementInPhase(phase model.TrafficPhaseConfig, fromWayID, toWayID string, lane int) bool {
	_, allowed := movementForLaneInPhase(phase, fromWayID, toWayID, lane)
	return allowed
}

func movementForLaneInPhase(phase model.TrafficPhaseConfig, fromWayID, toWayID string, lane int) (model.TrafficMovementConfig, bool) {
	var result model.TrafficMovementConfig
	seen := make(map[[2]int]bool)
	for _, movement := range phase.AllowedMovements {
		if movement.FromWayID != fromWayID || movement.ToWayID != toWayID {
			continue
		}
		for _, connection := range movement.ConnectionsFrom(lane) {
			pair := [2]int{connection.FromLane, connection.ToLane}
			if seen[pair] {
				continue
			}
			if len(result.LaneConnections) == 0 {
				result = movement
				result.FromLanes = []int{lane}
				result.ToLanes = nil
				result.LaneConnections = nil
			}
			seen[pair] = true
			result.ToLanes = append(result.ToLanes, connection.ToLane)
			result.LaneConnections = append(result.LaneConnections, connection)
		}
	}
	return result, len(result.LaneConnections) > 0
}

func (s *Simulation) activeSignal(intersection *model.Intersection) model.ActivePhase {
	runtime := s.signals[intersection.ID]
	if runtime == nil {
		return intersection.TrafficControl.Active(s.time)
	}
	phase := intersection.TrafficControl.Phases[runtime.PhaseIndex]
	duration := runtime.GreenTarget
	switch runtime.Stage {
	case "amber":
		duration = phase.AmberSeconds
	case "all_red":
		duration = phase.AllRedSeconds
	}
	return model.ActivePhase{Index: runtime.PhaseIndex, Phase: phase, Stage: runtime.Stage, Remaining: math.Max(0, duration-runtime.StageElapsed)}
}
