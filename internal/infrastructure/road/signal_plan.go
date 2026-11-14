package model

import (
	"fmt"
	"math"
	"sort"
)

const generatedPlanBasis = "clockwise distance-prioritized lane allocation; maximal conflict-free geometric connector sets; Webster green bounds with differential-queue max-pressure scheduling"

type phaseDraft struct {
	phase TrafficPhaseConfig
}

func planTrafficControl(intersection *Intersection, input TrafficControlConfig, incoming, outgoing []*Way, rules MovementRulesConfig, drivingSide string, intersectionRadius float64) (TrafficControl, error) {
	if input.Mode != "" && input.Mode != "flow" {
		return TrafficControl{}, fmt.Errorf("unknown control mode %s", input.Mode)
	}
	if input.Mode == "flow" {
		if len(incoming) != 1 && len(outgoing) != 1 {
			return TrafficControl{}, fmt.Errorf("flow node %s must be a simple merge or split", intersection.ID)
		}
		phase := TrafficPhaseConfig{ID: "flow", Name: "Merge / diverge", DurationSeconds: 3600}
		for _, from := range incoming {
			phase.AllowedIncoming = append(phase.AllowedIncoming, from.ID)
			phase.AllowedMovements = append(phase.AllowedMovements, makeApproachMovements(from, outgoing, rules, drivingSide)...)
		}
		if len(input.Connections) > 0 {
			byIn, byOut := map[string]*Way{}, map[string]*Way{}
			for _, w := range incoming {
				byIn[w.ID] = w
			}
			for _, w := range outgoing {
				byOut[w.ID] = w
			}
			covered := map[string]map[int]bool{}
			phase.AllowedMovements = nil
			for _, m := range input.Connections {
				a, b := byIn[m.FromWayID], byOut[m.ToWayID]
				if a == nil || b == nil {
					return TrafficControl{}, fmt.Errorf("flow node %s references an unrelated road", intersection.ID)
				}
				if err := validateLaneIndices(m.FromLanes, a.Lanes); err != nil {
					return TrafficControl{}, err
				}
				if err := validateLaneIndices(m.ToLanes, b.Lanes); err != nil {
					return TrafficControl{}, err
				}
				if err := validateLaneConnections(m); err != nil {
					return TrafficControl{}, err
				}
				if len(m.LaneConnections) == 0 {
					return TrafficControl{}, fmt.Errorf("empty flow lane connections")
				}
				m.Turn = classifyMovement(a, b, rules)
				if covered[a.ID] == nil {
					covered[a.ID] = map[int]bool{}
				}
				for _, c := range m.LaneConnections {
					covered[a.ID][c.FromLane] = true
				}
				phase.AllowedMovements = append(phase.AllowedMovements, m)
			}
			for _, w := range incoming {
				if len(covered[w.ID]) != w.Lanes {
					return TrafficControl{}, fmt.Errorf("flow node %s leaves incoming lanes disconnected", intersection.ID)
				}
			}
		}
		return TrafficControl{Mode: "flow", Phases: []TrafficPhaseConfig{phase}, CycleSeconds: 3600, PlanningBasis: "Unsignalized merge / diverge with receiving-lane storage checks", TimingMethod: "Continuous flow"}, nil
	}
	generated := len(input.Phases) == 0
	phases := append([]TrafficPhaseConfig(nil), input.Phases...)
	if generated {
		phases = generateTrafficPhases(intersection, incoming, outgoing, rules, drivingSide, intersectionRadius)
	}
	if len(phases) == 0 {
		phases = []TrafficPhaseConfig{{ID: "open", Name: "Open junction", DurationSeconds: 15}}
	}

	incomingByID, outgoingByID := make(map[string]*Way, len(incoming)), make(map[string]*Way, len(outgoing))
	for _, way := range incoming {
		incomingByID[way.ID] = way
	}
	for _, way := range outgoing {
		outgoingByID[way.ID] = way
	}
	coveredIncoming := make(map[string]bool, len(incoming))
	phaseIDs := make(map[string]bool, len(phases))
	control := TrafficControl{OffsetSeconds: input.OffsetSeconds, Generated: generated, Adaptive: generated, PlanningBasis: generatedPlanBasis, TimingMethod: "Webster green bounds + differential-queue max-pressure scheduling"}
	if !generated {
		control.PlanningBasis = "explicit movement plan from city configuration"
		control.TimingMethod = "configured fixed-time cycle"
		control.MaxRedSeconds = 0
	}

	for index := range phases {
		phase := phases[index]
		if phase.ID == "" {
			phase.ID = fmt.Sprintf("phase-%d", index+1)
		}
		if phaseIDs[phase.ID] {
			return TrafficControl{}, fmt.Errorf("intersection %q has duplicate phase %q", intersection.ID, phase.ID)
		}
		phaseIDs[phase.ID] = true
		if phase.Name == "" {
			phase.Name = phase.ID
		}
		if phase.DurationSeconds <= 0 {
			return TrafficControl{}, fmt.Errorf("intersection %q phase %q needs a positive duration", intersection.ID, phase.ID)
		}
		if phase.AmberSeconds < 0 || phase.AllRedSeconds < 0 {
			return TrafficControl{}, fmt.Errorf("intersection %q phase %q has a negative clearance", intersection.ID, phase.ID)
		}
		if phase.MinGreenSeconds <= 0 {
			phase.MinGreenSeconds = phase.DurationSeconds
		}
		if phase.MaxGreenSeconds <= 0 {
			phase.MaxGreenSeconds = phase.DurationSeconds
		}
		if phase.MinGreenSeconds > phase.DurationSeconds || phase.MaxGreenSeconds < phase.DurationSeconds {
			return TrafficControl{}, fmt.Errorf("intersection %q phase %q green %.1fs is outside its %.1f–%.1fs bounds", intersection.ID, phase.ID, phase.DurationSeconds, phase.MinGreenSeconds, phase.MaxGreenSeconds)
		}
		if len(phase.AllowedMovements) == 0 && len(phase.AllowedIncoming) > 0 {
			for _, fromID := range phase.AllowedIncoming {
				from := incomingByID[fromID]
				if from == nil {
					return TrafficControl{}, fmt.Errorf("intersection %q phase %q allows non-incoming way %q", intersection.ID, phase.ID, fromID)
				}
				phase.AllowedMovements = append(phase.AllowedMovements, makeApproachMovements(from, outgoing, rules, drivingSide)...)
			}
		}
		phase.AllowedIncoming = nil
		phase.DemandScore = 0
		incomingSeen := make(map[string]bool)
		for movementIndex := range phase.AllowedMovements {
			movement := &phase.AllowedMovements[movementIndex]
			from, to := incomingByID[movement.FromWayID], outgoingByID[movement.ToWayID]
			if from == nil || to == nil {
				return TrafficControl{}, fmt.Errorf("intersection %q phase %q has movement %s -> %s outside the junction", intersection.ID, phase.ID, movement.FromWayID, movement.ToWayID)
			}
			expectedTurn := classifyMovement(from, to, rules)
			if movement.Turn == "" {
				movement.Turn = expectedTurn
			} else if movement.Turn != expectedTurn {
				return TrafficControl{}, fmt.Errorf("intersection %q phase %q movement %s -> %s is labeled %q but geometry is %q", intersection.ID, phase.ID, movement.FromWayID, movement.ToWayID, movement.Turn, expectedTurn)
			}
			if len(movement.LaneConnections) == 0 {
				if len(movement.FromLanes) == 0 {
					movement.FromLanes = lanesForTurn(from.Lanes, movement.Turn, drivingSide)
				}
				if len(movement.ToLanes) == 0 {
					movement.ToLanes = lanesForMovementExit(to.Lanes, movement.Turn, drivingSide)
				}
				movement.LaneConnections = connectLaneSets(movement.FromLanes, movement.ToLanes)
			} else {
				if len(movement.FromLanes) == 0 {
					movement.FromLanes = connectionLaneUnion(movement.LaneConnections, true)
				}
				if len(movement.ToLanes) == 0 {
					movement.ToLanes = connectionLaneUnion(movement.LaneConnections, false)
				}
			}
			if err := validateLaneIndices(movement.FromLanes, from.Lanes); err != nil {
				return TrafficControl{}, fmt.Errorf("intersection %q phase %q movement %s: %w", intersection.ID, phase.ID, movement.FromWayID, err)
			}
			if err := validateLaneIndices(movement.ToLanes, to.Lanes); err != nil {
				return TrafficControl{}, fmt.Errorf("intersection %q phase %q movement %s: %w", intersection.ID, phase.ID, movement.ToWayID, err)
			}
			if err := validateLaneConnections(*movement); err != nil {
				return TrafficControl{}, fmt.Errorf("intersection %q phase %q movement %s -> %s: %w", intersection.ID, phase.ID, movement.FromWayID, movement.ToWayID, err)
			}
			if movement.DemandScore <= 0 {
				movement.DemandScore = movementDemand(from, movement.Turn, len(movement.FromLanes))
			}
			phase.DemandScore += movement.DemandScore
			incomingSeen[from.ID], coveredIncoming[from.ID] = true, true
		}
		for wayID := range incomingSeen {
			phase.AllowedIncoming = append(phase.AllowedIncoming, wayID)
		}
		sort.Strings(phase.AllowedIncoming)
		if err := validateCompatiblePhase(intersection.ID, phase, incomingByID, outgoingByID, intersectionRadius); err != nil {
			return TrafficControl{}, err
		}
		control.CycleSeconds += phase.DurationSeconds + phase.AmberSeconds + phase.AllRedSeconds
		control.LostTime += phase.AmberSeconds + phase.AllRedSeconds
		control.VolumeRatio += phase.FlowRatio
		control.Phases = append(control.Phases, phase)
	}
	for _, way := range incoming {
		if len(outgoing) > 0 && !coveredIncoming[way.ID] {
			return TrafficControl{}, fmt.Errorf("intersection %q traffic plan permanently blocks incoming way %q", intersection.ID, way.ID)
		}
	}
	if generated {
		control.MaxRedSeconds = control.CycleSeconds
	}
	return control, nil
}

