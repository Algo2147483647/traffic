package sim

import (
	"fmt"
	"math"
	"math/rand"
	"traffic/internal/dispatch"
	"traffic/internal/operations/demand"
)

func (s *Simulation) setDemand(c demand.Config) error {
	if math.IsNaN(c.Rate) || math.IsInf(c.Rate, 0) || c.Rate < 0 || c.Rate > 60 || math.IsNaN(c.TaxiShare) || c.TaxiShare < 0 || c.TaxiShare > 1 {
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
		if s.demandStream != nil {
			c.Seed = s.demandStream.Config.Seed
		} else {
			c.Seed = s.graph.Simulation.Seed
		}
	}
	if s.demandStream == nil {
		s.demandStream = demand.New(c, s.clock.Time)
	} else {
		s.demandStream.Update(c, s.clock.Time)
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
	d := s.demandStream
	if d == nil {
		return
	}
	stops := s.taxiStops()
	if len(stops) < 2 {
		return
	}
	d.Generate(s.clock.Time, len(stops))
	for _, j := range d.Journeys {
		if j.State != "queued" {
			continue
		}
		if !j.Prepared {
			j.DepartAt = s.clock.Time
			if j.Mode == "metro" {
				pairs := s.metro.ServedPairs()
				if len(pairs) == 0 {
					continue
				}
				pair := pairs[d.Choose(len(pairs))]
				j.Origin, j.Destination = pair[0], pair[1]
			}
			j.Prepared = true
		}
		if c := s.algorithm.view.Config.Demand; c.ID != "" && c.ID != "native" && !j.Ready {
			continue
		}
		if j.DepartAt > s.clock.Time {
			continue
		}
		if j.Mode == "metro" {
			pairs := s.metro.ServedPairs()
			if len(pairs) == 0 {
				continue
			}
			pair := [2]int{j.Origin, j.Destination}
			if s.metro.AddJourney(pair[0], pair[1], j.ID) {
				j.Origin = pair[0]
				j.Destination = pair[1]
				j.Entity = j.ID
				j.State = "active"
			}
			continue
		}
		from, to := stops[j.Origin%len(stops)], stops[j.Destination%len(stops)]
		if j.Mode == "taxi" {
			if s.taxi == nil {
				continue
			}
			t := s.taxi
			t.created++
			o := &TaxiOrder{ID: t.created, JourneyID: j.ID, Pickup: from.ID, Destination: to.ID, State: "Waiting", Created: s.clock.Time}
			t.orders = append(t.orders, o)
			j.State = "active"
			j.Entity = o.ID
			s.journal.Record(s.clock.Time, "order", o.ID, "Waiting")
			continue
		}
		if len(s.people) >= s.maxPeople {
			continue
		}
		w := s.graph.Ways[from.WayID]
		lane := w.Lanes - 1
		if !s.laneEntryAvailable(w.ID, lane, from.Along, nil) {
			continue
		}
		p := &personState{id: s.nextID, journeyID: j.ID, state: "driving", wayID: w.ID, lane: lane, lanePosition: float64(lane), along: from.Along}
		plan, ok := s.taxiRoute(p, to)
		if !ok {
			continue
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
	}
	d.Prune()
}
func (s *Simulation) finishJourney(id int, cancelled bool) {
	if id > 0 && s.demandStream != nil {
		s.demandStream.Finish(id, cancelled)
	}
}
func (s *Simulation) removeFinishedPrivateCars() {
	if s.demandStream == nil {
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
	summary := s.demandStream.Summary()
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
	for _, j := range s.demandStream.Journeys {
		expected := 0
		if j.State == "active" {
			expected = 1
		}
		if linked[j.ID] != expected {
			summary.Errors = append(summary.Errors, fmt.Sprintf("journey %d has %d executor tasks, expected %d", j.ID, linked[j.ID], expected))
		}
		delete(linked, j.ID)
	}
	if len(linked) > 0 {
		summary.Errors = append(summary.Errors, "orphan executor tasks")
	}
	summary.Conserved = summary.Conserved && len(summary.Errors) == 0
	return summary
}
