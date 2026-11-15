package roadscheduling

import (
	"fmt"
	"math"
	"math/rand"
	"traffic/internal/operations/demand"
	"traffic/internal/operations/taxi/dispatch"
)

func (s *Simulation) setDemand(c demand.Config) error {
	if math.IsNaN(c.Rate) || math.IsInf(c.Rate, 0) || c.Rate < 0 || math.IsNaN(c.TaxiShare) || c.TaxiShare < 0 || c.TaxiShare > 1 {
		return fmt.Errorf("invalid shared demand")
	}
	if math.IsNaN(c.MetroShare) || math.IsInf(c.MetroShare, 0) || c.MetroShare < 0 || c.MetroShare+c.TaxiShare > 1 {
		return fmt.Errorf("invalid mode shares")
	}
	if c.MetroShare > 0 && len(s.metro.ServedPairs()) == 0 {
		return fmt.Errorf("map has no served metro journeys")
	}
	if len(s.taxiStops()) < 2 {
		return fmt.Errorf("map needs two trip-access roads")
	}
	if c.Seed == 0 {
		if s.demandContext.Stream != nil {
			c.Seed = s.demandContext.Stream.Config.Seed
		} else {
			c.Seed = s.graph.Simulation.Seed
		}
	}
	if s.demandContext.Stream == nil {
		s.demandContext.Stream = demand.New(c, s.clock.Time)
	} else {
		s.demandContext.Stream.Update(c, s.clock.Time)
	}
	if s.taxi == nil && c.TaxiShare > 0 {
		s.startTaxiLive(DefaultTaxiConfig())
	}
	s.metro.UseSharedDemand()
	s.mode = "mixed"
	if c.TaxiShare == 0 && c.MetroShare == 0 {
		s.mode = "traffic"
	}
	if c.TaxiShare == 1 {
		s.mode = "taxi"
	}
	s.spawnRate = 0
	if s.taxi != nil {
		s.taxi.nextArrival = math.Inf(1)
		s.taxi.config.Demand = c.Rate
	}
	return nil
}
func (s *Simulation) startTaxiLive(c TaxiConfig) {
	policy, _ := dispatch.New(c.Algorithm)
	s.taxi = &taxiRuntime{config: c, stops: s.taxiStops(), policy: policy, demand: rand.New(rand.NewSource(c.Seed)), cruise: rand.New(rand.NewSource(c.Seed ^ 0x29a7)), nextArrival: math.Inf(1)}
	s.addTaxiVehicles(c.Fleet)
}
func (s *Simulation) advanceSharedDemand() {
	d := s.demandContext.Stream
	if d == nil {
		return
	}
	stops := s.taxiStops()
	if len(stops) < 2 {
		return
	}
	d.Dispatch(s.clock.Time, len(stops), external(s.algorithm.view.Config.Demand), s.metro.ServedPairs(), s.metro.AddJourney, func(j *demand.Journey) bool {
		from, to := stops[j.Origin%len(stops)], stops[j.Destination%len(stops)]
		if j.Mode == "taxi" {
			if s.taxi == nil {
				return false
			}
			t := s.taxi
			t.created++
			o := &TaxiOrder{ID: t.created, JourneyID: j.ID, Pickup: from.ID, Destination: to.ID, State: "Waiting", Created: s.clock.Time}
			t.orders = append(t.orders, o)
			j.State = "active"
			j.Entity = o.ID
			s.journal.Record(s.clock.Time, "order", o.ID, "Waiting")
			return true
		}
		if len(s.people) >= s.maxPeople {
			return false
		}
		w := s.graph.Ways[from.WayID]
		lane := w.Lanes - 1
		if !s.laneEntryAvailable(w.ID, lane, from.Along, nil) {
			return false
		}
		p := &personState{id: s.nextID, journeyID: j.ID, state: "driving", wayID: w.ID, lane: lane, lanePosition: float64(lane), along: from.Along}
		plan, ok := s.taxiRoute(p, to)
		if !ok {
			return false
		}
		s.nextID++
		p.route, p.routeLanes, p.routeEntryLanes = plan.route, plan.lanes, plan.entryLanes
		p.destinationAlong = to.Along
		p.destinationLane = s.graph.Ways[to.WayID].Lanes - 1
		p.origin = from.Position
		p.destination = to.Position
		p.position, p.heading = w.SampleLane(p.along, p.lanePosition)
		s.people = append(s.people, p)
		j.Entity = p.id
		j.State = "active"
		return true
	})
}
func (s *Simulation) finishJourney(id int, cancelled bool) {
	if id > 0 && s.demandContext.Stream != nil {
		s.demandContext.Stream.Finish(id, cancelled)
	}
}
func (s *Simulation) removeFinishedPrivateCars() {
	if s.demandContext.Stream == nil {
		return
	}
	keep := s.people[:0]
	for _, p := range s.people {
		if p.state != "arrived" {
			keep = append(keep, p)
		}
	}
	s.people = keep
}

// auditDemand verifies the operational ledger against real executors, not just totals.
func (s *Simulation) auditDemand() demand.Summary {
	linked := map[int]int{}
	for _, p := range s.people {
		if p.journeyID > 0 && p.state != "arrived" {
			linked[p.journeyID]++
		}
	}
	if s.taxi != nil {
		for _, o := range s.taxi.orders {
			if o.JourneyID > 0 && o.State != "Completed" && o.State != "Cancelled" {
				linked[o.JourneyID]++
			}
		}
	}
	for id, n := range s.metro.ActiveJourneys() {
		linked[id] += n
	}
	return s.demandContext.Stream.Audit(linked)
}