func generateTrafficPhases(intersection *Intersection, incoming, outgoing []*Way, rules MovementRulesConfig, drivingSide string, intersectionRadius float64) []TrafficPhaseConfig {
	if len(incoming) == 0 {
		return nil
	}
	incoming = append([]*Way(nil), incoming...)
	outgoing = append([]*Way(nil), outgoing...)
	sort.Slice(incoming, func(i, j int) bool { return incoming[i].ID < incoming[j].ID })
	sort.Slice(outgoing, func(i, j int) bool { return outgoing[i].ID < outgoing[j].ID })
	incomingByID, outgoingByID := make(map[string]*Way, len(incoming)), make(map[string]*Way, len(outgoing))
	for _, way := range incoming {
		incomingByID[way.ID] = way
	}
	for _, way := range outgoing {
		outgoingByID[way.ID] = way
	}

	// Treat every lane-to-lane connector as a separately controllable movement.
	// This lets one through lane continue while another approach safely enters
	// an otherwise unused receiving lane.
	var movements []TrafficMovementConfig
	for _, from := range incoming {
		for _, movement := range makeApproachMovements(from, outgoing, rules, drivingSide) {
			movements = append(movements, splitMovementConnectors(movement)...)
		}
	}
	if len(movements) == 0 {
		return nil
	}

	conflicts := movementConflictMatrix(movements, incomingByID, outgoingByID, intersectionRadius)
	phaseSets := selectCompatiblePhaseSets(movements, conflicts)
	drafts := make([]phaseDraft, 0, len(phaseSets))
	seenPhases := make(map[string]bool, len(phaseSets))
	for _, selected := range phaseSets {
		key := phaseIndexKey(selected)
		if seenPhases[key] {
			continue
		}
		seenPhases[key] = true
		phaseMovements := make([]TrafficMovementConfig, len(selected))
		for index, movementIndex := range selected {
			phaseMovements[index] = movements[movementIndex]
		}
		ways, onlyProtectedTurns := phaseApproaches(phaseMovements, incomingByID)
		phase := TrafficPhaseConfig{
			ID: fmt.Sprintf("compatible-%d", len(drafts)+1), Name: compatiblePhaseName(phaseMovements, ways),
			MinGreenSeconds: minimumGreen(ways, onlyProtectedTurns), AmberSeconds: 3, AllRedSeconds: 1.5,
			AllowedMovements: phaseMovements,
		}
		drafts = append(drafts, phaseDraft{phase: phase})
	}
	if len(drafts) == 0 {
		return nil
	}
	allocateGreenTime(drafts)
	phases := make([]TrafficPhaseConfig, len(drafts))
	for index := range drafts {
		phases[index] = drafts[index].phase
	}
	return phases
}

const maximalPhaseSearchLimit = 256

