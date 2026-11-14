package sim

import (
	"fmt"
	"math"
	"traffic/internal/foundation"
	model "traffic/internal/infrastructure/road"
)

// Command is the sole public mutation contract. Graph is an immutable host input;
// remote callers switch validated presets through the existing control endpoint.
type Command struct {
	Kind       string       `json:"kind"`
	Population int          `json:"population,omitempty"`
	MaxPeople  int          `json:"max_population,omitempty"`
	Value      float64      `json:"value,omitempty"`
	Mode       string       `json:"mode,omitempty"`
	Taxi       TaxiConfig   `json:"taxi,omitempty"`
	Graph      *model.Graph `json:"-"`
}
type Arrival struct {
	Kind       string
	Way        string
	RouteIndex int
	Task       int
}

func (s *Simulation) bindJournal() { s.journal.Stamp = s.stamp; s.journal.Tick = s.clock.Tick }
func (s *Simulation) QueueCommand(e foundation.Envelope[Command]) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !e.Valid(s.stamp) {
		return 0, fmt.Errorf("stale command revision")
	}
	if e.Payload.Kind == "step" {
		return 0, fmt.Errorf("step is only valid as an immediate command")
	}
	if e.Due < s.clock.Tick {
		return 0, fmt.Errorf("command is in the past")
	}
	if e.Due > s.clock.Tick+1000000 {
		return 0, fmt.Errorf("command horizon exceeded")
	}
	if err := validateCommand(e.Payload); err != nil {
		return 0, err
	}
	e.Payload = copyCommand(e.Payload)
	return s.commands.Push(e), nil
}
func copyCommand(c Command) Command {
	if c.Taxi.Parameters != nil {
		p := map[string]float64{}
		for k, v := range c.Taxi.Parameters {
			p[k] = v
		}
		c.Taxi.Parameters = p
	}
	return c
}
func (s *Simulation) Execute(c Command) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateCommand(c); err != nil {
		return err
	}
	seq := s.commands.Push(foundation.Envelope[Command]{Stamp: s.stamp, Due: s.clock.Tick, Payload: copyCommand(c)})
	return s.drainCommandsLocked()[seq]
}
func validateCommand(c Command) error {
	if math.IsNaN(c.Value) || math.IsInf(c.Value, 0) {
		return fmt.Errorf("command value must be finite")
	}
	switch c.Kind {
	case "pause", "resume", "reset", "set_mode", "set_taxi_load", "step":
	case "set_time_scale":
		if c.Value <= 0 || c.Value > 10 {
			return fmt.Errorf("invalid playback rate")
		}
	case "set_load":
		if c.Population < 1 || c.Population > 10000 || c.MaxPeople < c.Population || c.MaxPeople > 10000 || c.Value < 0 || c.Value > 100 {
			return fmt.Errorf("invalid population load")
		}
	case "set_spacing":
		if c.Value < 2 || c.Value > 200 {
			return fmt.Errorf("invalid clearance")
		}
	case "set_graph":
		if c.Graph == nil {
			return fmt.Errorf("missing graph")
		}
	default:
		return fmt.Errorf("unknown command %q", c.Kind)
	}
	if c.Kind == "reset" && (c.Population < 0 || c.Population > 10000) {
		return fmt.Errorf("invalid population")
	}
	return nil
}
func (s *Simulation) drainCommandsLocked() map[uint64]error {
	results := map[uint64]error{}
	for _, e := range s.commands.Drain(s.clock.Tick) {
		var err error
		if !e.Valid(s.stamp) {
			err = fmt.Errorf("stale command revision")
		} else {
			err = s.applyCommandLocked(e.Payload)
		}
		results[e.Sequence] = err
		if err == nil {
			s.stamp.World++
		}
		s.bindJournal()
		state := fmt.Sprintf("%d:%s:applied", e.Sequence, e.Payload.Kind)
		if err != nil {
			state = fmt.Sprintf("%d:%s:rejected:%s", e.Sequence, e.Payload.Kind, err)
		}
		s.journal.Record(s.clock.Time, "command", int(e.Entity), state)
	}
	return results
}
func (s *Simulation) applyCommandLocked(c Command) error {
	switch c.Kind {
	case "pause":
		s.setPausedLocked(true)
	case "resume":
		s.setPausedLocked(false)
	case "step":
		s.stepLocked(c.Value)
	case "reset":
		s.resetLockedCommand(c.Population)
	case "set_time_scale":
		s.setTimeScaleLocked(c.Value)
	case "set_spacing":
		s.setSpacingLocked(c.Value)
	case "set_load":
		s.setLoadLocked(c.Population, c.Value, c.MaxPeople)
	case "set_graph":
		s.replaceGraphLocked(c.Graph)
	case "set_mode":
		return s.setModeLocked(c.Mode, c.Taxi)
	case "set_taxi_load":
		return s.setTaxiLoadLocked(c.Taxi.Fleet, c.Taxi.Demand)
	}
	return nil
}
func (s *Simulation) queueArrival(p *personState, kind string) {
	task := 0
	if s.taxi != nil {
		for _, v := range s.taxi.vehicles {
			if v.ID == p.id {
				task = v.OrderID
				break
			}
		}
	}
	s.arrivalsQueue.Push(foundation.Envelope[Arrival]{Stamp: s.stamp, Due: s.clock.Tick, Entity: foundation.EntityID(p.id), Payload: Arrival{kind, p.wayID, p.routeIndex, task}})
}
func (s *Simulation) commitArrivalsLocked() {
	people := map[int]*personState{}
	for _, p := range s.people {
		people[p.id] = p
	}
	for _, e := range s.arrivalsQueue.Drain(s.clock.Tick) {
		p := people[int(e.Entity)]
		a := e.Payload
		if !e.Valid(s.stamp) || p == nil || p.wayID != a.Way || p.routeIndex != a.RouteIndex {
			continue
		}
		if s.taxi != nil {
			valid := false
			for _, v := range s.taxi.vehicles {
				if v.ID == p.id && v.OrderID == a.Task {
					valid = true
					break
				}
			}
			if !valid {
				continue
			}
		}
		s.journal.Record(s.clock.Time, "arrival", p.id, a.Kind)
		if a.Kind == "destination" {
			s.exitLane(p)
		} else {
			s.transition(p)
		}
	}
}

func (s *Simulation) ReplaceGraph(g *model.Graph) {
	_ = s.Execute(Command{Kind: "set_graph", Graph: g})
}
func (s *Simulation) Reset(n int)     { _ = s.Execute(Command{Kind: "reset", Population: n}) }
func (s *Simulation) Step(dt float64) { _ = s.Execute(Command{Kind: "step", Value: dt}) }
func (s *Simulation) SetPaused(p bool) {
	kind := "resume"
	if p {
		kind = "pause"
	}
	_ = s.Execute(Command{Kind: kind})
}
func (s *Simulation) SetTimeScale(v float64) {
	_ = s.Execute(Command{Kind: "set_time_scale", Value: v})
}
func (s *Simulation) SetSpacing(v float64) { _ = s.Execute(Command{Kind: "set_spacing", Value: v}) }
func (s *Simulation) SetLoad(n int, r float64, max int) {
	_ = s.Execute(Command{Kind: "set_load", Population: n, MaxPeople: max, Value: r})
}
func (s *Simulation) SetMode(m string, c TaxiConfig) error {
	return s.Execute(Command{Kind: "set_mode", Mode: m, Taxi: c})
}
func (s *Simulation) SetTaxiLoad(n int, r float64) error {
	return s.Execute(Command{Kind: "set_taxi_load", Taxi: TaxiConfig{Fleet: n, Demand: r}})
}
