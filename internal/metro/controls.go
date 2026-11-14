package metro

import (
	"fmt"
	"math"
)

// Control proposes a short-lived speed ceiling and earliest departure time.
// Movement authority, interlocking, braking, dwell and capacity stay host-owned.
type Control struct {
	Train       int     `json:"train"`
	Speed       float64 `json:"speed_mps"`
	DepartAfter float64 `json:"depart_after_s"`
}

func (r *Runtime) ApplyControls(controls []Control) error {
	owners := map[int]*Runtime{}
	trains := map[int]*train{}
	if len(r.lines) > 0 {
		for _, line := range r.lines {
			for _, t := range line.trains {
				id := line.lineID*100 + t.ID
				owners[id] = line
				trains[id] = t
			}
		}
	} else {
		for _, t := range r.trains {
			owners[t.ID] = r
			trains[t.ID] = t
		}
	}
	seen := map[int]bool{}
	for _, v := range controls {
		owner := owners[v.Train]
		if owner == nil || seen[v.Train] || math.IsNaN(v.Speed) || math.IsInf(v.Speed, 0) || v.Speed < 0 || v.Speed > owner.config.MaxSpeed/3.6 || math.IsNaN(v.DepartAfter) || math.IsInf(v.DepartAfter, 0) || v.DepartAfter < r.time || v.DepartAfter > r.time+5 {
			return fmt.Errorf("invalid train control")
		}
		seen[v.Train] = true
	}
	for _, v := range controls {
		t := trains[v.Train]
		t.control = v
		t.controlUntil = owners[v.Train].time + 5
	}
	return nil
}

func (r *Runtime) ClearControls() {
	if r == nil {
		return
	}
	for _, line := range r.lines {
		line.ClearControls()
	}
	for _, t := range r.trains {
		t.controlUntil = 0
	}
}