// selectCompatiblePhaseSets considers complete maximal movement sets, then
// chooses a compact cover weighted by currently configured movement demand.
// Considering the whole set avoids the common greedy failure where pairing a
// left turn with its own approach prevents a much larger opposing-through pair.
func selectCompatiblePhaseSets(movements []TrafficMovementConfig, conflicts [][]bool) [][]int {
	candidates, complete := maximalCompatibleSets(conflicts, maximalPhaseSearchLimit)
	if !complete || len(candidates) == 0 {
		return coloredCompatiblePhaseSets(movements, conflicts)
	}
	requiredPairs := compatibleLaneTransferPairs(movements, conflicts)
	if len(candidates) <= 16 {
		if exact := exactCompactPhaseCover(candidates, movements, requiredPairs); len(exact) > 0 {
			return exact
		}
	}
	uncoveredMovements := make(map[int]bool, len(movements))
	for index := range movements {
		uncoveredMovements[index] = true
	}
	uncoveredPairs := make(map[[2]int]bool, len(requiredPairs))
	for pair := range requiredPairs {
		uncoveredPairs[pair] = true
	}
	selected := make([][]int, 0)
	selectedKeys := make(map[string]bool)
	for len(uncoveredMovements) > 0 {
		best := -1
		bestUncovered, bestCapacity := -1.0, -1.0
		for index, candidate := range candidates {
			key := phaseIndexKey(candidate)
			if selectedKeys[key] {
				continue
			}
			uncoveredDemand, capacity := 0.0, 0.0
			for _, movementIndex := range candidate {
				weight := math.Max(.001, movements[movementIndex].DemandScore)
				capacity += weight
				if uncoveredMovements[movementIndex] {
					uncoveredDemand += weight
				}
			}
			for first := 0; first < len(candidate); first++ {
				for second := first + 1; second < len(candidate); second++ {
					pair := orderedPair(candidate[first], candidate[second])
					if !uncoveredPairs[pair] {
						continue
					}
					firstWeight := math.Max(.001, movements[pair[0]].DemandScore)
					secondWeight := math.Max(.001, movements[pair[1]].DemandScore)
					uncoveredDemand += math.Sqrt(firstWeight * secondWeight)
				}
			}
			if best < 0 || uncoveredDemand > bestUncovered+1e-9 ||
				(math.Abs(uncoveredDemand-bestUncovered) <= 1e-9 && capacity > bestCapacity+1e-9) ||
				(math.Abs(uncoveredDemand-bestUncovered) <= 1e-9 && math.Abs(capacity-bestCapacity) <= 1e-9 && phaseIndexKey(candidate) < phaseIndexKey(candidates[best])) {
				best, bestUncovered, bestCapacity = index, uncoveredDemand, capacity
			}
		}
		if best < 0 || bestUncovered <= 0 {
			return coloredCompatiblePhaseSets(movements, conflicts)
		}
		phase := append([]int(nil), candidates[best]...)
		selected = append(selected, phase)
		selectedKeys[phaseIndexKey(phase)] = true
		for _, movementIndex := range phase {
			delete(uncoveredMovements, movementIndex)
		}
		for first := 0; first < len(phase); first++ {
			for second := first + 1; second < len(phase); second++ {
				delete(uncoveredPairs, orderedPair(phase[first], phase[second]))
			}
		}
	}
	// Remove a phase made redundant by later, more complementary choices.
	for index := len(selected) - 1; index >= 0; index-- {
		without := make([][]int, 0, len(selected)-1)
		without = append(without, selected[:index]...)
		without = append(without, selected[index+1:]...)
		if phaseSetsCover(without, len(movements), nil) {
			selected = without
		}
	}
	// The demand-weighted maximal-set cover can choose complementary cliques
	// that expose useful lane transfers but require substantially more signal
	// stages than a complete coloring. Prefer the colored maximal cover whenever
	// it serves every connector with fewer stages; each avoided stage removes an
	// amber/all-red interval and materially increases junction capacity.
	compact := coloredCompatiblePhaseSets(movements, conflicts)
	if len(compact) > 0 && len(compact) < len(selected) {
		return compact
	}
	return selected
}

// exactCompactPhaseCover is used for the small feasible-set families typical
// of three-way junctions. It retains preferred safe lane-separated transfers,
// then minimizes phase count and favors higher-capacity covers. Larger families
// use the bounded greedy cover below to keep graph loading predictable.
func exactCompactPhaseCover(candidates [][]int, movements []TrafficMovementConfig, preferredPairs map[[2]int]bool) [][]int {
	minimumCount := len(candidates) + 1
	var findMinimum func(int, [][]int)
	findMinimum = func(index int, selected [][]int) {
		if len(selected) >= minimumCount {
			return
		}
		if index == len(candidates) {
			if phaseSetsCover(selected, len(movements), nil) {
				minimumCount = len(selected)
			}
			return
		}
		findMinimum(index+1, append(selected, candidates[index]))
		findMinimum(index+1, selected)
	}
	findMinimum(0, nil)
	if minimumCount > len(candidates) {
		return nil
	}

	// Small candidate families can retain every preferred lane-separated
	// transfer, then minimize the phase count among covers with that capability.
	phaseLimit := len(candidates)
	bestCount := phaseLimit + 1
	bestPairScore, bestCapacity := -1.0, -1.0
	bestKey := ""
	var best [][]int
	var search func(int, [][]int)
	search = func(index int, selected [][]int) {
		if len(selected) > phaseLimit {
			return
		}
		if index == len(candidates) {
			if !phaseSetsCover(selected, len(movements), nil) {
				return
			}
			pairScore, capacity := phaseSetPreference(selected, movements, preferredPairs)
			key := phaseSetKey(selected)
			if pairScore > bestPairScore+1e-9 ||
				(math.Abs(pairScore-bestPairScore) <= 1e-9 && len(selected) < bestCount) ||
				(math.Abs(pairScore-bestPairScore) <= 1e-9 && len(selected) == bestCount && capacity > bestCapacity+1e-9) ||
				(math.Abs(pairScore-bestPairScore) <= 1e-9 && len(selected) == bestCount && math.Abs(capacity-bestCapacity) <= 1e-9 && (bestKey == "" || key < bestKey)) {
				bestCount, bestPairScore, bestCapacity, bestKey = len(selected), pairScore, capacity, key
				best = clonePhaseSets(selected)
			}
			return
		}
		search(index+1, append(selected, candidates[index]))
		search(index+1, selected)
	}
	search(0, nil)
	return best
}

func phaseSetPreference(phases [][]int, movements []TrafficMovementConfig, preferredPairs map[[2]int]bool) (pairScore, capacity float64) {
	coveredPairs := make(map[[2]int]bool, len(preferredPairs))
	for _, phase := range phases {
		for _, movement := range phase {
			capacity += math.Max(.001, movements[movement].DemandScore)
		}
		for first := 0; first < len(phase); first++ {
			for second := first + 1; second < len(phase); second++ {
				pair := orderedPair(phase[first], phase[second])
				if preferredPairs[pair] {
					coveredPairs[pair] = true
				}
			}
		}
	}
	for pair := range coveredPairs {
		firstWeight := math.Max(.001, movements[pair[0]].DemandScore)
		secondWeight := math.Max(.001, movements[pair[1]].DemandScore)
		pairScore += math.Sqrt(firstWeight * secondWeight)
	}
	return pairScore, capacity
}

func phaseSetKey(phases [][]int) string {
	key := ""
	for _, phase := range phases {
		key += phaseIndexKey(phase) + ";"
	}
	return key
}

func clonePhaseSets(phases [][]int) [][]int {
	clone := make([][]int, len(phases))
	for index, phase := range phases {
		clone[index] = append([]int(nil), phase...)
	}
	return clone
}

// compatibleLaneTransferPairs are opportunities a road-level movement model
// normally misses: a right turn and through flow entering distinct lanes of the
// same exit. They receive a selection bonus so compact phase covers retain this
// useful merge without expanding into every permissive combination.
func compatibleLaneTransferPairs(movements []TrafficMovementConfig, conflicts [][]bool) map[[2]int]bool {
	result := make(map[[2]int]bool)
	for first := range movements {
		for second := first + 1; second < len(movements); second++ {
			if movements[first].FromWayID != movements[second].FromWayID &&
				movements[first].ToWayID == movements[second].ToWayID &&
				((movements[first].Turn == "through" && movements[second].Turn == "right") ||
					(movements[first].Turn == "right" && movements[second].Turn == "through")) &&
				preferredLaneTransferConnector(movements, first) && preferredLaneTransferConnector(movements, second) &&
				!conflicts[first][second] {
				result[[2]int{first, second}] = true
			}
		}
	}
	return result
}

