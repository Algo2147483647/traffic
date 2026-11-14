package experiments

import (
	"context"
	"fmt"
	"math"
	"time"
	"traffic/internal/model"
	"traffic/internal/sim"
)

type Result struct {
	Algorithm   string        `json:"algorithm"`
	Seed        int64         `json:"seed"`
	Horizon     float64       `json:"horizon_s"`
	ActualTime  float64       `json:"actual_time_s"`
	WallSeconds float64       `json:"wall_seconds"`
	Status      string        `json:"status"`
	Error       string        `json:"error,omitempty"`
	Taxi        *sim.TaxiView `json:"taxi"`
}

// Each invocation owns its simulation. The taxi-lab executable isolates these runs from the desktop.
func Run(ctx context.Context, g *model.Graph, c sim.TaxiConfig, horizon float64) (Result, error) {
	r := Result{Algorithm: c.Algorithm, Seed: c.Seed, Horizon: horizon, Status: "running"}
	if horizon <= 0 || horizon > 86400 {
		return r, fmt.Errorf("horizon must be in (0,86400]")
	}
	s := sim.New(g)
	defer s.Close()
	if e := s.SetMode("taxi", c); e != nil {
		return r, e
	}
	start := time.Now()
	var snap sim.Snapshot
	for {
		select {
		case <-ctx.Done():
			r.Status = "cancelled"
			r.Error = ctx.Err().Error()
			goto done
		default:
		}
		remaining := horizon - s.Snapshot().Time
		if remaining <= 1e-8 {
			r.Status = "completed"
			break
		}
		s.Step(math.Min(.05, remaining))
		snap = s.Snapshot()
		if snap.Taxi.LastError != "" {
			r.Status = "failed"
			r.Error = snap.Taxi.LastError
			break
		}
		if snap.Time >= horizon {
			r.Status = "completed"
			break
		}
		if snap.Taxi.Solving {
			time.Sleep(time.Millisecond)
		}
	}
done:
	snap = s.Snapshot()
	r.ActualTime = snap.Time
	r.WallSeconds = time.Since(start).Seconds()
	r.Taxi = snap.Taxi
	return r, nil
}
