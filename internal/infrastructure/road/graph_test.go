package model

import (
	"math"
	"testing"
)

func testConfig() CityConfig {
	return CityConfig{
		Name: "Test City", Bounds: Bounds{MaxX: 200, MaxY: 100},
		Simulation: SimulationConfig{People: 2, WalkSpeed: 10, DriveSpeed: 20, SafeGap: 10, VehicleSize: 4},
		Intersections: []IntersectionConfig{
			{ID: "A", Name: "Alpha", Position: Vec2{X: 10, Y: 50}},
			{ID: "B", Name: "Beta", Position: Vec2{X: 190, Y: 50}},
		},
		Ways: []WayConfig{
			{ID: "AB", From: "A", To: "B", Lanes: 3, LaneWidth: 4, SpeedLimit: 20},
			{ID: "BA", From: "B", To: "A", Lanes: 1, LaneWidth: 4, SpeedLimit: 20},
		},
	}
}

func TestMultipleElevatedLevels(t *testing.T) {
	c := testConfig()
	for i := range c.Intersections {
		c.Intersections[i].Layer = 2
		c.Intersections[i].Elevation = 13
	}
	for i := range c.Ways {
		c.Ways[i].Structure = "bridge"
	}
	c.Intersections = append(c.Intersections, IntersectionConfig{ID: "C", Position: Vec2{X: 10, Y: 90}}, IntersectionConfig{ID: "D", Position: Vec2{X: 190, Y: 90}})
	c.Ways = append(c.Ways, WayConfig{ID: "CD", From: "C", To: "D", Lanes: 1, LaneWidth: 3.5, SpeedLimit: 10}, WayConfig{ID: "DC", From: "D", To: "C", Lanes: 1, LaneWidth: 3.5, SpeedLimit: 10})
	g, err := BuildGraph(c)
	if err != nil {
		t.Fatal(err)
	}
	if g.Ways["AB"].StartLayer != 2 || g.Ways["AB"].EndLayer != 2 {
		t.Fatal("lost numeric deck levels")
	}
	c.Intersections[1].Layer = 1
	if _, err = BuildGraph(c); err == nil {
		t.Fatal("bridge connected different floors without a ramp")
	}
}

