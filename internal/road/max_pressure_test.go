package sim

import (
	"math"
	"testing"

	model "traffic/internal/infrastructure/road"
)

func TestMaxPressureEvaluatesEveryPhaseAndSelectsLargestDifferential(t *testing.T) {
	control := maxPressureTestControl()
	queues := QueueSnapshot{
		Incoming: map[MovementQueueKey]float64{
			{FromWayID: "A-in", ToWayID: "A-out", FromLane: 0}: 5,
			{FromWayID: "A-in", ToWayID: "A-out", FromLane: 1}: 4,
			{FromWayID: "B-in", ToWayID: "B-out", FromLane: 0}: 7,
		},
		Outgoing: map[LaneQueueKey]float64{
			{WayID: "A-out", Lane: 0}: 2,
			{WayID: "A-out", Lane: 1}: 4,
		},
	}

	decision := (MaxPressureScheduler{}).Evaluate(control, queues, -1)
	if len(decision.Phases) != len(control.Phases) {
		t.Fatalf("evaluated %d phases, want %d", len(decision.Phases), len(control.Phases))
	}
	// Phase A: max(0, (5+4) - average(2,4)) = 6.
	if got := decision.Phases[0]; got.IncomingQueue != 9 || got.OutgoingQueue != 3 || got.Pressure != 6 {
		t.Fatalf("unexpected phase A pressure: %+v", got)
	}
	// Phase B has pressure 7 and must win even though it follows phase A in the plan.
	if decision.PhaseIndex != 1 || decision.Pressure != 7 {
		t.Fatalf("selected phase %d at pressure %.1f, want phase 1 at 7", decision.PhaseIndex, decision.Pressure)
	}
}

func TestMaxPressureRespondsToChangingDemandAndDownstreamQueues(t *testing.T) {
	control := maxPressureTestControl()
	scheduler := MaxPressureScheduler{}
	queues := QueueSnapshot{Incoming: make(map[MovementQueueKey]float64), Outgoing: make(map[LaneQueueKey]float64)}

	queues.Incoming[MovementQueueKey{FromWayID: "A-in", ToWayID: "A-out", FromLane: 0}] = 10
	queues.Incoming[MovementQueueKey{FromWayID: "B-in", ToWayID: "B-out", FromLane: 0}] = 4
	if got := scheduler.Evaluate(control, queues, -1).PhaseIndex; got != 0 {
		t.Fatalf("phase %d selected for initial A demand, want 0", got)
	}

	// Blocking A's two downstream lanes reduces A's pressure from 10 to 2.
	// The unchanged B queue now has the largest differential.
	queues.Outgoing[LaneQueueKey{WayID: "A-out", Lane: 0}] = 8
	queues.Outgoing[LaneQueueKey{WayID: "A-out", Lane: 1}] = 8
	if got := scheduler.Evaluate(control, queues, 0).PhaseIndex; got != 1 {
		t.Fatalf("phase %d selected after A downstream filled, want 1", got)
	}

	delete(queues.Incoming, MovementQueueKey{FromWayID: "A-in", ToWayID: "A-out", FromLane: 0})
	queues.Incoming[MovementQueueKey{FromWayID: "B-in", ToWayID: "B-out", FromLane: 0}] = 12
	if got := scheduler.Evaluate(control, queues, 0).PhaseIndex; got != 1 {
		t.Fatalf("phase %d selected after demand shifted to B, want 1", got)
	}
}

func TestMaxPressureUsesExactLaneConnectionTargets(t *testing.T) {
	movement := model.TrafficMovementConfig{
		FromWayID: "in", ToWayID: "out", FromLanes: []int{0, 1}, ToLanes: []int{0, 1},
		LaneConnections: []model.LaneConnectionConfig{{FromLane: 0, ToLane: 0}, {FromLane: 1, ToLane: 1}},
	}
	control := model.TrafficControl{Phases: []model.TrafficPhaseConfig{{ID: "connected", AllowedMovements: []model.TrafficMovementConfig{movement}}}}
	queues := QueueSnapshot{
		Incoming: map[MovementQueueKey]float64{
			{FromWayID: "in", ToWayID: "out", FromLane: 0}: 5,
			{FromWayID: "in", ToWayID: "out", FromLane: 1}: 5,
		},
		Outgoing: map[LaneQueueKey]float64{{WayID: "out", Lane: 0}: 5, {WayID: "out", Lane: 1}: 0},
	}
	got := (MaxPressureScheduler{}).Evaluate(control, queues, -1).Phases[0]
	if got.IncomingQueue != 10 || got.OutgoingQueue != 5 || got.Pressure != 5 {
		t.Fatalf("pressure mixed unrelated lane targets: %+v", got)
	}
}

