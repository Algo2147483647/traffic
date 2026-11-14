package model

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"traffic/internal/algorithms"
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
		m, found := algorithms.Lookup(id)
		if !found || (m.Capability != "geometry" && m.Capability != "phases") {
			return c, fmt.Errorf("unknown planner %s", id)
		}
		if (id == c.GeometryAlgorithm && m.Capability != "geometry") || (id == c.PhaseAlgorithm && m.Capability != "phases") {
			return c, fmt.Errorf("planner capability mismatch")
		}
		body, e := json.Marshal(c)
		if e != nil {
			return c, e
		}
		result, e := algorithms.Invoke(context.Background(), algorithms.Request{Version: 1, Algorithm: id, AlgorithmVersion: m.AlgorithmVersion, Capability: m.Capability, Request: 1, Seed: c.Simulation.Seed, Payload: body})
		if e != nil {
			return c, e
		}
		var candidate CityConfig
		if e = algorithms.Decode(result.Payload, &candidate); e != nil {
			return c, e
		}
		candidate.Metro = c.Metro
		if m.Capability == "phases" {
			// A phase worker cannot modify topology, dimensions or metro data.
			baseline := c
			baseline.Intersections = append([]IntersectionConfig(nil), c.Intersections...)
			baseline.GeometryAlgorithm = ""
			baseline.PhaseAlgorithm = ""
			for i := range candidate.Intersections {
				if i >= len(baseline.Intersections) || candidate.Intersections[i].ID != baseline.Intersections[i].ID {
					return c, fmt.Errorf("phase worker changed junction identities")
				}
			}
			if len(candidate.Intersections) != len(baseline.Intersections) {
				return c, fmt.Errorf("phase worker changed junction count")
			}
			for i := range baseline.Intersections {
				baseline.Intersections[i].Traffic = candidate.Intersections[i].Traffic
			}
			candidate = baseline
		}
		candidate.GeometryAlgorithm = ""
		candidate.PhaseAlgorithm = ""
		if _, e = BuildGraph(candidate); e != nil {
			return c, e
		}
		return candidate, nil
	}
	data, err := json.Marshal(c)
	if err != nil {
		return c, err
	}
	var detached CityConfig
	if err = json.Unmarshal(data, &detached); err != nil {
		return c, err
	}
	detached.Metro = c.Metro
	return p(detached)
}
