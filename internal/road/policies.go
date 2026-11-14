package sim

import (
	"fmt"
	"math"
	"sort"
	"sync"
	model "traffic/internal/infrastructure/road"
)

// Strategies are per-simulation instances. Nil hooks retain the verified baseline.
// Inputs are detached values; hooks propose decisions, never mutate road state.
type RoadCost struct {
	WayID                      string
	Length, Speed, DefaultCost float64
}
type FlowRequest struct {
	Intersection, From, To string
	Lane                   int
	Time                   float64
}
type Strategies struct {
	RouteCost    func(RoadCost) float64
	DesiredSpeed func(limit float64) float64
	LaneChoice   func(current, proposed, lanes int) int
	PhaseScores  func([]float64) []float64
	FlowPermit   func(FlowRequest) bool
	TaxiCruise   func(vehicleID int, stops []TaxiStop, proposed string) string
	DemandDelay  func(rate, draw float64) float64
}
type PolicyInfo struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

var profileMu sync.RWMutex
var profiles = map[string]struct {
	info    PolicyInfo
	factory func(map[string]float64) Strategies
}{}

func RegisterStrategies(id, version string, f func(map[string]float64) Strategies) error {
	profileMu.Lock()
	defer profileMu.Unlock()
	if id == "" || version == "" || f == nil {
		return fmt.Errorf("invalid road policy")
	}
	if _, ok := profiles[id]; ok {
		return fmt.Errorf("duplicate road policy %s", id)
	}
	profiles[id] = struct {
		info    PolicyInfo
		factory func(map[string]float64) Strategies
	}{PolicyInfo{id, version}, f}
	return nil
}
func PolicyCatalog() []PolicyInfo {
	profileMu.RLock()
	defer profileMu.RUnlock()
	out := []PolicyInfo{{"baseline", "1"}}
	for _, v := range profiles {
		out = append(out, v.info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func NewWithPolicy(g *model.Graph, id string, parameters map[string]float64) (*Simulation, error) {
	s := New(g)
	if id == "baseline" || id == "" {
		return s, nil
	}
	profileMu.RLock()
	p, ok := profiles[id]
	profileMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown road policy %s", id)
	}
	copyParams := map[string]float64{}
	for k, v := range parameters {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("invalid policy parameter")
		}
		copyParams[k] = v
	}
	s.policies = p.factory(copyParams)
	s.resetLocked(g.Simulation.People)
	return s, nil
}
func (s *Simulation) policySpeed(limit float64) float64 {
	if s.policies.DesiredSpeed == nil {
		return limit
	}
	v := s.policies.DesiredSpeed(limit)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return limit
	}
	return math.Max(0, math.Min(limit, v))
}