func preferredLaneTransferConnector(movements []TrafficMovementConfig, index int) bool {
	movement := movements[index]
	if len(movement.ToLanes) != 1 {
		return false
	}
	switch movement.Turn {
	case "through":
		return true
	case "left":
		return movement.ToLanes[0] == 0
	case "right":
		maximum := movement.ToLanes[0]
		for _, candidate := range movements {
			if candidate.FromWayID == movement.FromWayID && candidate.ToWayID == movement.ToWayID && len(candidate.ToLanes) == 1 {
				maximum = max(maximum, candidate.ToLanes[0])
			}
		}
		return movement.ToLanes[0] == maximum
	default:
		return false
	}
}

func orderedPair(first, second int) [2]int {
	if first > second {
		first, second = second, first
	}
	return [2]int{first, second}
}

func phaseSetsCover(phases [][]int, movementCount int, requiredPairs map[[2]int]bool) bool {
	movements := make([]bool, movementCount)
	pairs := make(map[[2]int]bool, len(requiredPairs))
	for _, phase := range phases {
		for _, movement := range phase {
			movements[movement] = true
		}
		for first := 0; first < len(phase); first++ {
			for second := first + 1; second < len(phase); second++ {
				pair := orderedPair(phase[first], phase[second])
				if requiredPairs[pair] {
					pairs[pair] = true
				}
			}
		}
	}
	for _, covered := range movements {
		if !covered {
			return false
		}
	}
	return len(pairs) == len(requiredPairs)
}

// maximalCompatibleSets enumerates maximal cliques in the complement of the
// conflict graph with a Bron-Kerbosch search. Four-way and three-way junctions
// are small enough to search exactly; the limit provides a deterministic guard
// for unusual high-degree custom junctions.
func maximalCompatibleSets(conflicts [][]bool, limit int) ([][]int, bool) {
	potential := make([]int, len(conflicts))
	for index := range potential {
		potential[index] = index
	}
	var result [][]int
	complete := true
	var search func([]int, []int, []int)
	search = func(current, candidates, excluded []int) {
		if !complete {
			return
		}
		if len(candidates) == 0 && len(excluded) == 0 {
			phase := append([]int(nil), current...)
			sort.Ints(phase)
			result = append(result, phase)
			if len(result) > limit {
				complete = false
			}
			return
		}
		pivot := cliquePivot(candidates, excluded, conflicts)
		branch := append([]int(nil), candidates...)
		for _, vertex := range branch {
			if pivot >= 0 && vertex != pivot && !conflicts[pivot][vertex] {
				continue
			}
			nextCurrent := append(append([]int(nil), current...), vertex)
			nextCandidates := compatibleIntersection(candidates, vertex, conflicts)
			nextExcluded := compatibleIntersection(excluded, vertex, conflicts)
			search(nextCurrent, nextCandidates, nextExcluded)
			candidates = removeIndex(candidates, vertex)
			excluded = append(excluded, vertex)
			if !complete {
				return
			}
		}
	}
	search(nil, potential, nil)
	sort.Slice(result, func(i, j int) bool { return phaseIndexKey(result[i]) < phaseIndexKey(result[j]) })
	return result, complete
}

func cliquePivot(candidates, excluded []int, conflicts [][]bool) int {
	pivot, best := -1, -1
	for _, candidate := range append(append([]int(nil), candidates...), excluded...) {
		compatible := 0
		for _, vertex := range candidates {
			if candidate != vertex && !conflicts[candidate][vertex] {
				compatible++
			}
		}
		if compatible > best {
			pivot, best = candidate, compatible
		}
	}
	return pivot
}

func compatibleIntersection(indices []int, vertex int, conflicts [][]bool) []int {
	result := make([]int, 0, len(indices))
	for _, candidate := range indices {
		if candidate != vertex && !conflicts[vertex][candidate] {
			result = append(result, candidate)
		}
	}
	return result
}

func removeIndex(indices []int, target int) []int {
	result := make([]int, 0, len(indices)-1)
	for _, index := range indices {
		if index != target {
			result = append(result, index)
		}
	}
	return result
}

func coloredCompatiblePhaseSets(movements []TrafficMovementConfig, conflicts [][]bool) [][]int {
	colors, colorCount := colorMovementConflicts(movements, conflicts)
	result := make([][]int, 0, colorCount)
	seen := make(map[string]bool)
	for color := 0; color < colorCount; color++ {
		selected := make([]int, 0, len(movements))
		selectedSet := make(map[int]bool)
		for index, assigned := range colors {
			if assigned == color {
				selected = append(selected, index)
				selectedSet[index] = true
			}
		}
		for _, candidate := range phaseExpansionOrder(movements, conflicts, selectedSet) {
			if compatibleWithSelection(candidate, selected, conflicts) {
				selected = append(selected, candidate)
			}
		}
		sort.Ints(selected)
		key := phaseIndexKey(selected)
		if !seen[key] {
			seen[key] = true
			result = append(result, selected)
		}
	}
	return result
}

func splitMovementConnectors(movement TrafficMovementConfig) []TrafficMovementConfig {
	result := make([]TrafficMovementConfig, 0, len(movement.LaneConnections))
	for _, connection := range movement.LaneConnections {
		part := movement
		part.FromLanes = []int{connection.FromLane}
		part.ToLanes = []int{connection.ToLane}
		part.LaneConnections = []LaneConnectionConfig{connection}
		part.DemandScore = movement.DemandScore / float64(len(movement.LaneConnections))
		result = append(result, part)
	}
	return result
}

func movementConflictMatrix(movements []TrafficMovementConfig, incoming, outgoing map[string]*Way, radius float64) [][]bool {
	conflicts := make([][]bool, len(movements))
	for index := range conflicts {
		conflicts[index] = make([]bool, len(movements))
	}
	for first := range movements {
		for second := first + 1; second < len(movements); second++ {
			conflict := movementsConflict(movements[first], movements[second], incoming, outgoing, radius)
			conflicts[first][second], conflicts[second][first] = conflict, conflict
		}
	}
	return conflicts
}

// colorMovementConflicts uses deterministic DSATUR coloring. Each color is a
// feasible phase and every movement is covered; the subsequent expansion may
// repeat a movement in any other phase where it is also safe.
func colorMovementConflicts(movements []TrafficMovementConfig, conflicts [][]bool) ([]int, int) {
	colors := make([]int, len(movements))
	for index := range colors {
		colors[index] = -1
	}
	colorCount := 0
	for colored := 0; colored < len(movements); colored++ {
		selected := -1
		selectedSaturation, selectedDegree := -1, -1
		for candidate := range movements {
			if colors[candidate] >= 0 {
				continue
			}
			neighborColors := make(map[int]bool)
			degree := 0
			for neighbor, conflict := range conflicts[candidate] {
				if !conflict {
					continue
				}
				degree++
				if colors[neighbor] >= 0 {
					neighborColors[colors[neighbor]] = true
				}
			}
			saturation := len(neighborColors)
			if selected < 0 || saturation > selectedSaturation ||
				(saturation == selectedSaturation && degree > selectedDegree) ||
				(saturation == selectedSaturation && degree == selectedDegree && movementLess(movements[candidate], movements[selected])) {
				selected, selectedSaturation, selectedDegree = candidate, saturation, degree
			}
		}
		used := make(map[int]bool)
		for neighbor, conflict := range conflicts[selected] {
			if conflict && colors[neighbor] >= 0 {
				used[colors[neighbor]] = true
			}
		}
		color := 0
		for used[color] {
			color++
		}
		colors[selected] = color
		colorCount = max(colorCount, color+1)
	}
	return colors, colorCount
}

