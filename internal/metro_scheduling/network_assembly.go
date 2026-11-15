package metroscheduling

import (
	"fmt"
	"math"
	"sort"
	track "traffic/internal/infrastructure/metro"
)

func NewNetwork(l *Layout, c Config) *Runtime {
	levels := map[string]int{}
	for key, floor := range c.LineLayers {
		if l.HighestLayer == 0 || floor <= l.HighestLayer {
			levels[key] = floor
		}
	}
	c.LineLayers = levels
	root := &Runtime{config: c, layout: l}
	gates := map[float64]*crossingGate{}
	for _, spec := range l.Lines {
		local := c
		local.Demand = c.Demand / float64(len(l.Lines))
		local.Seed += int64(spec.ID) * 101
		r := newRuntime(spec.Stations, local, false)
		r.layout = l
		if floor, ok := c.LineLayers[fmt.Sprint(spec.ID)]; ok {
			spec.Layer = floor
			spec.Elevation = float64(floor) * 5
		}
		if spec.Elevation == 0 {
			spec.Elevation = float64(spec.Layer) * 5
		}
		if h, ok := c.LineElevations[fmt.Sprint(spec.ID)]; ok {
			spec.Elevation = h
		}
		copy := spec
		r.spec = &copy
		r.lineID = spec.ID
		if gates[spec.Elevation] == nil {
			gates[spec.Elevation] = &crossingGate{}
		}
		r.gate = gates[spec.Elevation]
		r.chain = track.Chain(spec.Route)
		if spec.Circular {
			r.initCircular()
		} else {
			r.initDirectional()
		}
		for i := range r.platforms {
			r.platforms[i] = [2]int{}
		}
		for _, t := range r.trains {
			r.platforms[t.Station][r.platform(t.Station, t.Direction)] = t.ID
			t.Length = float64(t.carCount) * l.CarLength
			t.routeDistance = spec.Stops[t.Station][directionIndex(t.Direction)]
			t.Position = r.directionPose(t.Direction, t.routeDistance).Position
		}
		root.lines = append(root.lines, r)
	}
	_ = root.configureLayers(c)
	root.initTransfers()
	if plans, err := root.servicePlans(c); err == nil {
		root.applyServicePlans(plans)
	}
	return root
}
func directionIndex(d int) int {
	if d < 0 {
		return 1
	}
	return 0
}
func (r *Runtime) carLength() float64 {
	if r.layout != nil {
		return r.layout.CarLength
	}
	return 22.8
}
func (r *Runtime) elevation() float64 {
	if r.spec != nil {
		if r.spec.Elevation != 0 {
			return r.spec.Elevation
		}
		return float64(r.spec.Layer) * 5
	}
	return -5
}
func (r *Runtime) floor() int {
	if r.spec != nil {
		return r.spec.Layer
	}
	return -1
}
func (r *Runtime) crossing() int {
	if r.spec != nil {
		return r.spec.CrossingStation
	}
	return 2
}
func (r *Runtime) pose(d float64) CarPose { return track.SamplePose(r.spec.Route, r.chain, d) }
func (r *Runtime) directionPose(direction int, d float64) CarPose {
	if r.circular() && direction < 0 {
		return track.SamplePose(r.spec.ReverseRoute, r.reverseChain, d)
	}
	return r.pose(d)
}
func (r *Runtime) turn(t *train, dt float64) {
	remaining := t.Authority - t.Distance
	target := math.Min(r.spec.LoopSpeed/3.6, math.Sqrt(dt*dt+2*math.Max(0, remaining))-dt)
	t.Speed = math.Max(0, math.Min(t.Speed+dt, math.Max(t.Speed-dt, target)))
	step := math.Min(remaining, t.Speed*dt)
	t.Distance += step
	t.routeDistance += step
	t.Position = r.directionPose(t.Direction, t.routeDistance).Position
	// Once the tail clears the arrival berth, an incoming train may use it.
	// The departure berth remains reserved for this entire terminal circuit.
	if t.Distance >= t.Length {
		p := r.platform(t.Station, -t.Direction)
		if r.platforms[t.Station][p] == t.ID {
			r.platforms[t.Station][p] = 0
		}
	}
	if t.Authority-t.Distance < 1e-5 {
		t.routeDistance = r.spec.Stops[t.Station][directionIndex(t.Direction)]
		t.Position = r.directionPose(t.Direction, t.routeDistance).Position
		t.State = "dwelling"
		t.needsLoop = false
		t.Speed = 0
		t.Authority = 0
		t.arrived = r.time
		t.scheduled = r.time
	}
}
func (r *Runtime) configureLayers(c Config) error {
	floors := map[int]float64{}
	heights := []float64{}
	for _, line := range r.lines {
		f := line.elevation()
		if v, ok := c.LineLayers[fmt.Sprint(line.lineID)]; ok {
			f = float64(v) * 5
		}
		if v, ok := c.LineElevations[fmt.Sprint(line.lineID)]; ok {
			f = v
		}
		if math.IsNaN(f) || math.IsInf(f, 0) || f >= 0 {
			return fmt.Errorf("metro elevation must be finite and below 0 m")
		}
		floors[line.lineID] = f
		heights = append(heights, f)
	}
	sort.Float64s(heights)
	gates := map[float64]*crossingGate{}
	for _, h := range heights {
		if gates[h] == nil {
			gates[h] = &crossingGate{}
		}
	}
	for _, line := range r.lines {
		f := floors[line.lineID]
		owner := line.gate.owner
		if owner != 0 && owner/100 == line.lineID {
			if gates[f].owner != 0 && gates[f].owner != owner {
				return fmt.Errorf("crossing occupied: wait before combining overlapping tunnel elevations")
			}
			gates[f].owner = owner
		}
	}
	for _, line := range r.lines {
		f := floors[line.lineID]
		if line.spec != nil {
			line.spec.Elevation = f
			line.spec.Layer = int(math.Floor(f / 5))
		}
		line.gate = gates[f]
	}
	return nil
}
