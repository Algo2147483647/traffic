package taxi

import (
	"fmt"
	"math"
)

func DefaultConfig() Config {
	return Config{Fleet: 32, Demand: 2, Algorithm: "nearest", Seed: 71429, MaxWait: 600, Dwell: 20, Interval: 2}
}
func NormalizeConfig(c Config) (Config, error) {
	if c.Algorithm == "" {
		c.Algorithm = "nearest"
	}
	if c.MaxWait == 0 {
		c.MaxWait = 600
	}
	if c.Dwell == 0 {
		c.Dwell = 20
	}
	if c.Interval == 0 {
		c.Interval = 2
	}
	if c.Seed == 0 {
		c.Seed = 71429
	}
	for _, v := range []float64{c.Demand, c.MaxWait, c.Dwell, c.Interval} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return c, fmt.Errorf("taxi values must be finite")
		}
	}
	if c.Fleet < 1 || c.Demand < 0 || c.MaxWait <= 0 || c.Dwell < 0 || c.Interval <= 0 {
		return c, fmt.Errorf("invalid taxi configuration")
	}
	for _, v := range c.Parameters {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e6 {
			return c, fmt.Errorf("invalid algorithm parameter")
		}
	}
	return c, nil
}