func phaseExpansionOrder(movements []TrafficMovementConfig, conflicts [][]bool, selected map[int]bool) []int {
	result := make([]int, 0, len(movements)-len(selected))
	for index := range movements {
		if !selected[index] {
			result = append(result, index)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		first, second := result[i], result[j]
		firstDegree, secondDegree := conflictDegree(conflicts[first]), conflictDegree(conflicts[second])
		if firstDegree != secondDegree {
			return firstDegree < secondDegree
		}
		if movements[first].DemandScore != movements[second].DemandScore {
			return movements[first].DemandScore > movements[second].DemandScore
		}
		return movementLess(movements[first], movements[second])
	})
	return result
}

func conflictDegree(conflicts []bool) int {
	degree := 0
	for _, conflict := range conflicts {
		if conflict {
			degree++
		}
	}
	return degree
}

func compatibleWithSelection(candidate int, selected []int, conflicts [][]bool) bool {
	for _, existing := range selected {
		if conflicts[candidate][existing] {
			return false
		}
	}
	return true
}

func movementLess(a, b TrafficMovementConfig) bool {
	if a.FromWayID != b.FromWayID {
		return a.FromWayID < b.FromWayID
	}
	if a.ToWayID != b.ToWayID {
		return a.ToWayID < b.ToWayID
	}
	aLane, bLane := -1, -1
	if len(a.FromLanes) > 0 {
		aLane = a.FromLanes[0]
	}
	if len(b.FromLanes) > 0 {
		bLane = b.FromLanes[0]
	}
	if aLane != bLane {
		return aLane < bLane
	}
	aTarget, bTarget := -1, -1
	if len(a.ToLanes) > 0 {
		aTarget = a.ToLanes[0]
	}
	if len(b.ToLanes) > 0 {
		bTarget = b.ToLanes[0]
	}
	return aTarget < bTarget
}

func phaseIndexKey(indices []int) string {
	return fmt.Sprint(indices)
}

func phaseApproaches(movements []TrafficMovementConfig, incoming map[string]*Way) ([]*Way, bool) {
	seen := make(map[string]bool)
	ways := make([]*Way, 0)
	onlyProtectedTurns := len(movements) > 0
	for _, movement := range movements {
		if !seen[movement.FromWayID] {
			seen[movement.FromWayID] = true
			ways = append(ways, incoming[movement.FromWayID])
		}
		if movement.Turn != "left" && movement.Turn != "u_turn" {
			onlyProtectedTurns = false
		}
	}
	sort.Slice(ways, func(i, j int) bool { return ways[i].ID < ways[j].ID })
	return ways, onlyProtectedTurns
}

func compatiblePhaseName(movements []TrafficMovementConfig, ways []*Way) string {
	kind := "compatible movements"
	onlyLeft, onlyMain := len(movements) > 0, len(movements) > 0
	for _, movement := range movements {
		onlyLeft = onlyLeft && (movement.Turn == "left" || movement.Turn == "u_turn")
		onlyMain = onlyMain && movement.Turn != "left" && movement.Turn != "u_turn"
	}
	if onlyLeft {
		kind = "protected left"
	} else if onlyMain {
		kind = "through + right"
	}
	return classLabel(dominantClass(ways)) + " " + kind
}

func classLabel(class string) string {
	if class == "" {
		return "Local"
	}
	return string(class[0]-32) + class[1:]
}

func classifyMovement(from, to *Way, rules MovementRulesConfig) string {
	angle := movementAngle(from, to)
	throughTolerance := rules.ThroughToleranceDegrees * math.Pi / 180
	uTurnTolerance := rules.UTurnToleranceDegrees * math.Pi / 180
	if math.Pi-math.Abs(angle) <= uTurnTolerance {
		return "u_turn"
	}
	if math.Abs(angle) <= throughTolerance {
		return "through"
	}
	// World Y grows downward, so negative signed angles turn visually left.
	if angle < 0 {
		return "left"
	}
	return "right"
}

func movementAngle(from, to *Way) float64 {
	_, incoming := from.Sample(math.Max(0, from.Length-1), 0)
	_, outgoing := to.Sample(math.Min(1, to.Length), 0)
	dot := incoming.X*outgoing.X + incoming.Y*outgoing.Y
	cross := incoming.X*outgoing.Y - incoming.Y*outgoing.X
	return math.Atan2(cross, dot)
}

type approachMovementTarget struct {
	way      *Way
	turn     string
	angle    float64
	distance int
}

// makeApproachMovements assigns the complete set of movements for one
// approach at once. Lane zero is the driver's leftmost lane. Target directions
// are therefore ordered by signed angle from visual left to right, which is
// clockwise in the simulator's screen-oriented coordinate system.
func makeApproachMovements(from *Way, outgoing []*Way, rules MovementRulesConfig, drivingSide string) []TrafficMovementConfig {
	targets := make([]approachMovementTarget, 0, len(outgoing))
	for _, to := range outgoing {
		turn := classifyMovement(from, to, rules)
		if turn == "u_turn" && len(outgoing) > 1 {
			continue
		}
		targets = append(targets, approachMovementTarget{
			way: to, turn: turn, angle: movementAngle(from, to),
		})
	}
	sort.Slice(targets, func(i, j int) bool {
		if math.Abs(targets[i].angle-targets[j].angle) > 1e-9 {
			return targets[i].angle < targets[j].angle
		}
		return targets[i].way.ID < targets[j].way.ID
	})
	// A dead-end reversal is not one of the N-1 destination directions covered
	// by the allocation rule. Keep the conventional median-side U-turn lane.
	if len(targets) == 1 && targets[0].turn == "u_turn" {
		return []TrafficMovementConfig{makeMovement(from, targets[0].way, targets[0].turn, drivingSide)}
	}

	// The current approach occupies the omitted position at the circular-order
	// boundary. Counting steps in either direction gives the requested
	// topological distance without letting irregular geometric angles bias it.
	directionCount := len(targets) + 1
	for index := range targets {
		clockwiseSteps := index + 1
		targets[index].distance = min(clockwiseSteps, directionCount-clockwiseSteps)
	}
	lanesByTarget := assignApproachTargetLanes(targets, from.Lanes)
	movements := make([]TrafficMovementConfig, 0, len(targets))
	for index, target := range targets {
		fromLanes := lanesByTarget[index]
		toLanes := lanesForMovementExit(target.way.Lanes, target.turn, drivingSide)
		movements = append(movements, TrafficMovementConfig{
			FromWayID: from.ID, ToWayID: target.way.ID, FromLanes: fromLanes, ToLanes: toLanes,
			LaneConnections: connectLaneSets(fromLanes, toLanes), Turn: target.turn,
			DemandScore: movementDemand(from, target.turn, len(fromLanes)),
		})
	}
	return movements
}

