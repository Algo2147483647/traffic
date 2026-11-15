package metroscheduling

import track "traffic/internal/infrastructure/metro"

func (r *Runtime) circular() bool { return r.spec != nil && r.spec.Circular }
func (r *Runtime) initCircular() {
	r.reverseChain = track.Chain(r.spec.ReverseRoute)
	r.lengths = track.SectionLengths(*r.spec)
	r.segments = make([]int, len(r.lengths))
	n := len(r.stations)
	if len(r.trains) > 1 {
		r.trains[1].Station = n / 2
	}
	for _, t := range r.trains {
		t.Next = (t.Station + t.Direction + n) % n
	}
}
func (r *Runtime) initDirectional() {
	r.lengths = track.SectionLengths(*r.spec)
	r.segments = make([]int, len(r.lengths))
}
