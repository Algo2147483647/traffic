package demand

import (
	"fmt"
	"math"
	"math/rand"
)

type Shares struct{ Car, Taxi, Metro float64 }

func (s Shares) Validate(metroAvailable bool) error {
	sum := s.Car + s.Taxi + s.Metro
	if math.IsNaN(sum) || math.IsInf(sum, 0) || math.Abs(sum-1) > 1e-9 || s.Car < 0 || s.Taxi < 0 || s.Metro < 0 {
		return fmt.Errorf("mode shares must be nonnegative and sum to one")
	}
	if s.Metro > 0 && !metroAvailable {
		return fmt.Errorf("metro execution is not enabled")
	}
	return nil
}
func (s Shares) Choose(r *rand.Rand) string {
	x := r.Float64()
	if x < s.Car {
		return "car"
	}
	if x < s.Car+s.Taxi {
		return "taxi"
	}
	return "metro"
}
