package roadscheduling

import (
	"traffic/internal/foundation"
	model "traffic/internal/infrastructure/road"
	rail "traffic/internal/metro_scheduling"
	"traffic/internal/operations/demand"
)

func newSimulation(g *model.Graph, adaptive bool) *Simulation {
	var m *rail.Runtime
	clock := &foundation.Clock{Rate: 1}
	return NewRuntime(g, adaptive, clock, func(graph *model.Graph) MetroPort {
		c := rail.DefaultConfig()
		if m != nil {
			c = m.Snapshot().Config
		} else if graph.Metro != nil {
			c = graph.Metro.Defaults
		}
		if graph.Metro != nil {
			m = rail.NewNetwork(graph.Metro, c)
		} else {
			m = rail.New(nil, c)
		}
		return m
	}, func(dt float64, p TickPhases) {
		if !p.Prepare() {
			return
		}
		p.DemandAndSignals()
		clock.Time += dt
		p.MotionAndArrivals()
		m.Advance(dt)
		for _, id := range m.DrainCompletedJourneys() {
			p.CompleteJourney(id)
		}
		clock.Tick++
		p.Commit()
	}, &demand.Context{})
}
func New(g *model.Graph) *Simulation          { return newSimulation(g, true) }
func NewFixedTime(g *model.Graph) *Simulation { return newSimulation(g, false) }
func NewWithPolicy(g *model.Graph, id string, p map[string]float64) (*Simulation, error) {
	s := New(g)
	if e := s.ConfigurePolicy(id, p); e != nil {
		return nil, e
	}
	return s, nil
}