func TestMaxPressureCountsSplitReceivingConnectorsOnce(t *testing.T) {
	control := model.TrafficControl{Phases: []model.TrafficPhaseConfig{{
		ID: "split-targets", AllowedMovements: []model.TrafficMovementConfig{
			{FromWayID: "in", ToWayID: "out", FromLanes: []int{0}, ToLanes: []int{0}, LaneConnections: []model.LaneConnectionConfig{{FromLane: 0, ToLane: 0}}},
			{FromWayID: "in", ToWayID: "out", FromLanes: []int{0}, ToLanes: []int{1}, LaneConnections: []model.LaneConnectionConfig{{FromLane: 0, ToLane: 1}}},
		},
	}}}
	queues := QueueSnapshot{
		Incoming: map[MovementQueueKey]float64{{FromWayID: "in", ToWayID: "out", FromLane: 0}: 5},
		Outgoing: map[LaneQueueKey]float64{{WayID: "out", Lane: 0}: 1, {WayID: "out", Lane: 1}: 3},
	}
	got := (MaxPressureScheduler{}).Evaluate(control, queues, -1).Phases[0]
	if got.IncomingQueue != 5 || got.OutgoingQueue != 2 || got.Pressure != 3 || len(got.Movements) != 1 {
		t.Fatalf("split receiving connectors double-counted one route queue: %+v", got)
	}
}

func TestMaxPressureCountsDemandOnlyForItsRequiredReceivingLane(t *testing.T) {
	key := MovementQueueKey{FromWayID: "in", ToWayID: "out", FromLane: 0}
	control := model.TrafficControl{Phases: []model.TrafficPhaseConfig{
		{ID: "lane-0", AllowedMovements: []model.TrafficMovementConfig{{
			FromWayID: "in", ToWayID: "out", FromLanes: []int{0}, ToLanes: []int{0},
			LaneConnections: []model.LaneConnectionConfig{{FromLane: 0, ToLane: 0}},
		}}},
		{ID: "lane-1", AllowedMovements: []model.TrafficMovementConfig{{
			FromWayID: "in", ToWayID: "out", FromLanes: []int{0}, ToLanes: []int{1},
			LaneConnections: []model.LaneConnectionConfig{{FromLane: 0, ToLane: 1}},
		}}},
	}}
	queues := QueueSnapshot{
		Incoming: map[MovementQueueKey]float64{key: 9},
		IncomingByTarget: map[TargetMovementQueueKey]float64{
			{MovementQueueKey: key, ToLane: -1}: 2,
			{MovementQueueKey: key, ToLane: 1}:  7,
		},
		Outgoing: make(map[LaneQueueKey]float64),
	}

	decision := (MaxPressureScheduler{}).Evaluate(control, queues, -1)
	if decision.Phases[0].IncomingQueue != 2 || decision.Phases[1].IncomingQueue != 9 || decision.PhaseIndex != 1 {
		t.Fatalf("target-lane demand was assigned to a phase that cannot serve it: %+v", decision)
	}
}

func TestMaxPressureTieAndEmptyDemandAreDeterministic(t *testing.T) {
	control := maxPressureTestControl()
	scheduler := MaxPressureScheduler{}
	empty := QueueSnapshot{}

	if got := scheduler.Evaluate(control, empty, -1).PhaseIndex; got != 0 {
		t.Fatalf("invalid previous phase tie selected %d, want configured phase 0", got)
	}
	if got := scheduler.Evaluate(control, empty, 0).PhaseIndex; got != 1 {
		t.Fatalf("tie after phase 0 selected %d, want cyclic phase 1", got)
	}
	if got := scheduler.Evaluate(control, empty, 1).PhaseIndex; got != 0 {
		t.Fatalf("tie after phase 1 selected %d, want cyclic phase 0", got)
	}
	if got := scheduler.Evaluate(model.TrafficControl{}, empty, -1).PhaseIndex; got != -1 {
		t.Fatalf("empty plan selected phase %d, want -1", got)
	}
}

func TestNextPhaseCannotImmediatelyReselectCompletedGreen(t *testing.T) {
	evaluations := []PhasePressure{{PhaseIndex: 0, Pressure: 100}, {PhaseIndex: 1, Pressure: 6}, {PhaseIndex: 2, Pressure: 3}}
	if got := selectMaximumPressure(evaluations, 0); got != 0 {
		t.Fatalf("unrestricted maximum = %d, want dominant phase 0", got)
	}
	if got := selectNextMaximumPressure(evaluations, 0); got != 1 {
		t.Fatalf("next phase reselected completed phase or ignored best alternative: %d", got)
	}
	evaluations[0].Pressure, evaluations[1].Pressure, evaluations[2].Pressure = 0, 0, 0
	if got := selectNextMaximumPressure(evaluations, 1); got != 2 {
		t.Fatalf("zero-pressure tie did not advance cyclically after phase 1: %d", got)
	}
	if got := selectNextMaximumPressure(evaluations[:1], 0); got != 0 {
		t.Fatalf("single-phase controller selected %d, want 0", got)
	}
}

