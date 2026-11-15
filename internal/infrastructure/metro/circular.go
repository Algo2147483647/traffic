package metro

import "math"

func Chain(route []Point) []float64 {
	out := make([]float64, len(route))
	for i := 1; i < len(route); i++ {
		out[i] = out[i-1] + math.Hypot(route[i].X-route[i-1].X, route[i].Y-route[i-1].Y)
	}
	return out
}
func SectionLengths(s LineSpec) []float64 {
	n := len(s.Stations) - 1
	if s.Circular {
		n++
	}
	out := make([]float64, 2*n)
	for i := 0; i < n; i++ {
		j := (i + 1) % len(s.Stations)
		if s.Circular {
			f, b := routeLength(s.Route), routeLength(s.ReverseRoute)
			out[i] = math.Mod(s.Stops[j][0]-s.Stops[i][0]+f, f)
			out[n+i] = math.Mod(s.Stops[i][1]-s.Stops[j][1]+b, b)
		} else {
			out[i] = s.Stops[j][0] - s.Stops[i][0]
			out[n+i] = s.Stops[i][1] - s.Stops[j][1]
		}
	}
	return out
}
