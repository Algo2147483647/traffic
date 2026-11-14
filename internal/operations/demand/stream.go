package demand

import (
	"math"
	"math/rand"
)

type Config struct {
	Rate       float64 `json:"trips_per_minute"`
	MetroShare float64 `json:"metro_share"`
	TaxiShare  float64 `json:"taxi_share"`
	Seed       int64   `json:"seed"`
}
type Journey struct {
	DepartAt    float64 `json:"depart_at_s"`
	Prepared    bool    `json:"-"`
	Ready       bool    `json:"-"`
	ID          int     `json:"id"`
	Mode        string  `json:"mode"`
	Origin      int     `json:"origin"`
	Destination int     `json:"destination"`
	State       string  `json:"state"`
	Entity      int     `json:"entity_id"`
}
type ModeSummary struct {
	Generated int `json:"generated"`
	Queued    int `json:"queued"`
	Active    int `json:"active"`
	Completed int `json:"completed"`
	Cancelled int `json:"cancelled"`
}
type Summary struct {
	Modes     map[string]ModeSummary `json:"modes"`
	Conserved bool                   `json:"conserved"`
	Errors    []string               `json:"errors"`
	Config    Config                 `json:"config"`
	Generated int                    `json:"generated"`
	Queued    int                    `json:"queued"`
	Active    int                    `json:"active"`
	Completed int                    `json:"completed"`
	Cancelled int                    `json:"cancelled"`
	Cars      int                    `json:"car_trips"`
	Metro     int                    `json:"metro_trips"`
	Taxis     int                    `json:"taxi_trips"`
}
type Stream struct {
	finished                                            map[string]ModeSummary
	Config                                              Config
	Journeys                                            []*Journey
	rng                                                 *rand.Rand
	next                                                float64
	generated, completed, cancelled, cars, taxis, metro int
}

func New(c Config, now float64) *Stream {
	s := &Stream{finished: map[string]ModeSummary{}, Config: c, rng: rand.New(rand.NewSource(c.Seed))}
	s.schedule(now)
	return s
}
func (s *Stream) schedule(now float64) {
	s.next = math.Inf(1)
	if s.Config.Rate > 0 {
		s.next = now + s.rng.ExpFloat64()*60/s.Config.Rate
	}
}
func (s *Stream) Update(c Config, now float64) {
	changed := c.Rate != s.Config.Rate || c.Seed != s.Config.Seed
	if c.Seed != s.Config.Seed {
		s.rng = rand.New(rand.NewSource(c.Seed))
	}
	s.Config = c
	if changed {
		s.schedule(now)
	}
}
func (s *Stream) Generate(now float64, stops int) {
	if stops < 2 {
		return
	}
	for now >= s.next && len(s.Journeys) < 10000 {
		s.Add(stops)
		s.schedule(s.next)
	}
}
func (s *Stream) Add(stops int) *Journey {
	a := s.rng.Intn(stops)
	b := s.rng.Intn(stops - 1)
	if b >= a {
		b++
	}
	mode := "car"
	draw := s.rng.Float64()
	if draw < s.Config.MetroShare {
		mode = "metro"
		s.metro++
	} else if draw < s.Config.MetroShare+s.Config.TaxiShare {
		mode = "taxi"
		s.taxis++
	} else {
		s.cars++
	}
	s.generated++
	j := &Journey{ID: s.generated, Mode: mode, Origin: a, Destination: b, State: "queued"}
	s.Journeys = append(s.Journeys, j)
	return j
}
func (s *Stream) Finish(id int, cancelled bool) {
	for _, j := range s.Journeys {
		if j.ID == id && j.State == "active" {
			totals := s.finished[j.Mode]
			if cancelled {
				totals.Cancelled++
				j.State = "cancelled"
				s.cancelled++
			} else {
				totals.Completed++
				j.State = "completed"
				s.completed++
			}
			s.finished[j.Mode] = totals
			return
		}
	}
}
func (s *Stream) Prune() {
	keep := s.Journeys[:0]
	for _, j := range s.Journeys {
		if j.State == "queued" || j.State == "active" {
			keep = append(keep, j)
		}
	}
	s.Journeys = keep
}
func (s *Stream) Summary() Summary {
	r := Summary{Config: s.Config, Generated: s.generated, Completed: s.completed, Cancelled: s.cancelled, Cars: s.cars, Taxis: s.taxis, Metro: s.metro}
	r.Modes = map[string]ModeSummary{}
	for _, mode := range []string{"car", "taxi", "metro"} {
		r.Modes[mode] = s.finished[mode]
	}
	for _, j := range s.Journeys {
		m := r.Modes[j.Mode]
		if j.State == "queued" {
			m.Queued++
			r.Queued++
		}
		if j.State == "active" {
			m.Active++
			r.Active++
		}
		r.Modes[j.Mode] = m
	}
	for mode, m := range r.Modes {
		m.Generated = m.Queued + m.Active + m.Completed + m.Cancelled
		r.Modes[mode] = m
	}
	r.Conserved = r.Generated == r.Queued+r.Active+r.Completed+r.Cancelled && r.Generated == r.Cars+r.Taxis+r.Metro
	for mode, total := range map[string]int{"car": r.Cars, "taxi": r.Taxis, "metro": r.Metro} {
		r.Conserved = r.Conserved && r.Modes[mode].Generated == total
	}
	r.Errors = []string{}
	return r
}

func (s *Stream) Choose(n int) int { return s.rng.Intn(n) }
