package roadscheduling

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"traffic/internal/foundation"

	model "traffic/internal/infrastructure/road"
)

func simulationGraph(t *testing.T) *model.Graph {
	t.Helper()
	graph, err := model.BuildGraph(model.CityConfig{
		Name: "Queue City", Bounds: model.Bounds{MaxX: 120, MaxY: 80},
		Simulation: model.SimulationConfig{People: 4, TickMS: 50, WalkSpeed: 10, DriveSpeed: 20, SafeGap: 10, VehicleSize: 4, Seed: 1},
		Intersections: []model.IntersectionConfig{
			{ID: "A", Position: model.Vec2{X: 10, Y: 40}},
			{ID: "B", Position: model.Vec2{X: 110, Y: 40}},
		},
		Ways: []model.WayConfig{
			{ID: "AB", From: "A", To: "B", Lanes: 1, LaneWidth: 4, SpeedLimit: 20},
			{ID: "BA", From: "B", To: "A", Lanes: 1, LaneWidth: 4, SpeedLimit: 20},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func multiLaneGraph(t *testing.T) *model.Graph {
	t.Helper()
	graph, err := model.BuildGraph(model.CityConfig{
		Name: "Lane Change City", Bounds: model.Bounds{MaxX: 700, MaxY: 400},
		Simulation: model.SimulationConfig{People: 1, TickMS: 50, WalkSpeed: 10, DriveSpeed: 20, SafeGap: 10, VehicleSize: 4, Intersection: 22, Seed: 2},
		Intersections: []model.IntersectionConfig{
			{ID: "A", Position: model.Vec2{X: 20, Y: 200}},
			{ID: "B", Position: model.Vec2{X: 620, Y: 200}},
			{ID: "N", Position: model.Vec2{X: 620, Y: 20}},
			{ID: "S", Position: model.Vec2{X: 620, Y: 380}},
		},
		Ways: []model.WayConfig{
			{ID: "AB", From: "A", To: "B", Lanes: 3, LaneWidth: 7, SpeedLimit: 20},
			{ID: "BA", From: "B", To: "A", Lanes: 3, LaneWidth: 7, SpeedLimit: 20},
			{ID: "BN", From: "B", To: "N", Lanes: 3, LaneWidth: 7, SpeedLimit: 20},
			{ID: "NB", From: "N", To: "B", Lanes: 3, LaneWidth: 7, SpeedLimit: 20},
			{ID: "BS", From: "B", To: "S", Lanes: 3, LaneWidth: 7, SpeedLimit: 20},
			{ID: "SB", From: "S", To: "B", Lanes: 3, LaneWidth: 7, SpeedLimit: 20},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func TestPeopleWalkToNearestWayThenDrive(t *testing.T) {
	simulation := New(simulationGraph(t))
	for i := 0; i < 100; i++ {
		simulation.Tick(.05)
	}
	snapshot := simulation.Snapshot()
	if snapshot.Time < 4.99 {
		t.Fatalf("time = %.2f, want 5", snapshot.Time)
	}
	if snapshot.Stats.Driving+snapshot.Stats.Waiting == 0 {
		t.Fatal("expected at least one person to reach a way")
	}
	for _, person := range snapshot.People {
		if person.Reference.IntersectionID == "" {
			t.Errorf("person %d has no reference intersection", person.ID)
		}
		if person.Transform[2] != [3]float64{0, 0, 1} {
			t.Errorf("person %d transform is not homogeneous", person.ID)
		}
	}
}

func TestFollowerWaitsForVehicleAhead(t *testing.T) {
	simulation := New(simulationGraph(t))
	simulation.people = []*personState{
		{id: 1, state: "driving", wayID: "AB", along: 50, heading: model.Vec2{X: 1}},
		{id: 2, state: "driving", wayID: "AB", along: 40, heading: model.Vec2{X: 1}},
	}
	simulation.advanceLocked(.1)
	leader, follower := simulation.people[0], simulation.people[1]
	if math.Abs(leader.along-50.015) > .001 {
		t.Fatalf("leader along = %.2f, want 50.015", leader.along)
	}
	if follower.state != "waiting" || follower.speed != 0 || follower.waitingFor != leader.id {
		t.Fatalf("follower did not wait: %+v", follower)
	}
}

func TestStoppedQueueCompactsWithoutReducingMovingGap(t *testing.T) {
	simulation := New(simulationGraph(t))
	leader := &personState{id: 1, state: "waiting", wayID: "AB", lane: 0, along: 50, route: []string{"AB"}, destinationAlong: 90}
	follower := &personState{id: 2, state: "driving", wayID: "AB", lane: 0, along: 36, route: []string{"AB"}, destinationAlong: 90}
	simulation.people = []*personState{leader, follower}

	simulation.advanceRoadVehiclesLocked(.1)
	if follower.along <= 36 || follower.state != "driving" {
		t.Fatalf("follower did not use the compact stopped-queue gap: %+v", follower)
	}

	leader.state, leader.speed, leader.along = "driving", 20, 50
	follower.state, follower.speed, follower.along = "driving", 20, 38
	simulation.advanceRoadVehiclesLocked(.1)
	if follower.along != 38 || follower.state != "waiting" {
		t.Fatalf("moving follower ignored the configured safe gap: %+v", follower)
	}
}

func TestRuntimeVehicleSpacingIsApplied(t *testing.T) {
	simulation := New(simulationGraph(t))
	simulation.SetSpacing(30)
	simulation.people = []*personState{
		{id: 1, state: "driving", wayID: "AB", along: 70, route: []string{"AB"}, destinationAlong: 90, heading: model.Vec2{X: 1}},
		{id: 2, state: "driving", wayID: "AB", along: 45, route: []string{"AB"}, destinationAlong: 90, heading: model.Vec2{X: 1}},
	}
	simulation.advanceLocked(.1)
	if simulation.people[1].speed != 0 || simulation.Snapshot().Spacing.SafeGap != 30 {
		t.Fatalf("runtime spacing was not applied: %+v", simulation.people[1])
	}
}

func TestRuntimeLoadSupportsTenThousandPeople(t *testing.T) {
	simulation := New(simulationGraph(t))
	simulation.SetLoad(10000, 0, 10000)
	if got := len(simulation.people); got != 10000 {
		t.Fatalf("population after SetLoad = %d, want 10000", got)
	}
	if simulation.maxPeople != 10000 {
		t.Fatalf("population cap = %d, want 10000", simulation.maxPeople)
	}
	for index, person := range simulation.people {
		if person.id != index+1 || len(person.route) == 0 {
			t.Fatalf("parallel route initialization produced invalid person %d: %+v", index, person)
		}
	}
}

func TestClearanceSignalSerializesEmptyMovementArrays(t *testing.T) {
	simulation := NewFixedTime(simulationGraph(t))
	control := simulation.graph.Intersections["A"].TrafficControl
	simulation.clock.Time = control.Phases[0].DurationSeconds + .1
	snapshot := simulation.Snapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if !strings.Contains(encoded, `"allowed_movements":[]`) || !strings.Contains(encoded, `"allowed_incoming_way_ids":[]`) {
		t.Fatalf("clearance snapshot used null signal arrays: %s", encoded)
	}
}

func TestOriginsAndDestinationsAreUniformRandomPoints(t *testing.T) {
	simulation := New(simulationGraph(t))
	simulation.Reset(1000)
	originQuadrants, destinationQuadrants := [4]int{}, [4]int{}
	for _, person := range simulation.people {
		if person.origin == person.destination {
			t.Fatalf("person %d has identical origin and destination", person.id)
		}
		originQuadrants[boolIndex(person.origin.X >= 60)+2*boolIndex(person.origin.Y >= 40)]++
		destinationQuadrants[boolIndex(person.destination.X >= 60)+2*boolIndex(person.destination.Y >= 40)]++
	}
	for index := range originQuadrants {
		if originQuadrants[index] < 180 || originQuadrants[index] > 320 {
			t.Errorf("origin quadrant %d count = %d", index, originQuadrants[index])
		}
		if destinationQuadrants[index] < 180 || destinationQuadrants[index] > 320 {
			t.Errorf("destination quadrant %d count = %d", index, destinationQuadrants[index])
		}
	}
}

func boolIndex(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestPersonCompletesVehicleAndFinalWalkingLegs(t *testing.T) {
	simulation := New(simulationGraph(t))
	sawDrive, sawFinalWalk, sawArrival := false, false, false
	for step := 0; step < 1600 && !sawArrival; step++ {
		simulation.Tick(.05)
		for _, person := range simulation.people {
			sawDrive = sawDrive || person.state == "driving" || person.state == "waiting"
			sawFinalWalk = sawFinalWalk || person.state == "walking_to_destination"
			sawArrival = sawArrival || person.state == "arrived"
		}
	}
	if !sawDrive || !sawFinalWalk || !sawArrival {
		t.Fatalf("trip stages drive=%v final_walk=%v arrived=%v", sawDrive, sawFinalWalk, sawArrival)
	}
}

func TestVehicleWaitsForRedSignal(t *testing.T) {
	cfg := model.CityConfig{Name: "Signal City", Bounds: model.Bounds{MaxX: 120, MaxY: 80}, Simulation: model.SimulationConfig{People: 1, DriveSpeed: 20, Intersection: 22},
		Intersections: []model.IntersectionConfig{
			{ID: "A", Position: model.Vec2{X: 10, Y: 40}},
			{ID: "B", Position: model.Vec2{X: 110, Y: 40}, Traffic: model.TrafficControlConfig{Phases: []model.TrafficPhaseConfig{
				{ID: "red", DurationSeconds: 5}, {ID: "green", DurationSeconds: 5, AllowedIncoming: []string{"AB"}},
			}}},
		},
		Ways: []model.WayConfig{{ID: "AB", From: "A", To: "B", Lanes: 1}, {ID: "BA", From: "B", To: "A", Lanes: 1}},
	}
	graph, err := model.BuildGraph(cfg)
	if err != nil {
		t.Fatal(err)
	}
	simulation := New(graph)
	simulation.people = []*personState{{id: 1, state: "driving", wayID: "AB", lane: 0, along: 78, route: []string{"AB", "BA"}, destinationAlong: 40, heading: model.Vec2{X: 1}}}
	simulation.advanceLocked(.1)
	if simulation.people[0].waitingSignal != "B" || simulation.people[0].routeIndex != 0 {
		t.Fatalf("vehicle crossed red signal: %+v", simulation.people[0])
	}
	// The step crosses the fixed-time boundary. Stop-line decisions must observe
	// the end-of-step clock (5.05), not the stale start time (4.95).
	simulation.clock.Time = 4.95
	simulation.advanceLocked(.1)
	if simulation.people[0].state != "turning" || simulation.people[0].routeIndex != 1 {
		t.Fatalf("vehicle did not enter its turn on green: %+v", simulation.people[0])
	}
	for index := 0; index < 30 && simulation.people[0].state == "turning"; index++ {
		simulation.advanceLocked(.1)
	}
	if simulation.people[0].wayID != "BA" || simulation.people[0].state != "driving" {
		t.Fatalf("vehicle did not finish the lane connector: %+v", simulation.people[0])
	}
}

func TestIntersectionUsesFreeCompatibleReceivingLane(t *testing.T) {
	simulation := New(multiLaneGraph(t))
	control := simulation.graph.Intersections["B"].TrafficControl
	phaseIndex := -1
	for index, phase := range control.Phases {
		movement, allowed := movementForLaneInPhase(phase, "AB", "BS", 2)
		if allowed && containsInt(movement.ToLanes, 0) && containsInt(movement.ToLanes, 1) {
			phaseIndex = index
			break
		}
	}
	if phaseIndex < 0 {
		t.Fatal("generated plan omitted the right turn from AB to BS")
	}
	simulation.signals["B"].PhaseIndex, simulation.signals["B"].Stage = phaseIndex, "green"
	nextAlong := math.Min(simulation.graph.Simulation.Intersection, simulation.graph.Ways["BS"].Length)
	person := &personState{
		id: 1, state: "driving", wayID: "AB", lane: 2, lanePosition: 2,
		along: simulation.stopLine(simulation.graph.Ways["AB"]), route: []string{"AB", "BS"}, destinationLane: 0, destinationAlong: 120,
	}
	blocker := &personState{id: 2, state: "waiting", wayID: "BS", lane: 0, lanePosition: 0, along: nextAlong}
	simulation.people = []*personState{person, blocker}

	simulation.transition(person)
	if person.state != "turning" || person.nextLane != 1 {
		t.Fatalf("vehicle did not use a free compatible receiving lane: %+v", person)
	}
}

func TestIntersectionWaitsForRouteCompatibleReceivingLane(t *testing.T) {
	simulation := New(multiLaneGraph(t))
	control := simulation.graph.Intersections["B"].TrafficControl
	required := movementLanes(simulation.graph.Intersections["S"].TrafficControl, "BS", "SB")
	if len(required) == 0 {
		t.Fatal("generated plan omitted the next route movement from BS to SB")
	}
	requiredLane := required[0]
	phaseIndex := -1
	for index, phase := range control.Phases {
		movement, allowed := movementForLaneInPhase(phase, "AB", "BS", 2)
		if allowed && containsInt(movement.ToLanes, requiredLane) && len(movement.ToLanes) > 1 {
			phaseIndex = index
			break
		}
	}
	if phaseIndex < 0 {
		t.Fatal("generated plan omitted a split receiving-lane phase for AB to BS")
	}
	simulation.signals["B"].PhaseIndex, simulation.signals["B"].Stage = phaseIndex, "green"
	nextAlong := math.Min(simulation.graph.Simulation.Intersection, simulation.graph.Ways["BS"].Length)
	person := &personState{
		id: 1, state: "driving", wayID: "AB", lane: 2, lanePosition: 2,
		along: simulation.stopLine(simulation.graph.Ways["AB"]), route: []string{"AB", "BS", "SB"}, destinationLane: 0, destinationAlong: 120,
	}
	blocker := &personState{id: 2, state: "waiting", wayID: "BS", lane: requiredLane, lanePosition: float64(requiredLane), along: nextAlong}
	simulation.people = []*personState{person, blocker}

	simulation.transition(person)
	if person.state != "waiting" || person.waitingReason != "downstream storage" || person.routeIndex != 0 {
		t.Fatalf("vehicle entered a free lane incompatible with its next movement: %+v", person)
	}

	simulation.people = []*personState{person}
	simulation.transition(person)
	if person.state != "turning" || person.nextLane != requiredLane {
		t.Fatalf("vehicle did not enter the route-compatible lane after storage cleared: %+v", person)
	}
}

func TestIntersectionWaitsWhenAllReceivingLanesAreFull(t *testing.T) {
	simulation := New(multiLaneGraph(t))
	control := simulation.graph.Intersections["B"].TrafficControl
	phaseIndex := -1
	for index, phase := range control.Phases {
		movement, allowed := movementForLaneInPhase(phase, "AB", "BS", 2)
		if allowed && len(movement.ToLanes) == simulation.graph.Ways["BS"].Lanes {
			phaseIndex = index
			break
		}
	}
	if phaseIndex < 0 {
		t.Fatal("generated plan omitted the right turn from AB to BS")
	}
	simulation.signals["B"].PhaseIndex, simulation.signals["B"].Stage = phaseIndex, "green"
	nextAlong := math.Min(simulation.graph.Simulation.Intersection, simulation.graph.Ways["BS"].Length)
	person := &personState{
		id: 1, state: "driving", wayID: "AB", lane: 2, lanePosition: 2,
		along: simulation.stopLine(simulation.graph.Ways["AB"]), route: []string{"AB", "BS"}, destinationLane: 0, destinationAlong: 120,
	}
	simulation.people = []*personState{person}
	for lane := 0; lane < simulation.graph.Ways["BS"].Lanes; lane++ {
		simulation.people = append(simulation.people, &personState{
			id: 2 + lane, state: "waiting", wayID: "BS", lane: lane, lanePosition: float64(lane), along: nextAlong,
		})
	}

	simulation.transition(person)
	if person.state != "waiting" || person.waitingReason != "downstream storage" || person.routeIndex != 0 {
		t.Fatalf("vehicle entered a full downstream road: %+v", person)
	}
}

func TestCongestionRerouteAvoidsBlockedImmediateWay(t *testing.T) {
	graph, err := model.BuildGraph(model.CityConfig{
		Name: "Reroute City", Bounds: model.Bounds{MaxX: 320, MaxY: 220},
		Simulation: model.SimulationConfig{People: 1, DriveSpeed: 20, SafeGap: 10, VehicleSize: 4, Intersection: 18},
		Intersections: []model.IntersectionConfig{
			{ID: "A", Position: model.Vec2{X: 10, Y: 100}},
			{ID: "B", Position: model.Vec2{X: 90, Y: 100}},
			{ID: "C", Position: model.Vec2{X: 160, Y: 190}},
			{ID: "D", Position: model.Vec2{X: 230, Y: 100}},
			{ID: "E", Position: model.Vec2{X: 310, Y: 100}},
		},
		Ways: []model.WayConfig{
			{ID: "AB", From: "A", To: "B", Lanes: 1, SpeedLimit: 20},
			{ID: "BD", From: "B", To: "D", Lanes: 1, SpeedLimit: 20},
			{ID: "BC", From: "B", To: "C", Lanes: 1, SpeedLimit: 20},
			{ID: "CD", From: "C", To: "D", Lanes: 1, SpeedLimit: 20},
			{ID: "DE", From: "D", To: "E", Lanes: 1, SpeedLimit: 20},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	simulation := &Simulation{clock: &foundation.Clock{}, graph: graph, safeGap: graph.Simulation.SafeGap}
	person := &personState{id: 1, state: "waiting", wayID: "AB", lane: 0, route: []string{"AB", "BD", "DE"}}
	simulation.people = []*personState{person}

	if !simulation.rerouteAroundCongestion(person, "BD") {
		t.Fatal("blocked direct route did not produce an alternate path")
	}
	want := []string{"AB", "BC", "CD", "DE"}
	if len(person.route) != len(want) {
		t.Fatalf("rerouted path = %v, want %v", person.route, want)
	}
	for index := range want {
		if person.route[index] != want[index] {
			t.Fatalf("rerouted path = %v, want %v", person.route, want)
		}
	}
}

func TestPersonWaitsOffRoadUntilOriginLaneHasStorage(t *testing.T) {
	simulation := New(simulationGraph(t))
	entrant := &personState{
		id: 1, state: "walking_to_lane", position: model.Vec2{X: 40, Y: 40}, walkTarget: model.Vec2{X: 40, Y: 40},
		route: []string{"AB"}, lane: 0, lanePosition: 0, along: 30,
	}
	blocker := &personState{id: 2, state: "waiting", wayID: "AB", lane: 0, lanePosition: 0, along: 35}
	simulation.people = []*personState{entrant, blocker}

	simulation.advanceWalking(entrant, .1)
	if entrant.state != "walking_to_lane" || entrant.wayID != "" || entrant.waitingReason != "road entry" {
		t.Fatalf("person entered an occupied origin lane: %+v", entrant)
	}
	blocker.along = 50
	simulation.advanceWalking(entrant, .1)
	if entrant.state != "driving" || entrant.wayID != "AB" || entrant.waitingReason != "" {
		t.Fatalf("person did not enter after storage became available: %+v", entrant)
	}
}

func TestRuntimeTrafficLoadAndContinuousSpawning(t *testing.T) {
	simulation := New(simulationGraph(t))
	simulation.SetLoad(5, 10, 12)
	if got := len(simulation.people); got != 5 {
		t.Fatalf("population after SetLoad = %d, want 5", got)
	}
	simulation.advanceLocked(.5)
	if got := len(simulation.people); got != 10 {
		t.Fatalf("population after spawning = %d, want 10", got)
	}
	snapshot := simulation.Snapshot()
	if snapshot.Load.SpawnRate != 10 || snapshot.Load.MaxPopulation != 12 {
		t.Fatalf("unexpected load snapshot: %+v", snapshot.Load)
	}
}

func TestRoadVehiclesRemainOnTheirLaneCenters(t *testing.T) {
	simulation := New(simulationGraph(t))
	for step := 0; step < 200; step++ {
		simulation.advanceLocked(.05)
		for _, person := range simulation.people {
			if person.state != "driving" && person.state != "waiting" {
				continue
			}
			position, _ := simulation.graph.Ways[person.wayID].Sample(person.along, person.lane)
			if person.position.Sub(position).Length() > .001 {
				t.Fatalf("person %d left lane center by %.4f", person.id, person.position.Sub(position).Length())
			}
		}
	}
}

func TestExitLaneSelectionHonorsMovementBeforeFuturePreference(t *testing.T) {
	if got := chooseExitLane([]int{0}, 3, 0, 4, 4); got != 0 {
		t.Fatalf("left turn selected exit lane %d, want lane 0", got)
	}
	if got := chooseExitLane([]int{3}, 0, 3, 4, 4); got != 3 {
		t.Fatalf("right turn selected exit lane %d, want lane 3", got)
	}
	if got := chooseExitLane([]int{0, 1, 2, 3}, 2, 0, 4, 4); got != 2 {
		t.Fatalf("through movement ignored future lane preference: got %d", got)
	}
}

func TestRightTurnSelectsRouteCompatibleReceivingLane(t *testing.T) {
	simulation := New(multiLaneGraph(t))
	control := simulation.graph.Intersections["B"].TrafficControl
	phaseIndex := -1
	for index, phase := range control.Phases {
		movement, allowed := movementForLaneInPhase(phase, "AB", "BS", 2)
		if allowed && containsInt(movement.ToLanes, 0) {
			phaseIndex = index
			break
		}
	}
	if phaseIndex < 0 {
		t.Fatal("generated plan omitted the right turn from AB to BS")
	}
	runtime := simulation.signals["B"]
	runtime.PhaseIndex, runtime.Stage = phaseIndex, "green"
	person := &personState{
		id: 1, state: "driving", wayID: "AB", lane: 2, lanePosition: 2,
		along: simulation.stopLine(simulation.graph.Ways["AB"]), route: []string{"AB", "BS"}, destinationLane: 0, destinationAlong: 120,
	}
	simulation.transition(person)
	if person.state != "turning" || person.nextWayID != "BS" || person.nextLane != 0 {
		t.Fatalf("right turn did not choose the downstream destination lane: %+v", person)
	}
	movement, ok := control.Movement("AB", "BS", 2)
	if !ok || len(movement.ToLanes) != simulation.graph.Ways["BS"].Lanes {
		t.Fatalf("right turn does not expose all receiving lanes: %+v, ok=%v", movement, ok)
	}
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestRoutePreparationChangesOneAdjacentLaneAtATime(t *testing.T) {
	simulation := New(multiLaneGraph(t))
	person := &personState{id: 1, state: "driving", wayID: "AB", lane: 0, lanePosition: 0, along: 390, speed: 20, route: []string{"AB", "BS"}, destinationAlong: 100}
	simulation.people = []*personState{person}

	simulation.planLaneChangesLocked()
	if !person.laneChanging || person.laneChangeFrom != 0 || person.laneChangeTo != 1 || person.laneChangeReason != "route preparation" {
		t.Fatalf("first route lane change was not adjacent: %+v", person)
	}
	simulation.advanceLaneChangesLocked(person.laneChangeDuration)
	if person.lane != 1 || person.lanePosition != 1 {
		t.Fatalf("first route lane change did not complete in lane 1: %+v", person)
	}
	simulation.planLaneChangesLocked()
	if !person.laneChanging || person.laneChangeFrom != 1 || person.laneChangeTo != 2 {
		t.Fatalf("second route lane change did not continue toward lane 2: %+v", person)
	}
}

func TestVehicleOvertakesSlowLeaderOnPreferredPassingSide(t *testing.T) {
	simulation := New(multiLaneGraph(t))
	leader := &personState{id: 1, state: "waiting", wayID: "AB", lane: 1, lanePosition: 1, along: 280, route: []string{"AB"}, destinationAlong: 580, destinationLane: 1}
	follower := &personState{id: 2, state: "driving", wayID: "AB", lane: 1, lanePosition: 1, along: 240, speed: 20, route: []string{"AB"}, destinationAlong: 580, destinationLane: 1}
	simulation.people = []*personState{leader, follower}

	simulation.planLaneChangesLocked()
	if !follower.laneChanging || follower.laneChangeTo != 0 || follower.laneChangeReason != "overtake" {
		t.Fatalf("follower did not overtake into the left passing lane under right-hand traffic: %+v", follower)
	}
}

func TestVehicleDoesNotOvertakeOutOfNextMovementLane(t *testing.T) {
	simulation := New(multiLaneGraph(t))
	routeLanes := movementLanes(simulation.graph.Intersections["B"].TrafficControl, "AB", "BS")
	if len(routeLanes) != 1 {
		t.Fatalf("right turn lanes = %v, want one required lane", routeLanes)
	}
	lane := routeLanes[0]
	leader := &personState{id: 1, state: "waiting", wayID: "AB", lane: lane, lanePosition: float64(lane), along: 140, route: []string{"AB", "BS", "SB"}}
	follower := &personState{id: 2, state: "driving", wayID: "AB", lane: lane, lanePosition: float64(lane), along: 100, speed: 20, route: []string{"AB", "BS", "SB"}}
	simulation.people = []*personState{leader, follower}

	simulation.planLaneChangesLocked()
	if follower.laneChanging {
		t.Fatalf("vehicle left the only lane serving its next movement to overtake: %+v", follower)
	}
}

func TestLaneChangeRequiresSafeFrontAndRearGaps(t *testing.T) {
	simulation := New(multiLaneGraph(t))
	person := &personState{id: 1, state: "driving", wayID: "AB", lane: 1, lanePosition: 1, along: 260, speed: 20}
	front := &personState{id: 2, state: "driving", wayID: "AB", lane: 0, lanePosition: 0, along: 270, speed: 20}
	rear := &personState{id: 3, state: "driving", wayID: "AB", lane: 2, lanePosition: 2, along: 251, speed: 25}
	simulation.people = []*personState{person, front, rear}
	if simulation.laneChangeSafe(person, 0) {
		t.Fatal("lane change ignored an unsafe front gap")
	}
	if simulation.laneChangeSafe(person, 2) {
		t.Fatal("lane change ignored an unsafe closing rear vehicle")
	}
}

func TestLaneChangeProgressesWhileStoppedAndOccupiesBothLanes(t *testing.T) {
	simulation := New(multiLaneGraph(t))
	changer := &personState{id: 1, state: "waiting", wayID: "AB", lane: 1, lanePosition: 1, laneChanging: true, laneChangeFrom: 1, laneChangeTo: 0, laneChangeDuration: 1.4, laneChangeReason: "overtake", along: 300, route: []string{"AB"}, destinationAlong: 580, destinationLane: 1}
	follower := &personState{id: 2, state: "driving", wayID: "AB", lane: 0, lanePosition: 0, along: 287, speed: 20, route: []string{"AB"}, destinationAlong: 580, destinationLane: 0}
	simulation.people = []*personState{changer, follower}

	simulation.advanceLaneChangesLocked(.7)
	if math.Abs(changer.lanePosition-.5) > .001 || changer.laneChangeProgress <= 0 {
		t.Fatalf("stopped lane change did not advance laterally: %+v", changer)
	}
	simulation.advanceRoadVehiclesLocked(.1)
	if follower.state != "waiting" || follower.waitingFor != changer.id {
		t.Fatalf("target-lane follower ignored changing vehicle: changer=%+v follower=%+v", changer, follower)
	}
	snapshot := simulation.Snapshot()
	if snapshot.People[0].LaneChange == nil || snapshot.People[0].LaneChange.ToLane != 0 || snapshot.People[0].LanePosition <= 0 || snapshot.People[0].LanePosition >= 1 {
		t.Fatalf("snapshot omitted active lane-change state: %+v", snapshot.People[0])
	}
}

func TestAdaptiveSignalExtendsGapsOutAndHonorsMaximumGreen(t *testing.T) {
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
		t.Fatal("central intersection did not receive an adaptive multiphase controller")
	}
	control := intersection.TrafficControl
	phase := control.Phases[0]

	runtime.PhaseIndex, runtime.Stage, runtime.StageElapsed, runtime.GreenTarget = 0, "green", phase.DurationSeconds, phase.DurationSeconds
	runtime.PhasePressures = make([]PhasePressure, len(control.Phases))
	runtime.PhasePressures[0].Pressure, runtime.PhasePressures[1].Pressure = 12, 1
	simulation.advanceSignalLocked(control, runtime, .1)
	if runtime.GreenTarget <= phase.DurationSeconds || runtime.Decision != "queue extension" {
		t.Fatalf("heavy active queue did not extend green: %+v", runtime)
	}

	runtime.PhaseIndex, runtime.Stage, runtime.StageElapsed, runtime.GreenTarget = 0, "green", phase.MinGreenSeconds, phase.DurationSeconds
	runtime.PhasePressures[0].Pressure, runtime.PhasePressures[1].Pressure = 0, 6
	simulation.advanceSignalLocked(control, runtime, .1)
	if runtime.Stage != "amber" || runtime.Decision != "clearance after gap out" {
		t.Fatalf("empty active approach did not gap out: %+v", runtime)
	}

	runtime.PhaseIndex, runtime.Stage, runtime.StageElapsed, runtime.GreenTarget = 0, "green", phase.MaxGreenSeconds, phase.MaxGreenSeconds
	runtime.PhasePressures[0].Pressure, runtime.PhasePressures[1].Pressure = 8, 6
	simulation.advanceSignalLocked(control, runtime, .1)
	if runtime.Stage != "amber" || runtime.Decision != "clearance after maximum green" {
		t.Fatalf("maximum green did not terminate the active phase: %+v", runtime)
	}
}

func TestAdaptiveSignalCommitsDifferentPhaseAfterClearance(t *testing.T) {
	control := signalStateTestControl()
	runtime := &signalRuntime{
		PhaseIndex: 0, LastServedPhase: 0, SelectedPhase: 0, Stage: "green",
		StageElapsed: 2, GreenTarget: 2,
		PhasePressures: []PhasePressure{{PhaseIndex: 0, Pressure: 100}, {PhaseIndex: 1, Pressure: 6}, {PhaseIndex: 2, Pressure: 3}},
	}
	simulation := &Simulation{clock: &foundation.Clock{}}
	simulation.advanceSignalLocked(control, runtime, 1.1)
	if runtime.PhaseIndex != 1 || runtime.Stage != "green" || runtime.LastServedPhase != 0 {
		t.Fatalf("dominant phase immediately reselected itself after clearance: %+v", runtime)
	}
	if runtime.StageElapsed <= 0 || runtime.StageElapsed >= .11 {
		t.Fatalf("leftover interval was not applied to the new green: %+v", runtime)
	}
}

func TestAdaptiveSelectionRespondsDuringClearance(t *testing.T) {
	control := signalStateTestControl()
	runtime := &signalRuntime{
		PhaseIndex: 0, LastServedPhase: 0, Stage: "all_red", StageElapsed: .2, GreenTarget: 2,
		PhasePressures: []PhasePressure{{PhaseIndex: 0}, {PhaseIndex: 1, Pressure: 8}, {PhaseIndex: 2, Pressure: 2}},
	}
	simulation := &Simulation{clock: &foundation.Clock{}}
	simulation.advanceSignalLocked(control, runtime, 0)
	if runtime.SelectedPhase != 1 || runtime.ActivePressure != 0 || runtime.CompetingPressure != 8 {
		t.Fatalf("zero-delta refresh did not publish current pressure selection: %+v", runtime)
	}
	runtime.PhasePressures[1].Pressure, runtime.PhasePressures[2].Pressure = 1, 12
	simulation.advanceSignalLocked(control, runtime, .81)
	if runtime.PhaseIndex != 2 || runtime.Stage != "green" {
		t.Fatalf("controller ignored traffic change during clearance: %+v", runtime)
	}
}

func TestAdaptiveSignalConsumesLargeStepAcrossExactStageDurations(t *testing.T) {
	control := signalStateTestControl()
	runtime := &signalRuntime{
		PhaseIndex: 0, LastServedPhase: 0, Stage: "green", GreenTarget: 2,
		PhasePressures: []PhasePressure{{PhaseIndex: 0}, {PhaseIndex: 1}, {PhaseIndex: 2}},
	}
	(&Simulation{clock: &foundation.Clock{}}).advanceSignalLocked(control, runtime, 3.25)
	if runtime.PhaseIndex != 1 || runtime.Stage != "green" || math.Abs(runtime.StageElapsed-.25) > .000001 {
		t.Fatalf("large step lost time at a stage boundary: %+v", runtime)
	}
}

func TestSimulationRejectsNonFiniteClockDeltas(t *testing.T) {
	simulation := New(simulationGraph(t))
	before := simulation.Snapshot()
	simulation.Step(math.NaN())
	simulation.Tick(math.Inf(1))
	after := simulation.Snapshot()
	if after.Time != before.Time || after.Tick != before.Tick {
		t.Fatalf("non-finite delta corrupted simulation clock: before=%+v after=%+v", before, after)
	}
}

func signalStateTestControl() model.TrafficControl {
	phases := make([]model.TrafficPhaseConfig, 3)
	for index := range phases {
		phases[index] = model.TrafficPhaseConfig{
			ID: fmt.Sprintf("phase-%d", index), DurationSeconds: 2, MinGreenSeconds: 1, MaxGreenSeconds: 2,
			AmberSeconds: .5, AllRedSeconds: .5,
		}
	}
	return model.TrafficControl{Phases: phases}
}

func TestAdaptiveControllerImprovesSeededMeridianDemand(t *testing.T) {
	config, err := model.LoadConfig("../../example/Map1/road.json")
	if err != nil {
		t.Fatal(err)
	}
	config.Simulation.ArrivalDwell = 181
	fixedGraph, err := model.BuildGraph(config)
	if err != nil {
		t.Fatal(err)
	}
	adaptiveGraph, err := model.BuildGraph(config)
	if err != nil {
		t.Fatal(err)
	}
	fixed, adaptive := NewFixedTime(fixedGraph), New(adaptiveGraph)
	for elapsed := 0.0; elapsed < 180; elapsed += .1 {
		fixed.Step(.1)
		adaptive.Step(.1)
	}
	fixedResult, adaptiveResult := fixed.Snapshot().Performance, adaptive.Snapshot().Performance
	if adaptiveResult.SignalDelaySeconds >= fixedResult.SignalDelaySeconds {
		t.Fatalf("adaptive signal delay %.1f did not improve fixed-time %.1f", adaptiveResult.SignalDelaySeconds, fixedResult.SignalDelaySeconds)
	}
	queueLimit := int(math.Ceil(float64(fixedResult.MaximumSignalQueue) * 1.10))
	throughputFloor := int(math.Floor(float64(fixedResult.CompletedTrips) * .90))
	if adaptiveResult.CompletedTrips < throughputFloor || adaptiveResult.MaximumSignalQueue > queueLimit {
		t.Fatalf("adaptive controller exceeded the 10%% throughput or peak-queue guardrail: fixed=%+v adaptive=%+v", fixedResult, adaptiveResult)
	}
}

func TestMeridianRemainsLiveUnderLongRunningClosedLoad(t *testing.T) {
	config, err := model.LoadConfig("../../example/Map1/road.json")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := model.BuildGraph(config)
	if err != nil {
		t.Fatal(err)
	}
	simulation := New(graph)
	completedAt480 := 0
	for elapsed := 0.0; elapsed < 600; elapsed += .2 {
		simulation.Step(.2)
		if elapsed >= 480 && completedAt480 == 0 {
			completedAt480 = simulation.Snapshot().Performance.CompletedTrips
		}
	}
	final := simulation.Snapshot()
	if final.Performance.CompletedTrips <= completedAt480+10 {
		t.Fatalf("closed load stopped completing trips after congestion formed: at 480s=%d final=%+v", completedAt480, final.Performance)
	}
	if final.Stats.Driving == 0 || final.Stats.AverageSpeed <= 0 {
		t.Fatalf("closed load reached a zero-speed gridlock: %+v", final.Stats)
	}
}

func TestAdaptiveSignalsCycleEveryPhaseWithoutDemand(t *testing.T) {
	config, err := model.LoadConfig("../../example/Map1/road.json")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := model.BuildGraph(config)
	if err != nil {
		t.Fatal(err)
	}
	simulation := New(graph)
	simulation.people = nil
	simulation.spawnRate = 0
	seen := make(map[string]map[string]bool)
	for id := range graph.Intersections {
		seen[id] = make(map[string]bool)
	}
	for elapsed := 0.0; elapsed < 300; elapsed += .5 {
		simulation.Step(.5)
		for _, signal := range simulation.Snapshot().Signals {
			if signal.Stage == "green" {
				seen[signal.IntersectionID][signal.PhaseID] = true
			}
		}
	}
	for id, intersection := range graph.Intersections {
		if len(seen[id]) != len(intersection.TrafficControl.Phases) {
			t.Errorf("intersection %s served %d of %d phases without demand", id, len(seen[id]), len(intersection.TrafficControl.Phases))
		}
	}
}