// assignApproachTargetLanes implements the three allocation regimes:
// dedicated lanes when supply is sufficient, far-direction-first reinforcement
// for surplus lanes, and balanced contiguous direction groups when lanes are
// scarce. Contiguous groups preserve monotone left-to-right movement order.
func assignApproachTargetLanes(targets []approachMovementTarget, laneCount int) [][]int {
	assignments := make([][]int, len(targets))
	if len(targets) == 0 || laneCount <= 0 {
		return assignments
	}
	if laneCount >= len(targets) {
		lanesPerTarget := laneCount / len(targets)
		extra := laneCount % len(targets)
		quotas := make([]int, len(targets))
		for index := range quotas {
			quotas[index] = lanesPerTarget
		}
		priority := make([]int, len(targets))
		for index := range priority {
			priority[index] = index
		}
		sort.SliceStable(priority, func(i, j int) bool {
			return targets[priority[i]].distance > targets[priority[j]].distance
		})
		for index := 0; index < extra; index++ {
			quotas[priority[index]]++
		}
		lane := 0
		for target, quota := range quotas {
			for count := 0; count < quota; count++ {
				assignments[target] = append(assignments[target], lane)
				lane++
			}
		}
		return assignments
	}

	groupSizes := balancedDirectionGroupSizes(targets, laneCount)
	target := 0
	for lane, groupSize := range groupSizes {
		for count := 0; count < groupSize; count++ {
			assignments[target] = append(assignments[target], lane)
			target++
		}
	}
	return assignments
}

// balancedDirectionGroupSizes partitions the clockwise target order into
// contiguous groups whose sizes differ by at most one. When more than one such
// partition exists, its cost places the larger (merged) groups on directions
// with the lowest circular distance, leaving farther directions as independent
// as the monotone lane order permits.
func balancedDirectionGroupSizes(targets []approachMovementTarget, laneCount int) []int {
	small := len(targets) / laneCount
	largeGroups := len(targets) % laneCount
	type state struct {
		group     int
		largeUsed int
	}
	type result struct {
		cost  int
		sizes []int
		ok    bool
	}
	memo := make(map[state]result)
	var solve func(group, largeUsed int) result
	solve = func(group, largeUsed int) result {
		key := state{group: group, largeUsed: largeUsed}
		if cached, ok := memo[key]; ok {
			return cached
		}
		if group == laneCount {
			answer := result{ok: largeUsed == largeGroups}
			memo[key] = answer
			return answer
		}
		best := result{}
		for _, size := range []int{small, small + 1} {
			usesLarge := 0
			if size == small+1 {
				usesLarge = 1
			}
			nextLarge := largeUsed + usesLarge
			if nextLarge > largeGroups || laneCount-group-1 < largeGroups-nextLarge {
				continue
			}
			start := group*small + largeUsed
			end := start + size
			if end > len(targets) {
				continue
			}
			tail := solve(group+1, nextLarge)
			if !tail.ok {
				continue
			}
			cost := tail.cost
			for index := start; index < end; index++ {
				cost += (size - 1) * targets[index].distance
			}
			if !best.ok || cost < best.cost {
				best = result{cost: cost, sizes: append([]int{size}, tail.sizes...), ok: true}
			}
		}
		memo[key] = best
		return best
	}
	return solve(0, 0).sizes
}

func makeMovement(from, to *Way, turn, drivingSide string) TrafficMovementConfig {
	fromLanes := lanesForTurn(from.Lanes, turn, drivingSide)
	toLanes := lanesForMovementExit(to.Lanes, turn, drivingSide)
	return TrafficMovementConfig{
		FromWayID: from.ID, ToWayID: to.ID, FromLanes: fromLanes, ToLanes: toLanes,
		LaneConnections: connectLaneSets(fromLanes, toLanes), Turn: turn,
		DemandScore: movementDemand(from, turn, len(fromLanes)),
	}
}

func lanesForTurn(lanes int, turn, drivingSide string) []int {
	if lanes <= 1 {
		return []int{0}
	}
	switch turn {
	case "left":
		return []int{0}
	case "right":
		return []int{lanes - 1}
	case "u_turn":
		if drivingSide == DrivingSideLeft {
			return []int{lanes - 1}
		}
		return []int{0}
	default:
		return allLanes(lanes)
	}
}

func lanesForMovementExit(lanes int, turn, drivingSide string) []int {
	// Preserve every legal downstream option in the route topology. Phase
	// generation splits these into individual connectors and recombines only
	// the geometrically compatible ones; the active signal then exposes exactly
	// that subset to a turning vehicle.
	return allLanes(lanes)
}

// connectLaneSets partitions the receiving lanes across the source lanes in
// monotone order. This exposes every downstream lane without creating crossing
// connectors. A single turn lane may fan into every receiving lane; when lane
// counts differ, adjacent source lanes split or merge their contiguous ranges.
func connectLaneSets(fromLanes, toLanes []int) []LaneConnectionConfig {
	from := append([]int(nil), fromLanes...)
	to := append([]int(nil), toLanes...)
	sort.Ints(from)
	sort.Ints(to)
	if len(from) == 0 || len(to) == 0 {
		return nil
	}
	connections := make([]LaneConnectionConfig, 0, max(len(from), len(to)))
	for fromIndex, fromLane := range from {
		start := int(math.Round(float64(fromIndex) * float64(len(to)) / float64(len(from))))
		end := int(math.Round(float64(fromIndex+1) * float64(len(to)) / float64(len(from))))
		if start == end {
			target := 0
			if len(from) > 1 && len(to) > 1 {
				target = int(math.Round(float64(fromIndex) * float64(len(to)-1) / float64(len(from)-1)))
			}
			connections = append(connections, LaneConnectionConfig{FromLane: fromLane, ToLane: to[target]})
			continue
		}
		for target := start; target < end; target++ {
			connections = append(connections, LaneConnectionConfig{FromLane: fromLane, ToLane: to[target]})
		}
	}
	return connections
}

func connectionLaneUnion(connections []LaneConnectionConfig, from bool) []int {
	seen := make(map[int]bool, len(connections))
	for _, connection := range connections {
		lane := connection.ToLane
		if from {
			lane = connection.FromLane
		}
		seen[lane] = true
	}
	lanes := make([]int, 0, len(seen))
	for lane := range seen {
		lanes = append(lanes, lane)
	}
	sort.Ints(lanes)
	return lanes
}

