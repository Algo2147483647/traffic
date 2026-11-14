package model

import (
	"encoding/json"
	"fmt"
	"sync"
)

type PlanAlgorithm func(CityConfig) (CityConfig, error)

var planMu sync.RWMutex
var planners = map[string]PlanAlgorithm{}

// RegisterPlanner allows code-based geometry/topology or signal-plan generation.
// The returned configuration always goes through ordinary graph/phase validation.
func RegisterPlanner(id string, planner PlanAlgorithm) error {
	planMu.Lock()
	defer planMu.Unlock()
	if id == "" || planner == nil {
		return fmt.Errorf("invalid planner")
	}
	if _, ok := planners[id]; ok {
		return fmt.Errorf("duplicate planner %s", id)
	}
	planners[id] = planner
	return nil
}
func applyPlanner(id string, c CityConfig) (CityConfig, error) {
	planMu.RLock()
	p, ok := planners[id]
	planMu.RUnlock()
	if !ok {
		return c, fmt.Errorf("unknown planner %s", id)
	}
	data, err := json.Marshal(c)
	if err != nil {
		return c, err
	}
	var detached CityConfig
	if err = json.Unmarshal(data, &detached); err != nil {
		return c, err
	}
	return p(detached)
}
