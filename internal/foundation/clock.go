package foundation

import "math"

const PhysicalStep = .05

// AdvanceFixed preserves fractional input time and stops at an algorithm barrier.
func (c *Clock) AdvanceFixed(dt float64, step func(float64)) {
	if dt <= 0 || math.IsNaN(dt) || math.IsInf(dt, 0) {
		return
	}
	c.remainder += dt
	for c.remainder >= PhysicalStep-1e-9 {
		before := c.Time
		step(PhysicalStep)
		if c.Time == before {
			c.remainder = 0
			return
		}
		c.remainder = math.Max(0, c.remainder-PhysicalStep)
	}
}
func (c *Clock) Reset() { c.Time = 0; c.Tick = 0; c.remainder = 0 }
