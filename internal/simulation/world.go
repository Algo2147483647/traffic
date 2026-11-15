package simulation

import (
	"traffic/internal/foundation"
	model "traffic/internal/infrastructure/road"
	rail "traffic/internal/metro_scheduling"
	"traffic/internal/operations/demand"
	road "traffic/internal/road_scheduling"
)

// Simulation owns the shared physical clock and the lifetime of both executors.
// The embedded road API retains the existing wire/command contract.
type Simulation struct {
	*road.Simulation
	clock  *foundation.Clock
	metro  *rail.Runtime
	ledger *demand.Context
}

func newWorld(g *model.Graph, adaptive bool) *Simulation {
	w := &Simulation{clock: &foundation.Clock{Rate: 1}, ledger: &demand.Context{}}
	w.Simulation = road.NewRuntime(g, adaptive, w.clock, w.resetMetro, w.advanceSubstep, w.ledger)
	return w
}
func (w *Simulation) resetMetro(g *model.Graph) road.MetroPort {
	c := rail.DefaultConfig()
	if w.metro != nil {
		c = w.metro.Snapshot().Config
	} else if g.Metro != nil {
		c = g.Metro.Defaults
	}
	if g.Metro != nil {
		w.metro = rail.NewNetwork(g.Metro, c)
	} else {
		w.metro = rail.New(nil, c)
	}
	return w.metro
}
func New(g *model.Graph) *Simulation          { return newWorld(g, true) }
func NewFixedTime(g *model.Graph) *Simulation { return newWorld(g, false) }
func NewWithPolicy(g *model.Graph, id string, p map[string]float64) (*Simulation, error) {
	w := New(g)
	if err := w.ConfigurePolicy(id, p); err != nil {
		w.Close()
		return nil, err
	}
	return w, nil
}

type Command = road.Command
type Snapshot = road.Snapshot
type AlgorithmConfig = road.AlgorithmConfig
type TaxiConfig = road.TaxiConfig
type TaxiView = road.TaxiView

var DefaultTaxiConfig = road.DefaultTaxiConfig
var PolicyCatalog = road.PolicyCatalog

type CityView = road.CityView
