package metro

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

type Layout struct {
	CarLength    float64    `json:"car_length_m"`
	CarWidth     float64    `json:"car_width_m"`
	TunnelWidth  float64    `json:"tunnel_inner_diameter_m"`
	TunnelOuter  float64    `json:"tunnel_outer_diameter_m"`
	TrackSpacing float64    `json:"track_spacing_m"`
	Defaults     Config     `json:"defaults"`
	Lines        []LineSpec `json:"lines"`
}
type LineSpec struct {
	ID              int          `json:"id"`
	Name            string       `json:"name"`
	Layer           int          `json:"layer"`
	Stations        []Point      `json:"stations"`
	Route           []Point      `json:"route"`
	Stops           [][2]float64 `json:"stop_chainages"`
	CrossingStation int          `json:"crossing_station"`
	LoopSpeed       float64      `json:"loop_speed_kmh"`
}
type CarPose struct {
	Position Point   `json:"position"`
	Heading  float64 `json:"heading"`
}

func LoadLayout(path string) (*Layout, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var l Layout
	if e = json.Unmarshal(b, &l); e != nil {
		return nil, e
	}
	if e = l.Validate(); e != nil {
		return nil, e
	}
	return &l, nil
}
func (l *Layout) Validate() error {
	if l.CarLength < 10 || l.CarLength > 30 || l.CarWidth < 2 || l.CarWidth > 4 || l.TunnelWidth < l.CarWidth+1 || l.TunnelOuter <= l.TunnelWidth || l.TrackSpacing <= l.TunnelOuter {
		return fmt.Errorf("invalid metro physical dimensions")
	}
	if len(l.Lines) == 0 {
		return fmt.Errorf("metro layout has no lines")
	}
	if err := New(nil, l.Defaults).Configure(l.Defaults); err != nil {
		return fmt.Errorf("metro defaults: %w", err)
	}
	ids := map[int]bool{}
	for _, s := range l.Lines {
		if ids[s.ID] || s.ID < 0 || s.Layer >= 0 || s.Layer < -16 || len(s.Stations) < 3 || len(s.Route) < 4 || len(s.Stops) != len(s.Stations) || s.CrossingStation >= len(s.Stations) || s.LoopSpeed <= 0 {
			return fmt.Errorf("invalid metro line %d", s.ID)
		}
		ids[s.ID] = true
		if math.Hypot(s.Route[0].X-s.Route[len(s.Route)-1].X, s.Route[0].Y-s.Route[len(s.Route)-1].Y) > 1e-5 {
			return fmt.Errorf("line %d route is not closed", s.ID)
		}
		for i := 1; i < len(s.Route); i++ {
			if math.Hypot(s.Route[i].X-s.Route[i-1].X, s.Route[i].Y-s.Route[i-1].Y) < 1e-8 {
				return fmt.Errorf("zero-length metro segment")
			}
		}
		total := routeLength(s.Route)
		for i, st := range s.Stops {
			for _, d := range st {
				if d < 0 || d >= total {
					return fmt.Errorf("invalid station chainage")
				}
			}
			if i > 0 && (st[0] <= s.Stops[i-1][0] || st[1] >= s.Stops[i-1][1]) {
				return fmt.Errorf("invalid directional stop order")
			}
		}
	}
	return nil
}
func routeLength(p []Point) float64 {
	d := 0.0
	for i := 1; i < len(p); i++ {
		d += math.Hypot(p[i].X-p[i-1].X, p[i].Y-p[i-1].Y)
	}
	return d
}
func NewNetwork(l *Layout, c Config) *Runtime {
	root := &Runtime{config: c, layout: l}
	gates := map[int]*crossingGate{}
	for _, spec := range l.Lines {
		local := c
		local.Demand = c.Demand / float64(len(l.Lines))
		local.Seed += int64(spec.ID) * 101
		r := New(spec.Stations, local)
		r.layout = l
		if floor, ok := c.LineLayers[fmt.Sprint(spec.ID)]; ok {
			spec.Layer = floor
		}
		copy := spec
		r.spec = &copy
		r.lineID = spec.ID
		if gates[spec.Layer] == nil {
			gates[spec.Layer] = &crossingGate{}
		}
		r.gate = gates[spec.Layer]
		r.chain = make([]float64, len(spec.Route))
		for i := 1; i < len(r.chain); i++ {
			r.chain[i] = r.chain[i-1] + math.Hypot(spec.Route[i].X-spec.Route[i-1].X, spec.Route[i].Y-spec.Route[i-1].Y)
		}
		for i := range r.platforms {
			r.platforms[i] = [2]int{}
		}
		for _, t := range r.trains {
			r.platforms[t.Station][r.platform(t.Station, t.Direction)] = t.ID
			t.Length = float64(t.carCount) * l.CarLength
			t.routeDistance = spec.Stops[t.Station][directionIndex(t.Direction)]
			t.Position = r.pose(t.routeDistance).Position
		}
		root.lines = append(root.lines, r)
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
func (r *Runtime) pose(d float64) CarPose {
	total := r.chain[len(r.chain)-1]
	d = math.Mod(d, total)
	if d < 0 {
		d += total
	}
	i := sort.Search(len(r.chain), func(i int) bool { return r.chain[i] >= d })
	if i == 0 {
		i = 1
	}
	a, b := r.spec.Route[i-1], r.spec.Route[i]
	f := (d - r.chain[i-1]) / (r.chain[i] - r.chain[i-1])
	return CarPose{Point{a.X + (b.X-a.X)*f, a.Y + (b.Y-a.Y)*f}, math.Atan2(b.Y-a.Y, b.X-a.X)}
}
func (r *Runtime) turn(t *train, dt float64) {
	remaining := t.Authority - t.Distance
	target := math.Min(r.spec.LoopSpeed/3.6, math.Sqrt(dt*dt+2*math.Max(0, remaining))-dt)
	t.Speed = math.Max(0, math.Min(t.Speed+dt, math.Max(t.Speed-dt, target)))
	step := math.Min(remaining, t.Speed*dt)
	t.Distance += step
	t.routeDistance += step
	t.Position = r.pose(t.routeDistance).Position
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
		t.Position = r.pose(t.routeDistance).Position
		t.State = "dwelling"
		t.needsLoop = false
		t.Speed = 0
		t.Authority = 0
		t.arrived = r.time
		t.scheduled = r.time
	}
}
func (r *Runtime) configureLayers(c Config) error {
	gates := map[int]*crossingGate{}
	floors := map[int]int{}
	for _, line := range r.lines {
		f := line.floor()
		if v, ok := c.LineLayers[fmt.Sprint(line.lineID)]; ok {
			f = v
		}
		if f >= 0 || f < -16 {
			return fmt.Errorf("metro levels must be -1 to -16")
		}
		floors[line.lineID] = f
		if gates[f] == nil {
			gates[f] = &crossingGate{}
		}
		owner := line.gate.owner
		if owner != 0 && owner/100 == line.lineID {
			if gates[f].owner != 0 && gates[f].owner != owner {
				return fmt.Errorf("crossing occupied: wait for trains to clear before combining levels")
			}
			gates[f].owner = owner
		}
	}
	for _, line := range r.lines {
		f := floors[line.lineID]
		if line.spec != nil {
			line.spec.Layer = f
		}
		line.gate = gates[f]
	}
	return nil
}
