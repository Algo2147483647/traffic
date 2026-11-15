package metroscheduling

import "math"

// Arrival forecasts use current train progress and scheduled dwell. Future
// block contention and future boarding demand are not predictable guarantees.
func travelEstimate(distance, speed, limit float64) float64 {
	distance = math.Max(0, distance)
	if distance == 0 {
		return 0
	}
	// Runtime acceleration and service braking are both 1 m/s².
	speed = math.Min(speed, limit)
	peak := math.Sqrt(distance + speed*speed/2)
	if peak <= limit {
		return math.Max(0, peak-speed) + peak
	}
	return math.Max(0, limit-speed) + limit + math.Max(0, distance-(limit*limit-speed*speed)/2-limit*limit/2)/limit
}
func (r *Runtime) arrivalEstimate(t *train, target, direction int) (float64, string, bool) {
	at, dir := t.Station, t.Direction
	elapsed := 0.0
	status := "estimated"
	if t.State == "waiting authority" {
		status = "waiting authority · estimated"
	}
	moving := t.State == "moving"
	if moving {
		elapsed = travelEstimate(t.Authority-t.Distance, t.Speed, r.config.MaxSpeed/3.6)
		at = t.Next
		if !r.circular() && (at == 0 || at == len(r.stations)-1) {
			dir = -dir
		}
	} else if at == target && dir == direction && t.State != "turning" && t.State != "scheduled" {
		return 0, "at platform", true
	} else {
		elapsed = math.Max(0, math.Max(t.scheduled, t.arrived+r.config.Dwell)-r.time)
		if t.State == "turning" {
			elapsed = math.Max(0, t.Authority-t.Distance) / r.spec.LoopSpeed
			if at == target && dir == direction {
				return elapsed, status, true
			}
		} else if t.needsLoop && r.spec != nil {
			total := r.chain[len(r.chain)-1]
			elapsed += math.Mod(r.spec.Stops[at][directionIndex(dir)]-math.Mod(t.routeDistance, total)+total, total) / r.spec.LoopSpeed
		}
	}
	for hop := 0; hop < 2*len(r.stations)+2; hop++ {
		if moving && at == target && dir == direction {
			return elapsed, status, true
		}
		if moving {
			elapsed += r.config.Dwell
			if !r.circular() && (at == 0 || at == len(r.stations)-1) {
				elapsed = math.Ceil((r.time+elapsed)/r.config.Headway)*r.config.Headway - r.time
				if r.spec != nil {
					total := r.chain[len(r.chain)-1]
					elapsed += math.Mod(r.spec.Stops[at][directionIndex(dir)]-r.spec.Stops[at][directionIndex(-dir)]+total, total) / r.spec.LoopSpeed
				}
			}
		}
		next := at + dir
		if r.circular() {
			next = (next + len(r.stations)) % len(r.stations)
		}
		if next < 0 || next >= len(r.stations) {
			return 0, "unavailable", false
		}
		seg := min(at, next)
		if r.spec != nil && !r.circular() && dir < 0 {
			seg += len(r.stations) - 1
		}
		if r.circular() {
			seg = at
			if dir < 0 {
				seg = len(r.stations) + next
			}
		}
		elapsed += travelEstimate(r.lengths[seg], 0, r.config.MaxSpeed/3.6)
		at = next
		moving = true
		if !r.circular() && (at == 0 || at == len(r.stations)-1) {
			dir = -dir
		}
	}
	return 0, "unavailable", false
}
func (r *Runtime) stationDetails(v *View) {
	for _, site := range v.Interchanges {
		detail := StationDetail{ID: site.ID, PassengerIDs: []int{}}
		belongs := func(id int) bool { return id == site.Stations[0] || id == site.Stations[1] }
		for _, j := range v.Journeys {
			if j.Index >= len(j.Plan) {
				continue
			}
			step := j.Plan[j.Index]
			if j.State == "transferring" && belongs(step.From) {
				detail.Transferring++
			}
			if (j.State == "waiting" || j.State == "transferring") && belongs(step.From) {
				detail.PassengerIDs = append(detail.PassengerIDs, j.ID)
			}
		}
		for _, station := range site.Stations {
			line, local := r.stationOwner(station)
			platform := PlatformDetail{Station: station, Line: line.lineID, Layer: line.floor(), Elevation: line.elevation()}
			for _, dir := range []int{1, -1} {
				next := local + dir
				if line.circular() {
					next = (next + len(line.stations)) % len(line.stations)
				}
				if next < 0 || next >= len(line.stations) {
					continue
				}
				d := PlatformDirection{Direction: dir, Toward: station - local + next, Waiting: len(line.queues[local][directionIndex(dir)]), Status: "no service"}
				if !line.config.Enabled {
					d.Status = "paused"
				} else {
					for _, t := range line.trains {
						if t.State == "withdrawn" {
							continue
						}
						candidate := t
						delay := 0.0
						depot := t.State == "depot"
						if depot {
							clone := *t
							clone.State = "dwelling"
							candidate = &clone
							if line.service != nil {
								delay = math.Max(.05, line.service.next[directionIndex(t.Direction)]-line.time)
							}
						}
						eta, status, ok := line.arrivalEstimate(candidate, local, dir)
						if depot {
							eta += delay
							status = "scheduled depot departure · estimated"
						}
						if ok && (d.ETA == nil || eta < *d.ETA) {
							value := eta
							d.ETA = &value
							d.Train = line.lineID*100 + t.ID
							d.Status = status
						}
					}
				}
				platform.Directions = append(platform.Directions, d)
			}
			detail.Platforms = append(detail.Platforms, platform)
		}
		v.StationDetails = append(v.StationDetails, detail)
	}
}