func TestMaximumRedPhaseOverridesHigherPressure(t *testing.T) {
	evaluations := []PhasePressure{{PhaseIndex: 0, Pressure: 100}, {PhaseIndex: 1, Pressure: 20}, {PhaseIndex: 2, Pressure: 2}}
	redElapsed := []float64{0, 30, 121}
	if got := selectNextPhase(evaluations, 0, redElapsed, 120); got != 2 {
		t.Fatalf("maximum-red guard selected phase %d, want overdue phase 2", got)
	}
	redElapsed[2] = 119
	if got := selectNextPhase(evaluations, 0, redElapsed, 120); got != 1 {
		t.Fatalf("pressure selection chose phase %d before maximum red, want phase 1", got)
	}
}

func TestMaxPressureSanitizesInvalidQueueValues(t *testing.T) {
	control := maxPressureTestControl()
	queues := QueueSnapshot{
		Incoming: map[MovementQueueKey]float64{
			{FromWayID: "A-in", ToWayID: "A-out", FromLane: 0}: math.NaN(),
			{FromWayID: "A-in", ToWayID: "A-out", FromLane: 1}: -3,
		},
		Outgoing: map[LaneQueueKey]float64{{WayID: "A-out", Lane: 0}: math.Inf(1)},
	}
	decision := (MaxPressureScheduler{}).Evaluate(control, queues, -1)
	if decision.Phases[0].Pressure != 0 || math.IsNaN(decision.Phases[0].Pressure) {
		t.Fatalf("invalid queues produced pressure %+v", decision.Phases[0])
	}
}

func TestSimulationAppliesNewMaximumPressureAtDecisionBoundary(t *testing.T) {
	config, err := model.LoadConfig("../../example/Map1/road.json")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := model.BuildGraph(config)
	if err != nil {
		t.Fatal(err)
	}
	simulation := New(graph)
	intersection := graph.Intersections["AC"]
	runtime := simulation.signals[intersection.ID]
	if runtime == nil || len(intersection.TrafficControl.Phases) < 2 {
		t.Fatal("central intersection needs at least two adaptive phases")
	}

	first := intersection.TrafficControl.Phases[0].AllowedMovements[0]
	second := intersection.TrafficControl.Phases[1].AllowedMovements[0]
	simulation.people = queuedMovement(simulation, first, 6)
	simulation.updateSignalsLocked(.01)
	if runtime.ActivePressure != 6 || runtime.SelectedPhase == runtime.PhaseIndex {
		t.Fatalf("active pressure or pending next phase was not stateful: %+v", runtime)
	}

	simulation.people = queuedMovement(simulation, second, 8)
	runtime.PhaseIndex = 0
	runtime.Stage = "all_red"
	runtime.StageElapsed = intersection.TrafficControl.Phases[0].AllRedSeconds
	simulation.updateSignalsLocked(.01)
	if runtime.PhaseIndex != 1 || runtime.Stage != "green" {
		t.Fatalf("changed demand was not applied at boundary: %+v", runtime)
	}
	state := simulation.Snapshot().Signals
	var central SignalState
	for _, signal := range state {
		if signal.IntersectionID == intersection.ID {
			central = signal
			break
		}
	}
	if len(central.PhasePressures) != len(intersection.TrafficControl.Phases) || central.IncomingQueue != 8 || central.QueuePressure != 8 {
		t.Fatalf("snapshot omitted pressure data: %+v", central)
	}
}

func maxPressureTestControl() model.TrafficControl {
	return model.TrafficControl{Phases: []model.TrafficPhaseConfig{
		{ID: "A", AllowedMovements: []model.TrafficMovementConfig{{
			FromWayID: "A-in", ToWayID: "A-out", FromLanes: []int{0, 1}, ToLanes: []int{0, 1},
		}}},
		{ID: "B", AllowedMovements: []model.TrafficMovementConfig{{
			FromWayID: "B-in", ToWayID: "B-out", FromLanes: []int{0}, ToLanes: []int{0},
		}}},
	}}
}

func queuedMovement(simulation *Simulation, movement model.TrafficMovementConfig, count int) []*personState {
	people := make([]*personState, 0, count)
	way := simulation.graph.Ways[movement.FromWayID]
	for index := 0; index < count; index++ {
		people = append(people, &personState{
			id: index + 1, state: "waiting", wayID: movement.FromWayID,
			lane: movement.FromLanes[index%len(movement.FromLanes)], along: simulation.stopLine(way),
			route: []string{movement.FromWayID, movement.ToWayID},
		})
	}
	return people
}