func validateLaneConnections(movement TrafficMovementConfig) error {
	if len(movement.LaneConnections) == 0 {
		return fmt.Errorf("has no lane connections")
	}
	fromSeen, toSeen := make(map[int]bool), make(map[int]bool)
	pairs := make(map[[2]int]bool, len(movement.LaneConnections))
	for _, connection := range movement.LaneConnections {
		if !containsLane(movement.FromLanes, connection.FromLane) {
			return fmt.Errorf("connection source lane %d is not declared in from_lanes", connection.FromLane)
		}
		if !containsLane(movement.ToLanes, connection.ToLane) {
			return fmt.Errorf("connection target lane %d is not declared in to_lanes", connection.ToLane)
		}
		pair := [2]int{connection.FromLane, connection.ToLane}
		if pairs[pair] {
			return fmt.Errorf("connection %d -> %d is duplicated", connection.FromLane, connection.ToLane)
		}
		pairs[pair], fromSeen[connection.FromLane], toSeen[connection.ToLane] = true, true, true
	}
	for first, a := range movement.LaneConnections {
		for _, b := range movement.LaneConnections[first+1:] {
			if (a.FromLane-b.FromLane)*(a.ToLane-b.ToLane) < 0 {
				return fmt.Errorf("connections %d -> %d and %d -> %d cross", a.FromLane, a.ToLane, b.FromLane, b.ToLane)
			}
		}
	}
	for _, lane := range movement.FromLanes {
		if !fromSeen[lane] {
			return fmt.Errorf("source lane %d has no connection", lane)
		}
	}
	for _, lane := range movement.ToLanes {
		if !toSeen[lane] {
			return fmt.Errorf("target lane %d has no connection", lane)
		}
	}
	return nil
}

func allLanes(lanes int) []int {
	result := make([]int, lanes)
	for lane := range result {
		result[lane] = lane
	}
	return result
}

func movementDemand(from *Way, turn string, usableLanes int) float64 {
	_ = usableLanes
	factor := .65
	switch turn {
	case "left":
		factor = .2
	case "right":
		factor = .15
	case "u_turn":
		factor = .03
	}
	return from.Demand * factor
}

func dominantClass(ways []*Way) string {
	order := map[string]int{"local": 0, "ramp": 1, "collector": 2, "arterial": 3}
	result := "local"
	for _, way := range ways {
		if order[way.RoadClass] > order[result] {
			result = way.RoadClass
		}
	}
	return result
}

func minimumGreen(ways []*Way, protectedLeft bool) float64 {
	class := dominantClass(ways)
	if protectedLeft {
		if class == "arterial" {
			return 9
		}
		return 7
	}
	switch class {
	case "arterial":
		return 16
	case "collector":
		return 11
	case "ramp":
		return 9
	default:
		return 7
	}
}

func allocateGreenTime(drafts []phaseDraft) {
	totalRatio, minimumTotal, clearance := 0.0, 0.0, 0.0
	for index := range drafts {
		for _, movement := range drafts[index].phase.AllowedMovements {
			drafts[index].phase.DemandScore += movement.DemandScore
		}
		drafts[index].phase.FlowRatio = phaseFlowRatio(drafts[index].phase)
		totalRatio += drafts[index].phase.FlowRatio
		minimumTotal += drafts[index].phase.MinGreenSeconds
		clearance += drafts[index].phase.AmberSeconds + drafts[index].phase.AllRedSeconds
	}
	designRatio := math.Min(.92, totalRatio)
	websterCycle := (1.5*clearance + 5) / math.Max(.08, 1-designRatio)
	targetCycle := math.Max(55, math.Min(120, websterCycle))
	targetCycle = math.Max(targetCycle, clearance+minimumTotal)
	extraGreen := math.Max(0, targetCycle-clearance-minimumTotal)
	for index := range drafts {
		share := 1 / float64(len(drafts))
		if totalRatio > 0 {
			share = drafts[index].phase.FlowRatio / totalRatio
		}
		phase := &drafts[index].phase
		phase.DurationSeconds = math.Round((phase.MinGreenSeconds+extraGreen*share)*2) / 2
		phase.MaxGreenSeconds = math.Round(math.Min(phase.DurationSeconds+20, math.Max(phase.MinGreenSeconds+4, phase.DurationSeconds*1.55))*2) / 2
	}
}

func phaseFlowRatio(phase TrafficPhaseConfig) float64 {
	lanes := make(map[string]bool)
	for _, movement := range phase.AllowedMovements {
		for _, lane := range movement.FromLanes {
			lanes[fmt.Sprintf("%s:%d", movement.FromWayID, lane)] = true
		}
	}
	// Demand weights are normalized against two saturation-flow units per
	// protected lane. This preserves a dimensionless critical flow ratio.
	return phase.DemandScore / math.Max(2, float64(len(lanes))*2)
}

func validateCompatiblePhase(intersectionID string, phase TrafficPhaseConfig, incoming, outgoing map[string]*Way, radius float64) error {
	for first := 0; first < len(phase.AllowedMovements); first++ {
		for second := first + 1; second < len(phase.AllowedMovements); second++ {
			a, b := phase.AllowedMovements[first], phase.AllowedMovements[second]
			if movementsConflict(a, b, incoming, outgoing, radius) {
				return fmt.Errorf("intersection %q phase %q contains geometrically conflicting movements %s L%v -> %s L%v and %s L%v -> %s L%v",
					intersectionID, phase.ID, a.FromWayID, a.FromLanes, a.ToWayID, a.ToLanes,
					b.FromWayID, b.FromLanes, b.ToWayID, b.ToLanes)
			}
		}
	}
	return nil
}

func movementsConflict(a, b TrafficMovementConfig, incoming, outgoing map[string]*Way, radius float64) bool {
	if a.FromWayID == b.FromWayID && lanesOverlap(a.FromLanes, b.FromLanes) {
		// Route choices from one physical lane are serialized at the stop line.
		// Different source lanes still receive the full geometric check so an
		// explicit weaving connector cannot be declared conflict-free merely
		// because it begins on the same approach.
		return false
	}
	fromA, fromB := incoming[a.FromWayID], incoming[b.FromWayID]
	toA, toB := outgoing[a.ToWayID], outgoing[b.ToWayID]
	if fromA == nil || fromB == nil || toA == nil || toB == nil {
		return true
	}
	if a.ToWayID == b.ToWayID && lanesOverlap(a.ToLanes, b.ToLanes) {
		return true
	}
	for _, connectionA := range a.LaneConnections {
		pathA := connectorPath(fromA, toA, connectionA, radius)
		for _, connectionB := range b.LaneConnections {
			pathB := connectorPath(fromB, toB, connectionB, radius)
			clearance := .42 * math.Min(math.Min(fromA.LaneWidth, toA.LaneWidth), math.Min(fromB.LaneWidth, toB.LaneWidth))
			if polylinesWithin(pathA, pathB, clearance) {
				return true
			}
		}
	}
	return false
}

func lanesOverlap(a, b []int) bool {
	seen := make(map[int]bool, len(a))
	for _, lane := range a {
		seen[lane] = true
	}
	for _, lane := range b {
		if seen[lane] {
			return true
		}
	}
	return false
}

