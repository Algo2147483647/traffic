package metro

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	geometry "traffic/internal/foundation/geometry"
	metroops "traffic/internal/operations/metro"
)

type TransferSpec struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	A       [2]int  `json:"a"`
	B       [2]int  `json:"b"`
	Seconds float64 `json:"seconds"`
}
type Layout struct {
	HighestLayer int             `json:"highest_layer,omitempty"`
	Transfers    []TransferSpec  `json:"transfers,omitempty"`
	CarLength    float64         `json:"car_length_m"`
	CarWidth     float64         `json:"car_width_m"`
	TunnelWidth  float64         `json:"tunnel_inner_diameter_m"`
	TunnelOuter  float64         `json:"tunnel_outer_diameter_m"`
	TrackSpacing float64         `json:"track_spacing_m"`
	Defaults     metroops.Config `json:"defaults"`
	Lines        []LineSpec      `json:"lines"`
}
type LineSpec struct {
	Elevation       float64      `json:"elevation_m"`
	Circular        bool         `json:"circular,omitempty"`
	ReverseRoute    []Point      `json:"reverse_route,omitempty"`
	ID              int          `json:"id"`
	Name            string       `json:"name"`
	Layer           int          `json:"layer"`
	Stations        []Point      `json:"stations"`
	Route           []Point      `json:"route"`
	Stops           [][2]float64 `json:"stop_chainages"`
	CrossingStation int          `json:"crossing_station"`
	LoopSpeed       float64      `json:"loop_speed_kmh"`
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
	for i := range l.Lines {
		s := &l.Lines[i]
		if s.Elevation == 0 {
			s.Elevation = float64(s.Layer) * 5
		}
		if math.IsNaN(s.Elevation) || math.IsInf(s.Elevation, 0) || s.Elevation >= 0 {
			return fmt.Errorf("invalid metro elevation")
		}
		if s.Layer == 0 {
			s.Layer = int(math.Floor(s.Elevation / 5))
		}
	}
	if l.CarLength < 10 || l.CarLength > 30 || l.CarWidth < 2 || l.CarWidth > 4 || l.TunnelWidth < l.CarWidth+1 || l.TunnelOuter <= l.TunnelWidth || l.TrackSpacing <= l.TunnelOuter {
		return fmt.Errorf("invalid metro physical dimensions")
	}
	if len(l.Lines) == 0 {
		return fmt.Errorf("metro layout has no lines")
	}
	if err := l.Defaults.Validate(); err != nil {
		return fmt.Errorf("metro defaults: %w", err)
	}
	if l.HighestLayer > 0 || l.HighestLayer < -16 {
		return fmt.Errorf("invalid highest metro layer")
	}
	ids := map[int]bool{}
	for _, s := range l.Lines {
		if ids[s.ID] || s.ID < 0 || len(s.Stations) < 3 || len(s.Route) < 4 || len(s.Stops) != len(s.Stations) || s.CrossingStation >= len(s.Stations) || s.LoopSpeed <= 0 {
			return fmt.Errorf("invalid metro line %d", s.ID)
		}
		ids[s.ID] = true
		if s.Circular && (len(s.ReverseRoute) < 4 || math.Hypot(s.ReverseRoute[0].X-s.ReverseRoute[len(s.ReverseRoute)-1].X, s.ReverseRoute[0].Y-s.ReverseRoute[len(s.ReverseRoute)-1].Y) > 1e-5) {
			return fmt.Errorf("circular line needs closed reverse track")
		}
		if math.Hypot(s.Route[0].X-s.Route[len(s.Route)-1].X, s.Route[0].Y-s.Route[len(s.Route)-1].Y) > 1e-5 {
			return fmt.Errorf("line %d route is not closed", s.ID)
		}
		for i := 1; i < len(s.Route); i++ {
			if math.Hypot(s.Route[i].X-s.Route[i-1].X, s.Route[i].Y-s.Route[i-1].Y) < 1e-8 {
				return fmt.Errorf("zero-length metro segment")
			}
		}
		if s.Circular {
			for i, p := range s.ReverseRoute {
				if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
					return fmt.Errorf("nonfinite ring track")
				}
				if i > 0 && math.Hypot(p.X-s.ReverseRoute[i-1].X, p.Y-s.ReverseRoute[i-1].Y) < 1e-8 {
					return fmt.Errorf("zero-length reverse track")
				}
			}
			for i := 1; i < len(s.Stops); i++ {
				if s.Stops[i][0] <= s.Stops[i-1][0] || (i > 1 && s.Stops[i][1] >= s.Stops[i-1][1]) {
					return fmt.Errorf("invalid ring stop order")
				}
			}
		}
		total := routeLength(s.Route)
		for i, st := range s.Stops {
			for direction, d := range st {
				limit := total
				if s.Circular && direction == 1 {
					limit = routeLength(s.ReverseRoute)
				}
				if math.IsNaN(d) || math.IsInf(d, 0) || d < 0 || d >= limit {
					return fmt.Errorf("invalid station chainage")
				}
			}
			if !s.Circular && i > 0 && (st[0] <= s.Stops[i-1][0] || st[1] >= s.Stops[i-1][1]) {
				return fmt.Errorf("invalid directional stop order")
			}
		}
	}
	return l.validateTransfers()
}
func routeLength(p []Point) float64 {
	d := 0.0
	for i := 1; i < len(p); i++ {
		d += math.Hypot(p[i].X-p[i-1].X, p[i].Y-p[i-1].Y)
	}
	return d
}
func (l *Layout) validateTransfers() error {
	ids := map[string]bool{}
	edges := map[[4]int]bool{}
	point := func(p [2]int) (Point, bool) {
		for _, line := range l.Lines {
			if line.ID == p[0] && p[1] >= 0 && p[1] < len(line.Stations) {
				return line.Stations[p[1]], true
			}
		}
		return Point{}, false
	}
	for _, t := range l.Transfers {
		a, ok := point(t.A)
		b, bok := point(t.B)
		key := [4]int{t.A[0], t.A[1], t.B[0], t.B[1]}
		reverse := [4]int{t.B[0], t.B[1], t.A[0], t.A[1]}
		if t.ID == "" || ids[t.ID] || edges[key] || edges[reverse] || !ok || !bok || t.A[0] == t.B[0] || math.Hypot(a.X-b.X, a.Y-b.Y) > 1 || math.IsNaN(t.Seconds) || math.IsInf(t.Seconds, 0) || t.Seconds < 1 || t.Seconds > 1800 {
			return fmt.Errorf("invalid transfer %s", t.ID)
		}
		ids[t.ID] = true
		edges[key] = true
	}
	return nil
}
func SamplePose(route []Point, chain []float64, d float64) CarPose {
	total := chain[len(chain)-1]
	d = math.Mod(d, total)
	if d < 0 {
		d += total
	}
	i := sort.Search(len(chain), func(i int) bool { return chain[i] >= d })
	if i == 0 {
		i = 1
	}
	a, b := route[i-1], route[i]
	f := (d - chain[i-1]) / (chain[i] - chain[i-1])
	return CarPose{Position: Point{X: a.X + (b.X-a.X)*f, Y: a.Y + (b.Y-a.Y)*f}, Heading: math.Atan2(b.Y-a.Y, b.X-a.X)}
}

type Point = geometry.Point
type CarPose = geometry.Pose
