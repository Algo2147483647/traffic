package app

import (
	"traffic/internal/algorithms/experiments"
	model "traffic/internal/infrastructure/road"
	sim "traffic/internal/simulation"
)

type taxiExperiment struct{ world *sim.Simulation }

func NewTaxiExperiment(g *model.Graph, c sim.TaxiConfig) (experiments.Session[*sim.TaxiView], error) {
	s := sim.New(g)
	if err := s.SetMode("taxi", c); err != nil {
		s.Close()
		return nil, err
	}
	return &taxiExperiment{s}, nil
}
func (s *taxiExperiment) Step(dt float64) { s.world.Step(dt) }
func (s *taxiExperiment) Close()          { s.world.Close() }
func (s *taxiExperiment) Observe() experiments.Observation[*sim.TaxiView] {
	v := s.world.Snapshot()
	return experiments.Observation[*sim.TaxiView]{Time: v.Time, Value: v.Taxi, Solving: v.Taxi.Solving, Error: v.Taxi.LastError}
}