func TestBuildGraphCreatesWeightedEdgesAndLaneTransforms(t *testing.T) {
	graph, err := BuildGraph(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	way := graph.Ways["AB"]
	if way.Weight != 3 || graph.Edges[0].Weight != 3 {
		t.Fatalf("edge weight = %d, want lane count 3", way.Weight)
	}
	if got := len(way.Segments[0].SubSegments); got != 3 {
		t.Fatalf("sub-segments = %d, want 3", got)
	}
	// Right-hand traffic puts eastbound AB below the shared centre line; lane
	// zero remains its left/median lane and lane centers stay one width apart.
	wantY := []float64{53.5, 57.5, 61.5}
	for i, segment := range way.Segments[0].SubSegments {
		if math.Abs(segment.Transform[1][2]-wantY[i]) > 0.001 {
			t.Errorf("lane %d transform y = %.2f, want %.2f", i, segment.Transform[1][2], wantY[i])
		}
	}
	if got := len(graph.Intersections["A"].Approaches); got != 2 {
		t.Fatalf("A approaches = %d, want inbound BA + outbound AB", got)
	}
}

func TestOpposingCarriagewaysAreSeparated(t *testing.T) {
	graph, err := BuildGraph(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	ab, _ := graph.Ways["AB"].Sample(90, 1)
	ba, _ := graph.Ways["BA"].Sample(90, 0)
	if math.Abs(ab.Y-ba.Y) < 4 {
		t.Fatalf("opposing carriageways overlap: AB y=%.1f BA y=%.1f", ab.Y, ba.Y)
	}
}

func TestDrivingSideControlsCarriagewayHandedness(t *testing.T) {
	right, err := BuildGraph(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	rightPosition, _ := right.Ways["AB"].Sample(90, 0)
	if right.DrivingSide != DrivingSideRight || rightPosition.Y <= 50 {
		t.Fatalf("default traffic should drive on the right: side=%q position=%+v", right.DrivingSide, rightPosition)
	}

	leftConfig := testConfig()
	leftConfig.DrivingSide = DrivingSideLeft
	left, err := BuildGraph(leftConfig)
	if err != nil {
		t.Fatal(err)
	}
	leftPosition, _ := left.Ways["AB"].Sample(90, 0)
	if leftPosition.Y >= 50 {
		t.Fatalf("left-hand traffic should put eastbound lanes above the centre line: %+v", leftPosition)
	}

	invalid := testConfig()
	invalid.DrivingSide = "middle"
	if _, err := BuildGraph(invalid); err == nil {
		t.Fatal("expected an unsupported driving-side error")
	}
	if got := lanesForTurn(3, "u_turn", DrivingSideRight); len(got) != 1 || got[0] != 0 {
		t.Fatalf("right-hand U-turn lanes = %v, want median lane 0", got)
	}
	if got := lanesForTurn(3, "u_turn", DrivingSideLeft); len(got) != 1 || got[0] != 2 {
		t.Fatalf("left-hand U-turn lanes = %v, want median lane 2", got)
	}
}

func TestWayProjectionAndLaneSample(t *testing.T) {
	graph, err := BuildGraph(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	way := graph.Ways["AB"]
	projection := way.Project(Vec2{X: 100, Y: 75})
	if math.Abs(projection.Along-90) > 0.001 || math.Abs(projection.Position.Y-50) > 0.001 {
		t.Fatalf("unexpected projection: %+v", projection)
	}
	position, tangent := way.Sample(90, 0)
	if position != (Vec2{X: 100, Y: 53.5}) || tangent != (Vec2{X: 1, Y: 0}) {
		t.Fatalf("unexpected lane sample: position=%+v tangent=%+v", position, tangent)
	}
}

func TestBuildGraphRejectsUnknownIntersection(t *testing.T) {
	cfg := testConfig()
	cfg.Ways[0].To = "missing"
	if _, err := BuildGraph(cfg); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestDefaultVehicleSpacingIsTwoPointFiveMeters(t *testing.T) {
	cfg := testConfig()
	cfg.Simulation.SafeGap = 0
	graph, err := BuildGraph(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if graph.Simulation.SafeGap != 2.5 {
		t.Fatalf("default safe gap = %v, want 2.5", graph.Simulation.SafeGap)
	}
}

func TestTrafficPlanSwitchesAndValidatesIncomingWays(t *testing.T) {
	cfg := testConfig()
	cfg.Intersections[1].Traffic = TrafficControlConfig{OffsetSeconds: 1, Phases: []TrafficPhaseConfig{
		{ID: "red", Name: "All red", DurationSeconds: 3},
		{ID: "green", Name: "Alpha inbound", DurationSeconds: 5, AllowedIncoming: []string{"AB"}},
	}}
	graph, err := BuildGraph(cfg)
	if err != nil {
		t.Fatal(err)
	}
	control := graph.Intersections["B"].TrafficControl
	if got := control.Active(0); got.Phase.ID != "red" || math.Abs(got.Remaining-2) > .001 {
		t.Fatalf("unexpected phase at t=0: %+v", got)
	}
	if control.Allows("AB", 0) {
		t.Fatal("all-red phase allowed AB")
	}
	if !control.Allows("AB", 3) {
		t.Fatal("green phase blocked AB")
	}

	cfg.Intersections[1].Traffic.Phases[1].AllowedIncoming = []string{"BA"}
	if _, err := BuildGraph(cfg); err == nil {
		t.Fatal("expected non-incoming signal way validation error")
	}
}

func TestShortestDirectedPathAndNearestLane(t *testing.T) {
	graph, err := BuildGraph(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	path, ok := graph.ShortestPath("A", "B")
	if !ok || len(path) != 1 || path[0].ID != "AB" {
		t.Fatalf("unexpected path: %+v, ok=%v", path, ok)
	}
	direct, ok := graph.RouteBetweenWays("AB", "AB", true)
	if !ok || !equalStrings(wayIDs(direct), []string{"AB"}) {
		t.Fatalf("same-way direct route = %v, ok=%v", wayIDs(direct), ok)
	}
	cycle, ok := graph.RouteBetweenWays("AB", "AB", false)
	if !ok || !equalStrings(wayIDs(cycle), []string{"AB", "BA", "AB"}) {
		t.Fatalf("same-way return route is not connector-valid: %v, ok=%v", wayIDs(cycle), ok)
	}
	nearest := graph.NearestLane(Vec2{X: 100, Y: 60.5})
	if nearest.Way.ID != "AB" || nearest.Lane != 2 {
		t.Fatalf("nearest lane = %s/%d, want AB/2", nearest.Way.ID, nearest.Lane)
	}
}

func TestGeneratedSignalPlanUsesProtectedMovementsDemandAndClearance(t *testing.T) {
	cfg := CityConfig{Name: "Planner", Bounds: Bounds{MaxX: 200, MaxY: 200}, Intersections: []IntersectionConfig{
		{ID: "C", Position: Vec2{X: 100, Y: 100}}, {ID: "W", Position: Vec2{X: 10, Y: 100}},
		{ID: "E", Position: Vec2{X: 190, Y: 100}}, {ID: "N", Position: Vec2{X: 100, Y: 10}}, {ID: "S", Position: Vec2{X: 100, Y: 190}},
	}}
	addPair := func(a, b, class string, lanes int, demand float64) {
		cfg.Ways = append(cfg.Ways,
			WayConfig{ID: a + b, From: a, To: b, Lanes: lanes, RoadClass: class, Demand: demand},
			WayConfig{ID: b + a, From: b, To: a, Lanes: lanes, RoadClass: class, Demand: demand})
	}
	addPair("W", "C", "arterial", 4, 2.4)
	addPair("E", "C", "arterial", 4, 2.4)
	addPair("N", "C", "local", 1, .5)
	addPair("S", "C", "local", 1, .5)
	graph, err := BuildGraph(cfg)
	if err != nil {
		t.Fatal(err)
	}
	control := graph.Intersections["C"].TrafficControl
	if !control.Generated || len(control.Phases) < 4 {
		t.Fatalf("expected generated protected plan, got %+v", control)
	}
	if !control.Adaptive || control.TimingMethod == "" || control.LostTime <= 0 || control.VolumeRatio <= 0 || control.MaxRedSeconds != control.CycleSeconds {
		t.Fatalf("generated plan lacks optimized controller metadata: %+v", control)
	}
	maxArterial, maxLocal := 0.0, 0.0
	for _, phase := range control.Phases {
		if phase.AmberSeconds <= 0 || phase.AllRedSeconds <= 0 || phase.DemandScore <= 0 || phase.FlowRatio <= 0 {
			t.Errorf("phase lacks clearance/demand metadata: %+v", phase)
		}
		if phase.MinGreenSeconds > phase.DurationSeconds || phase.DurationSeconds > phase.MaxGreenSeconds {
			t.Errorf("phase green is outside its actuation bounds: %+v", phase)
		}
		for _, movement := range phase.AllowedMovements {
			if movement.Turn != "through" {
				continue
			}
			if graph.Ways[movement.FromWayID].RoadClass == "arterial" {
				maxArterial = math.Max(maxArterial, phase.DurationSeconds)
			} else {
				maxLocal = math.Max(maxLocal, phase.DurationSeconds)
			}
		}
	}
	if maxArterial < maxLocal {
		t.Fatalf("arterial through green %.1f should not be shorter than local through green %.1f", maxArterial, maxLocal)
	}
	first := control.Phases[0]
	movement := first.AllowedMovements[0]
	if control.Active(first.DurationSeconds+.1).Stage != "amber" || control.AllowsMovement(movement.FromWayID, movement.ToWayID, movement.FromLanes[0], first.DurationSeconds+.1) {
		t.Fatal("amber clearance still permitted a movement")
	}
	for _, incoming := range graph.Incoming["C"] {
		for _, outgoing := range graph.Outgoing["C"] {
			turn := classifyMovement(incoming, outgoing, graph.MovementRules)
			if turn == "u_turn" && len(graph.Outgoing["C"]) > 1 {
				continue
			}
			targets := make(map[int]bool)
			for fromLane := 0; fromLane < incoming.Lanes; fromLane++ {
				movement, ok := control.Movement(incoming.ID, outgoing.ID, fromLane)
				if !ok {
					continue
				}
				for _, target := range movement.TargetLanes(fromLane) {
					targets[target] = true
				}
			}
			if len(targets) != outgoing.Lanes {
				t.Errorf("movement %s -> %s collectively reaches %d of %d receiving lanes", incoming.ID, outgoing.ID, len(targets), outgoing.Lanes)
			}
		}
	}
}

func TestMovementLookupNarrowsReceivingLanesToExactSourceLane(t *testing.T) {
	connections := connectLaneSets([]int{0, 1}, []int{0, 1, 2, 3})
	movement := TrafficMovementConfig{
		FromWayID: "in", ToWayID: "out", FromLanes: []int{0, 1}, ToLanes: []int{0, 1, 2, 3},
		LaneConnections: connections, Turn: "through",
	}
	control := TrafficControl{Phases: []TrafficPhaseConfig{{AllowedMovements: []TrafficMovementConfig{movement}}}}
	first, ok := control.Movement("in", "out", 0)
	if !ok || !equalInts(first.ToLanes, []int{0, 1}) || len(first.LaneConnections) != 2 {
		t.Fatalf("lane 0 lookup returned unrelated targets: %+v, ok=%v", first, ok)
	}
	second, ok := control.Movement("in", "out", 1)
	if !ok || !equalInts(second.ToLanes, []int{2, 3}) || len(second.LaneConnections) != 2 {
		t.Fatalf("lane 1 lookup returned unrelated targets: %+v, ok=%v", second, ok)
	}
	split := TrafficControl{Phases: []TrafficPhaseConfig{
		{AllowedMovements: []TrafficMovementConfig{{FromWayID: "in", ToWayID: "out", FromLanes: []int{0}, ToLanes: []int{0}, LaneConnections: []LaneConnectionConfig{{FromLane: 0, ToLane: 0}}}}},
		{AllowedMovements: []TrafficMovementConfig{{FromWayID: "in", ToWayID: "out", FromLanes: []int{0}, ToLanes: []int{1}, LaneConnections: []LaneConnectionConfig{{FromLane: 0, ToLane: 1}}}}},
	}}
	combined, ok := split.Movement("in", "out", 0)
	if !ok || !equalInts(combined.ToLanes, []int{0, 1}) || len(combined.LaneConnections) != 2 {
		t.Fatalf("movement lookup did not combine topology split across phases: %+v, ok=%v", combined, ok)
	}
}

func TestGeneratedLaneConnectionsCoverBothRoadsWithoutCrossing(t *testing.T) {
	for fromCount := 1; fromCount <= 6; fromCount++ {
		for toCount := 1; toCount <= 6; toCount++ {
			connections := connectLaneSets(allLanes(fromCount), allLanes(toCount))
			fromSeen, toSeen := make(map[int]bool), make(map[int]bool)
			for first, a := range connections {
				fromSeen[a.FromLane], toSeen[a.ToLane] = true, true
				for _, b := range connections[first+1:] {
					if a.FromLane < b.FromLane && a.ToLane > b.ToLane {
						t.Fatalf("%d -> %d connector topology crosses: %+v then %+v", fromCount, toCount, a, b)
					}
				}
			}
			if len(fromSeen) != fromCount || len(toSeen) != toCount {
				t.Fatalf("%d -> %d topology does not cover every lane: %+v", fromCount, toCount, connections)
			}
		}
	}
}

func TestApproachLaneAllocationFollowsClockwiseDistanceRules(t *testing.T) {
	rules := MovementRulesConfig{ThroughToleranceDegrees: 5, UTurnToleranceDegrees: 5}
	wantOrder := []string{"left-near", "left-far", "right-far", "right-near"}
	tests := []struct {
		name      string
		lanes     int
		wantLanes [][]int
	}{
		{name: "scarce balanced merge", lanes: 2, wantLanes: [][]int{{0}, {0}, {1}, {1}}},
		{name: "scarce far priority", lanes: 3, wantLanes: [][]int{{0}, {1}, {2}, {2}}},
		{name: "one dedicated lane each", lanes: 4, wantLanes: [][]int{{0}, {1}, {2}, {3}}},
		{name: "surplus reinforces far directions", lanes: 6, wantLanes: [][]int{{0}, {1, 2}, {3, 4}, {5}}},
		{name: "double dedicated lanes", lanes: 8, wantLanes: [][]int{{0, 1}, {2, 3}, {4, 5}, {6, 7}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			from, outgoing := radialLaneAllocationWays(test.lanes)
			movements := makeApproachMovements(from, outgoing, rules, DrivingSideRight)
			if len(movements) != len(wantOrder) {
				t.Fatalf("movements = %d, want %d", len(movements), len(wantOrder))
			}
			for index, movement := range movements {
				if movement.ToWayID != wantOrder[index] {
					t.Errorf("target %d = %s, want clockwise target %s", index, movement.ToWayID, wantOrder[index])
				}
				if !equalInts(movement.FromLanes, test.wantLanes[index]) {
					t.Errorf("target %s lanes = %v, want %v", movement.ToWayID, movement.FromLanes, test.wantLanes[index])
				}
				targetLanes := make(map[int]bool)
				for _, connection := range movement.LaneConnections {
					targetLanes[connection.ToLane] = true
				}
				if len(targetLanes) != len(movement.ToLanes) {
					t.Errorf("target %s reaches %d receiving lanes through %+v", movement.ToWayID, len(targetLanes), movement.LaneConnections)
				}
			}
		})
	}
}

func radialLaneAllocationWays(incomingLanes int) (*Way, []*Way) {
	center := Vec2{}
	from := straightLaneAllocationWay("in", "origin", "center", Vec2{X: -100}, center, incomingLanes)
	target := func(id string, degrees float64) *Way {
		angle := degrees * math.Pi / 180
		end := Vec2{X: 100 * math.Cos(angle), Y: 100 * math.Sin(angle)}
		return straightLaneAllocationWay(id, "center", id, center, end, 2)
	}
	// Deliberately shuffled: allocation must use geometry rather than way IDs or
	// configuration order.
	return from, []*Way{
		target("right-near", 120), target("left-far", -40),
		target("left-near", -120), target("right-far", 40),
	}
}

func straightLaneAllocationWay(id, from, to string, start, end Vec2, lanes int) *Way {
	length := end.Sub(start).Length()
	return &Way{
		ID: id, From: from, To: to, Lanes: lanes, LaneWidth: 4,
		Length: length, Points: []Vec2{start, end}, cumulative: []float64{0, length},
	}
}

func TestFourWayPlanPairsOpposingFlowsAndKeepsEveryPhaseMaximal(t *testing.T) {
	graph := buildJunctionGraph(t, map[string]Vec2{
		"W": {X: 10, Y: 110}, "E": {X: 210, Y: 110},
		"N": {X: 110, Y: 10}, "S": {X: 110, Y: 210},
	})
	control := graph.Intersections["C"].TrafficControl
	if len(control.Phases) != 4 {
		t.Fatalf("four-way plan has %d phases, want two opposing-through and two protected-turn patterns", len(control.Phases))
	}
	if !phaseContainsBoth(control.Phases, "WC", "CE", "EC", "CW") {
		t.Fatal("opposing east/west through movements never receive green together")
	}
	if !phaseContainsBoth(control.Phases, "WC", "CN", "EC", "CS") {
		t.Fatal("compatible opposing east/west left turns never receive green together")
	}
	assertGeneratedPhasesAreSafeAndMaximal(t, graph, "C")
}

func TestThreeWayPlanCombinesPerpendicularNonCrossingTurns(t *testing.T) {
	graph := buildJunctionGraph(t, map[string]Vec2{
		"W": {X: 10, Y: 110}, "E": {X: 210, Y: 110}, "S": {X: 110, Y: 210},
	})
	control := graph.Intersections["C"].TrafficControl
	if len(control.Phases) > 4 {
		t.Fatalf("three-way plan uses %d phases; compatible movements were not consolidated", len(control.Phases))
	}
	if !phaseContainsBoth(control.Phases, "WC", "CE", "EC", "CW") {
		t.Fatal("opposing through movements at the top of the T never receive green together")
	}
	// These right turns originate on perpendicular approaches but occupy
	// separate corners. The former direction-only validator split them.
	if !phaseContainsBoth(control.Phases, "WC", "CS", "SC", "CE") {
		t.Fatal("perpendicular non-crossing right turns never receive green together")
	}
	incoming := map[string]*Way{"WC": graph.Ways["WC"], "SC": graph.Ways["SC"]}
	outgoing := map[string]*Way{"CE": graph.Ways["CE"]}
	a, _ := control.Movement("WC", "CE", 0)
	b, _ := control.Movement("SC", "CE", 1)
	a.ToLanes, a.LaneConnections = []int{0}, []LaneConnectionConfig{{FromLane: 0, ToLane: 0}}
	b.ToLanes, b.LaneConnections = []int{1}, []LaneConnectionConfig{{FromLane: 1, ToLane: 1}}
	if movementsConflict(a, b, incoming, outgoing, graph.Simulation.Intersection) {
		t.Fatal("through traffic and a turn into separate lanes of the same exit were marked conflicting")
	}
	if !phaseContainsConnectorPair(control.Phases,
		"WC", "CE", LaneConnectionConfig{FromLane: 0, ToLane: 0},
		"SC", "CE", LaneConnectionConfig{FromLane: 1, ToLane: 1}) {
		t.Fatal("safe lane-separated through and turn connectors never receive green together")
	}
	assertGeneratedPhasesAreSafeAndMaximal(t, graph, "C")
}

func buildJunctionGraph(t *testing.T, arms map[string]Vec2) *Graph {
	t.Helper()
	cfg := CityConfig{
		Name: "Junction phase test", Bounds: Bounds{MaxX: 220, MaxY: 220},
		Simulation:    SimulationConfig{Intersection: 58},
		Intersections: []IntersectionConfig{{ID: "C", Position: Vec2{X: 110, Y: 110}}},
	}
	order := []string{"W", "E", "N", "S"}
	for _, id := range order {
		position, exists := arms[id]
		if !exists {
			continue
		}
		cfg.Intersections = append(cfg.Intersections, IntersectionConfig{ID: id, Position: position})
		cfg.Ways = append(cfg.Ways,
			WayConfig{ID: id + "C", From: id, To: "C", Lanes: 2, LaneWidth: 7, Demand: 1},
			WayConfig{ID: "C" + id, From: "C", To: id, Lanes: 2, LaneWidth: 7, Demand: 1},
		)
	}
	graph, err := BuildGraph(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func phaseContainsBoth(phases []TrafficPhaseConfig, fromA, toA, fromB, toB string) bool {
	for _, phase := range phases {
		first, second := false, false
		for _, movement := range phase.AllowedMovements {
			first = first || movement.FromWayID == fromA && movement.ToWayID == toA
			second = second || movement.FromWayID == fromB && movement.ToWayID == toB
		}
		if first && second {
			return true
		}
	}
	return false
}

func phaseContainsConnectorPair(phases []TrafficPhaseConfig, fromA, toA string, connectionA LaneConnectionConfig, fromB, toB string, connectionB LaneConnectionConfig) bool {
	for _, phase := range phases {
		first, second := false, false
		for _, movement := range phase.AllowedMovements {
			if len(movement.LaneConnections) != 1 {
				continue
			}
			connection := movement.LaneConnections[0]
			first = first || movement.FromWayID == fromA && movement.ToWayID == toA && connection == connectionA
			second = second || movement.FromWayID == fromB && movement.ToWayID == toB && connection == connectionB
		}
		if first && second {
			return true
		}
	}
	return false
}

func assertGeneratedPhasesAreSafeAndMaximal(t *testing.T, graph *Graph, intersectionID string) {
	t.Helper()
	intersection := graph.Intersections[intersectionID]
	incoming, outgoing := make(map[string]*Way), make(map[string]*Way)
	for _, way := range graph.Incoming[intersectionID] {
		incoming[way.ID] = way
	}
	for _, way := range graph.Outgoing[intersectionID] {
		outgoing[way.ID] = way
	}
	var all []TrafficMovementConfig
	for _, phase := range intersection.TrafficControl.Phases {
		if err := validateCompatiblePhase(intersectionID, phase, incoming, outgoing, graph.Simulation.Intersection); err != nil {
			t.Fatal(err)
		}
		for _, movement := range phase.AllowedMovements {
			if !containsAtomicMovement(all, movement) {
				all = append(all, movement)
			}
		}
	}
	for _, phase := range intersection.TrafficControl.Phases {
		for _, candidate := range all {
			if containsAtomicMovement(phase.AllowedMovements, candidate) {
				continue
			}
			compatible := true
			for _, existing := range phase.AllowedMovements {
				if movementsConflict(candidate, existing, incoming, outgoing, graph.Simulation.Intersection) {
					compatible = false
					break
				}
			}
			if compatible {
				t.Fatalf("phase %q omits compatible connector %+v", phase.ID, candidate)
			}
		}
	}
}

func containsAtomicMovement(movements []TrafficMovementConfig, candidate TrafficMovementConfig) bool {
	if len(candidate.LaneConnections) != 1 {
		return false
	}
	connection := candidate.LaneConnections[0]
	for _, movement := range movements {
		if movement.FromWayID == candidate.FromWayID && movement.ToWayID == candidate.ToWayID &&
			len(movement.LaneConnections) == 1 && movement.LaneConnections[0] == connection {
			return true
		}
	}
	return false
}

func TestExplicitLaneConnectionsArePreservedAndValidated(t *testing.T) {
	movement := TrafficMovementConfig{
		FromWayID: "WC", ToWayID: "CS", FromLanes: []int{1}, ToLanes: []int{0, 2}, Turn: "right",
		LaneConnections: []LaneConnectionConfig{{FromLane: 1, ToLane: 0}, {FromLane: 1, ToLane: 2}},
	}
	cfg := CityConfig{Name: "Explicit connectors", Bounds: Bounds{MaxX: 220, MaxY: 220}, Intersections: []IntersectionConfig{
		{ID: "W", Position: Vec2{X: 10, Y: 100}},
		{ID: "C", Position: Vec2{X: 100, Y: 100}, Traffic: TrafficControlConfig{Phases: []TrafficPhaseConfig{{ID: "turn", DurationSeconds: 10, AllowedMovements: []TrafficMovementConfig{movement}}}}},
		{ID: "S", Position: Vec2{X: 100, Y: 200}},
	}, Ways: []WayConfig{
		{ID: "WC", From: "W", To: "C", Lanes: 2}, {ID: "CS", From: "C", To: "S", Lanes: 3},
	}}
	graph, err := BuildGraph(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := graph.Intersections["C"].TrafficControl.Movement("WC", "CS", 1)
	if !ok || !equalInts(got.ToLanes, []int{0, 2}) || len(got.LaneConnections) != 2 {
		t.Fatalf("explicit connector topology changed: %+v, ok=%v", got, ok)
	}
	if _, ok := graph.Intersections["C"].TrafficControl.Movement("WC", "CS", 0); ok {
		t.Fatal("explicit movement unexpectedly allowed an undeclared source lane")
	}

	cfg.Intersections[1].Traffic.Phases[0].AllowedMovements[0].LaneConnections = append(
		cfg.Intersections[1].Traffic.Phases[0].AllowedMovements[0].LaneConnections,
		LaneConnectionConfig{FromLane: 1, ToLane: 2},
	)
	if _, err := BuildGraph(cfg); err == nil {
		t.Fatal("duplicate explicit lane connector was accepted")
	}

	crossing := movement
	crossing.FromLanes = []int{0, 1}
	crossing.ToLanes = []int{0, 2}
	crossing.LaneConnections = []LaneConnectionConfig{{FromLane: 0, ToLane: 2}, {FromLane: 1, ToLane: 0}}
	cfg.Intersections[1].Traffic.Phases[0].AllowedMovements[0] = crossing
	if _, err := BuildGraph(cfg); err == nil {
		t.Fatal("crossing explicit lane connectors were accepted")
	}
}

func TestShortestPathRejectsRoadSequenceWithoutIntersectionConnector(t *testing.T) {
	cfg := CityConfig{Name: "Connected routing", Bounds: Bounds{MaxX: 220, MaxY: 140}, Intersections: []IntersectionConfig{
		{ID: "A", Position: Vec2{X: 10, Y: 60}}, {ID: "B", Position: Vec2{X: 100, Y: 60}},
		{ID: "C", Position: Vec2{X: 200, Y: 60}}, {ID: "D", Position: Vec2{X: 110, Y: 120}},
	}, Ways: []WayConfig{
		{ID: "AB", From: "A", To: "B", Lanes: 1}, {ID: "BC", From: "B", To: "C", Lanes: 1},
		{ID: "BD", From: "B", To: "D", Lanes: 1}, {ID: "DC", From: "D", To: "C", Lanes: 1},
	}}
	graph, err := BuildGraph(cfg)
	if err != nil {
		t.Fatal(err)
	}
	control := &graph.Intersections["B"].TrafficControl
	for phaseIndex := range control.Phases {
		kept := control.Phases[phaseIndex].AllowedMovements[:0]
		for _, movement := range control.Phases[phaseIndex].AllowedMovements {
			if movement.FromWayID != "AB" || movement.ToWayID != "BC" {
				kept = append(kept, movement)
			}
		}
		control.Phases[phaseIndex].AllowedMovements = kept
	}
	path, ok := graph.ShortestPath("A", "C")
	if !ok || len(path) != 3 || path[0].ID != "AB" || path[1].ID != "BD" || path[2].ID != "DC" {
		t.Fatalf("router used a disconnected shortcut: path=%v ok=%v", wayIDs(path), ok)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func wayIDs(ways []*Way) []string {
	ids := make([]string, len(ways))
	for index, way := range ways {
		ids[index] = way.ID
	}
	return ids
}

func TestMovementClassificationUsesScreenCoordinatesAndConfiguredTolerance(t *testing.T) {
	makeWay := func(id string, dx, dy float64) *Way {
		return &Way{ID: id, Lanes: 1, LaneWidth: 4, Length: 10, Points: []Vec2{{}, {X: dx * 10, Y: dy * 10}}, cumulative: []float64{0, 10}}
	}
	rules := MovementRulesConfig{ThroughToleranceDegrees: 40, UTurnToleranceDegrees: 35}
	east := makeWay("east", 1, 0)
	north := makeWay("north", 0, -1)
	south := makeWay("south", 0, 1)
	if got := classifyMovement(east, north, rules); got != "left" {
		t.Fatalf("east to north = %q, want left", got)
	}
	if got := classifyMovement(east, south, rules); got != "right" {
		t.Fatalf("east to south = %q, want right", got)
	}

	angle := 40.5 * math.Pi / 180
	branch := makeWay("branch", math.Cos(angle), math.Sin(angle))
	if got := classifyMovement(east, branch, rules); got != "right" {
		t.Fatalf("40.5-degree branch = %q with 40-degree tolerance, want right", got)
	}
	rules.ThroughToleranceDegrees = 41
	if got := classifyMovement(east, branch, rules); got != "through" {
		t.Fatalf("40.5-degree branch = %q with 41-degree tolerance, want through", got)
	}
}

func TestExplicitMovementRejectsGeometricallyReversedTurn(t *testing.T) {
	cfg := CityConfig{Name: "Turn validation", Bounds: Bounds{MaxX: 200, MaxY: 200}, Intersections: []IntersectionConfig{
		{ID: "W", Position: Vec2{X: 10, Y: 100}},
		{ID: "C", Position: Vec2{X: 100, Y: 100}, Traffic: TrafficControlConfig{Phases: []TrafficPhaseConfig{{
			ID: "bad", DurationSeconds: 10, AllowedMovements: []TrafficMovementConfig{{FromWayID: "WC", ToWayID: "CN", Turn: "right"}},
		}}}},
		{ID: "N", Position: Vec2{X: 100, Y: 10}},
	}, Ways: []WayConfig{
		{ID: "WC", From: "W", To: "C", Lanes: 1}, {ID: "CW", From: "C", To: "W", Lanes: 1},
		{ID: "NC", From: "N", To: "C", Lanes: 1}, {ID: "CN", From: "C", To: "N", Lanes: 1},
	}}
	if _, err := BuildGraph(cfg); err == nil {
		t.Fatal("expected a reversed explicit turn label to be rejected")
	}
}
