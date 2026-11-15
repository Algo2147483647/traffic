package experiments

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Specification identifies a reproducible run independently of any traffic executor.
type Specification struct {
	Algorithm string
	Seed      int64
	Horizon   float64
}
type Observation[T any] struct {
	Time    float64
	Value   T
	Solving bool
	Error   string
}
type Session[T any] interface {
	Step(float64)
	Observe() Observation[T]
	Close()
}
type Result[T any] struct {
	Algorithm   string  `json:"algorithm"`
	Seed        int64   `json:"seed"`
	Horizon     float64 `json:"horizon_s"`
	ActualTime  float64 `json:"actual_time_s"`
	WallSeconds float64 `json:"wall_seconds"`
	Status      string  `json:"status"`
	Error       string  `json:"error,omitempty"`
	Taxi        T       `json:"taxi"`
}

// Run receives an assembled session. It cannot import or construct a road/metro world.
func Run[T any](ctx context.Context, c Specification, factory func() (Session[T], error)) (Result[T], error) {
	r := Result[T]{Algorithm: c.Algorithm, Seed: c.Seed, Horizon: c.Horizon, Status: "running"}
	if math.IsNaN(c.Horizon) || math.IsInf(c.Horizon, 0) || c.Horizon <= 0 || c.Horizon > 86400 {
		return r, fmt.Errorf("horizon must be in (0,86400]")
	}
	s, err := factory()
	if err != nil {
		return r, err
	}
	defer s.Close()
	start := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			r.Status = "cancelled"
			r.Error = err.Error()
			break
		}
		obs := s.Observe()
		if obs.Error != "" {
			r.Status = "failed"
			r.Error = obs.Error
			break
		}
		remaining := c.Horizon - obs.Time
		if remaining <= 1e-8 {
			r.Status = "completed"
			break
		}
		s.Step(math.Min(.05, remaining))
		if obs.Solving {
			select {
			case <-ctx.Done():
			case <-time.After(time.Millisecond):
			}
		}
	}
	obs := s.Observe()
	r.ActualTime = obs.Time
	r.Taxi = obs.Value
	r.WallSeconds = time.Since(start).Seconds()
	return r, nil
}
