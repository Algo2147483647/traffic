package demand

import "math"

// Arrivals owns the deterministic ordinary-trip source. Capacity saturation
// suspends generation, preserving the existing population-control semantics.
type Arrivals struct{ remainder float64 }

func (a *Arrivals) Advance(rate, dt float64, available int) int {
	if rate <= 0 || dt <= 0 || available <= 0 {
		return 0
	}
	a.remainder += rate * dt
	count := min(available, int(math.Floor(a.remainder+1e-12)))
	a.remainder -= float64(count)
	return count
}