// connectorPath mirrors the tangent-continuous cubic used by the simulation
// when a vehicle traverses an intersection. Conflict decisions therefore use
// the same lane-centre geometry that vehicles actually follow.
func connectorPath(from, to *Way, connection LaneConnectionConfig, radius float64) []Vec2 {
	start, incomingTangent := from.Sample(math.Max(0, from.Length-radius), connection.FromLane)
	end, outgoingTangent := to.Sample(math.Min(radius, to.Length), connection.ToLane)
	handle := math.Min(radius*.9, start.Sub(end).Length()*.45)
	control1 := start.Add(incomingTangent.Mul(handle))
	control2 := end.Sub(outgoingTangent.Mul(handle))
	const segments = 32
	path := make([]Vec2, segments+1)
	for index := range path {
		t := float64(index) / segments
		u := 1 - t
		path[index] = start.Mul(u * u * u).
			Add(control1.Mul(3 * u * u * t)).
			Add(control2.Mul(3 * u * t * t)).
			Add(end.Mul(t * t * t))
	}
	return path
}

func polylinesWithin(a, b []Vec2, clearance float64) bool {
	clearanceSquared := clearance * clearance
	for first := 0; first+1 < len(a); first++ {
		for second := 0; second+1 < len(b); second++ {
			if segmentDistanceSquared(a[first], a[first+1], b[second], b[second+1]) < clearanceSquared {
				return true
			}
		}
	}
	return false
}

func segmentDistanceSquared(a0, a1, b0, b1 Vec2) float64 {
	if segmentsIntersect(a0, a1, b0, b1) {
		return 0
	}
	return math.Min(
		math.Min(pointSegmentDistanceSquared(a0, b0, b1), pointSegmentDistanceSquared(a1, b0, b1)),
		math.Min(pointSegmentDistanceSquared(b0, a0, a1), pointSegmentDistanceSquared(b1, a0, a1)),
	)
}

func pointSegmentDistanceSquared(point, start, end Vec2) float64 {
	line := end.Sub(start)
	lengthSquared := line.X*line.X + line.Y*line.Y
	if lengthSquared == 0 {
		delta := point.Sub(start)
		return delta.X*delta.X + delta.Y*delta.Y
	}
	t := ((point.X-start.X)*line.X + (point.Y-start.Y)*line.Y) / lengthSquared
	t = math.Max(0, math.Min(1, t))
	delta := point.Sub(start.Add(line.Mul(t)))
	return delta.X*delta.X + delta.Y*delta.Y
}

func segmentsIntersect(a0, a1, b0, b1 Vec2) bool {
	const epsilon = 1e-8
	orient := func(a, b, c Vec2) float64 {
		ab, ac := b.Sub(a), c.Sub(a)
		return ab.X*ac.Y - ab.Y*ac.X
	}
	onSegment := func(a, b, point Vec2) bool {
		return point.X >= math.Min(a.X, b.X)-epsilon && point.X <= math.Max(a.X, b.X)+epsilon &&
			point.Y >= math.Min(a.Y, b.Y)-epsilon && point.Y <= math.Max(a.Y, b.Y)+epsilon
	}
	o1, o2, o3, o4 := orient(a0, a1, b0), orient(a0, a1, b1), orient(b0, b1, a0), orient(b0, b1, a1)
	if ((o1 > epsilon && o2 < -epsilon) || (o1 < -epsilon && o2 > epsilon)) &&
		((o3 > epsilon && o4 < -epsilon) || (o3 < -epsilon && o4 > epsilon)) {
		return true
	}
	return (math.Abs(o1) <= epsilon && onSegment(a0, a1, b0)) ||
		(math.Abs(o2) <= epsilon && onSegment(a0, a1, b1)) ||
		(math.Abs(o3) <= epsilon && onSegment(b0, b1, a0)) ||
		(math.Abs(o4) <= epsilon && onSegment(b0, b1, a1))
}

func validateLaneIndices(indices []int, lanes int) error {
	seen := make(map[int]bool, len(indices))
	for _, lane := range indices {
		if lane < 0 || lane >= lanes {
			return fmt.Errorf("lane %d outside [0,%d)", lane, lanes)
		}
		if seen[lane] {
			return fmt.Errorf("lane %d is duplicated", lane)
		}
		seen[lane] = true
	}
	return nil
}

// Movement returns the configured movement for a lane, independent of phase.
// The returned movement is narrowed to the exact connectors that originate in
// that lane, so callers cannot accidentally select an unrelated target lane
// from the movement-wide union.
func (control TrafficControl) Movement(fromWayID, toWayID string, lane int) (TrafficMovementConfig, bool) {
	var result TrafficMovementConfig
	pairs := make(map[[2]int]bool)
	for _, phase := range control.Phases {
		for _, movement := range phase.AllowedMovements {
			if movement.FromWayID != fromWayID || movement.ToWayID != toWayID {
				continue
			}
			for _, connection := range movement.ConnectionsFrom(lane) {
				pair := [2]int{connection.FromLane, connection.ToLane}
				if pairs[pair] {
					continue
				}
				if len(result.LaneConnections) == 0 {
					result = movement
					result.FromLanes = []int{lane}
					result.ToLanes = nil
					result.LaneConnections = nil
				}
				pairs[pair] = true
				result.LaneConnections = append(result.LaneConnections, connection)
			}
		}
	}
	if len(result.LaneConnections) == 0 {
		return TrafficMovementConfig{}, false
	}
	result.ToLanes = connectionLaneUnion(result.LaneConnections, false)
	return result, true
}

// HasMovement reports whether at least one lane connector joins the two ways.
func (control TrafficControl) HasMovement(fromWayID, toWayID string) bool {
	for _, phase := range control.Phases {
		for _, movement := range phase.AllowedMovements {
			if movement.FromWayID == fromWayID && movement.ToWayID == toWayID &&
				(len(movement.LaneConnections) > 0 || (len(movement.FromLanes) > 0 && len(movement.ToLanes) > 0)) {
				return true
			}
		}
	}
	return false
}

// ConnectionsFrom returns the explicit target connectors for one source lane.
func (movement TrafficMovementConfig) ConnectionsFrom(lane int) []LaneConnectionConfig {
	connections := make([]LaneConnectionConfig, 0, len(movement.LaneConnections))
	for _, connection := range movement.LaneConnections {
		if connection.FromLane == lane {
			connections = append(connections, connection)
		}
	}
	// Preserve compatibility with TrafficControl values assembled directly by
	// older callers rather than normalized through BuildGraph.
	if len(connections) == 0 && len(movement.LaneConnections) == 0 && containsLane(movement.FromLanes, lane) {
		for _, target := range movement.ToLanes {
			connections = append(connections, LaneConnectionConfig{FromLane: lane, ToLane: target})
		}
	}
	return connections
}

// TargetLanes returns the sorted, unique receiving lanes reachable from one
// exact incoming lane.
func (movement TrafficMovementConfig) TargetLanes(lane int) []int {
	seen := make(map[int]bool)
	for _, connection := range movement.ConnectionsFrom(lane) {
		seen[connection.ToLane] = true
	}
	targets := make([]int, 0, len(seen))
	for target := range seen {
		targets = append(targets, target)
	}
	sort.Ints(targets)
	return targets
}

func containsLane(lanes []int, lane int) bool {
	for _, candidate := range lanes {
		if candidate == lane {
			return true
		}
	}
	return false
}
