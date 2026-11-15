package metro

import (
	"fmt"
	"math"
)

type ServiceConfig struct {
	Algorithm string  `json:"algorithm"`
	Headway   float64 `json:"target_headway_s"`
}
type Config struct {
	Service        *ServiceConfig     `json:"service,omitempty"`
	LineElevations map[string]float64 `json:"line_elevations_m,omitempty"`
	LineLayers     map[string]int     `json:"line_layers,omitempty"`
	Cars           int                `json:"cars"`
	MaxSpeed       float64            `json:"max_speed_kmh"`
	Enabled        bool               `json:"enabled"`
	Headway        float64            `json:"headway_s"`
	Demand         float64            `json:"passengers_per_minute"`
	Capacity       int                `json:"capacity"`
	Dwell          float64            `json:"dwell_s"`
	Seed           int64              `json:"seed"`
}

func DefaultConfig() Config {
	return Config{Cars: 6, MaxSpeed: 300, Enabled: true, Headway: 180, Demand: 12, Capacity: 120, Dwell: 20, Seed: 4181}
}

func (c Config) Validate() error {
	if c.MaxSpeed == 0 {
		c.MaxSpeed = 300
	}
	if c.Cars == 0 {
		c.Cars = 6
	}
	for _, elevation := range c.LineElevations {
		if math.IsNaN(elevation) || math.IsInf(elevation, 0) || elevation >= 0 {
			return fmt.Errorf("metro elevation must be a finite negative value in metres")
		}
	}
	for _, floor := range c.LineLayers {
		if floor >= 0 {
			return fmt.Errorf("metro levels must be -1 to -16")
		}
	}
	if c.Cars < 1 {
		return fmt.Errorf("cars must be positive")
	}
	if math.IsNaN(c.MaxSpeed) || math.IsInf(c.MaxSpeed, 0) || c.MaxSpeed <= 0 {
		return fmt.Errorf("metro maximum speed must be positive")
	}

	if math.IsNaN(c.Headway) || math.IsNaN(c.Demand) || math.IsNaN(c.Dwell) || math.IsInf(c.Headway, 0) || math.IsInf(c.Demand, 0) || math.IsInf(c.Dwell, 0) || c.Headway <= 0 || c.Demand < 0 || c.Capacity < 1 || c.Dwell < 0 {
		return fmt.Errorf("invalid metro settings")
	}
	return nil
}
