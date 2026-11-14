package metro

import "math"

func (r *Runtime) circular() bool { return r.spec != nil && r.spec.Circular }
func (r *Runtime) initCircular() {
	r.reverseChain = make([]float64, len(r.spec.ReverseRoute))
	for i := 1; i < len(r.reverseChain); i++ {
		a, b := r.spec.ReverseRoute[i-1], r.spec.ReverseRoute[i]
		r.reverseChain[i] = r.reverseChain[i-1] + math.Hypot(b.X-a.X, b.Y-a.Y)
	}
	n := len(r.stations)
	r.segments = make([]int, 2*n)
	r.lengths = make([]float64, 2*n)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		forward := r.chain[len(r.chain)-1]
		reverse := r.reverseChain[len(r.reverseChain)-1]
		r.lengths[i] = math.Mod(r.spec.Stops[j][0]-r.spec.Stops[i][0]+forward, forward)
		r.lengths[n+i] = math.Mod(r.spec.Stops[i][1]-r.spec.Stops[j][1]+reverse, reverse)
	}
	r.trains[1].Station = n / 2
	for _, t := range r.trains {
		t.Next = (t.Station + t.Direction + n) % n
	}
}
