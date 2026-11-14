package sim

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
	"traffic/internal/dispatch"

	"traffic/internal/operations/demand"
	"traffic/internal/operations/taxi"
)

type TaxiConfig = taxi.Config
type TaxiStop = taxi.Stop
type TaxiOrder = taxi.Order
type TaxiVehicle struct {
	taxi.Vehicle
	p *personState
}

type TaxiView struct {
	Config     TaxiConfig    `json:"config"`
	Stops      []TaxiStop    `json:"stops"`
	Orders     []TaxiOrder   `json:"orders"`
	Vehicles   []TaxiVehicle `json:"vehicles"`
	Created    int           `json:"created"`
	Completed  int           `json:"completed"`
	Cancelled  int           `json:"cancelled"`
	Picked     int           `json:"picked"`
	MeanWait   float64       `json:"mean_wait_s"`
	Failures   int           `json:"failures"`
	LastError  string        `json:"last_error"`
	Solving    bool          `json:"solving"`
	SolverMS   float64       `json:"solver_ms"`
	Candidates int           `json:"candidates"`
}
type taxiPlan struct {
	route         []string
	lanes         []int
	entryLanes    []int
	distance, eta float64
	target        TaxiStop
}
type decisionResult struct {
	proposal dispatch.Proposal
	err      error
	ms       float64
}
type taxiRuntime struct {
	config                                                       TaxiConfig
	stops                                                        []TaxiStop
	vehicles                                                     []*TaxiVehicle
	orders                                                       []*TaxiOrder
	demand, cruise                                               *rand.Rand
	nextArrival, nextDispatch                                    float64
	created, completed, cancelled, picked, failures, consecutive int
	waitSum                                                      float64
	lastError                                                    string
	solverMS                                                     float64
	candidateCount                                               int
	policy                                                       dispatch.Policy
	request                                                      uint64
	pending                                                      chan decisionResult
	cancel                                                       context.CancelFunc
	input                                                        dispatch.Input
	plans                                                        map[dispatch.Pair]taxiPlan
}

func DefaultTaxiConfig() TaxiConfig {
	return TaxiConfig{Fleet: 32, Demand: 2, Algorithm: "nearest", Seed: 71429, MaxWait: 600, Dwell: 20, Interval: 2}
}
func normalizeTaxi(c TaxiConfig) (TaxiConfig, error) {
	if c.Algorithm == "" {
		c.Algorithm = "nearest"
	}
	if c.MaxWait == 0 {
		c.MaxWait = 600
	}
	if c.Dwell == 0 {
		c.Dwell = 20
	}
	if c.Interval == 0 {
		c.Interval = 2
	}
	if c.Seed == 0 {
		c.Seed = 71429
	}
	for _, v := range []float64{c.Demand, c.MaxWait, c.Dwell, c.Interval} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return c, fmt.Errorf("taxi values must be finite")
		}
	}
	if c.Fleet < 1 || c.Fleet > 200 || c.Demand < 0 || c.Demand > 60 || c.MaxWait < 10 || c.MaxWait > 3600 || c.Dwell < .1 || c.Dwell > 60 || c.Interval < 1 || c.Interval > 30 {
		return c, fmt.Errorf("invalid taxi configuration")
	}
	for _, v := range c.Parameters {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e6 {
			return c, fmt.Errorf("invalid algorithm parameter")
		}
	}
	return c, nil
}
func (s *Simulation) setModeLocked(mode string, c TaxiConfig) error {
	if mode != "traffic" && mode != "taxi" && mode != "mixed" {
		return fmt.Errorf("mode must be traffic, taxi or mixed")
	}
	share := 0.0
	if mode == "taxi" {
		share = 1
	}
	if mode == "mixed" {
		share = .5
	}
	rate := 2.0
	if mode != "traffic" {
		var err error
		c, err = normalizeTaxi(c)
		if err != nil {
			return err
		}
		policy, err := dispatch.New(c.Algorithm)
		if err != nil {
			return err
		}
		if len(s.taxiStops()) < 2 {
			return fmt.Errorf("map needs two trip-access roads")
		}
		if s.taxi == nil {
			s.startTaxiLive(c)
		} else {
			if err = s.setTaxiLoadLocked(c.Fleet, c.Demand); err != nil {
				return err
			}
			s.taxi.config = c
			s.taxi.policy = policy
		}
		rate = c.Demand
	}
	if s.demandStream != nil && mode == "traffic" {
		rate = s.demandStream.Config.Rate
	}
	first := s.demandStream == nil
	seed := int64(0)
	if first {
		seed = c.Seed
	}
	if err := s.setDemand(demand.Config{Rate: rate, TaxiShare: share, Seed: seed}); err != nil {
		return err
	}
	if first && mode != "traffic" {
		for i := 0; i < min(c.Fleet, 8); i++ {
			s.demandStream.Add(len(s.taxiStops()))
		}
		s.advanceSharedDemand()
	}
	return nil
}

func (s *Simulation) clearTaxi() {
	if s.taxi != nil && s.taxi.cancel != nil {
		s.taxi.cancel()
	}
	s.taxi = nil
	s.taxiEpoch++
	s.journal.Record(s.clock.Time, "epoch", int(s.taxiEpoch), "invalidated")
}
func (s *Simulation) Close() { s.mu.Lock(); defer s.mu.Unlock(); s.cancelAlgorithms(); s.clearTaxi() }
func (s *Simulation) taxiStops() []TaxiStop {
	out := []TaxiStop{}
	for _, w := range s.graph.SortedWays() {
		if !w.TripAccess || w.Structure != "ground" || w.Role != "" || w.Length < 120 {
			continue
		}
		along := w.Length * .55
		pos, _ := w.Sample(along, w.Lanes-1)
		out = append(out, TaxiStop{ID: "stop:" + w.ID, WayID: w.ID, Along: along, Position: pos})
	}
	return out
}
func (s *Simulation) startTaxi(c TaxiConfig) {
	p, _ := dispatch.New(c.Algorithm)
	t := &taxiRuntime{config: c, stops: s.taxiStops(), policy: p, demand: rand.New(rand.NewSource(c.Seed)), cruise: rand.New(rand.NewSource(c.Seed ^ 0x29a7))}
	s.taxi = t
	s.people = nil
	s.spawnRate = 0
	s.nextID = 1
	for _, stop := range t.stops {
		w := s.graph.Ways[stop.WayID]
		for along := 35.0; along < w.Length-35 && len(t.vehicles) < c.Fleet; along += s.minimumVehicleSpacing() + 15 {
			lane := w.Lanes - 1
			p := &personState{id: s.nextID, state: "waiting", wayID: w.ID, lane: lane, lanePosition: float64(lane), along: along, route: []string{w.ID}, destinationAlong: along}
			p.position, p.heading = w.Sample(along, lane)
			s.nextID++
			v := &TaxiVehicle{Vehicle: taxi.NewVehicle(p.id, s.clock.Time), p: p}
			s.people = append(s.people, p)
			t.vehicles = append(t.vehicles, v)
		}
		if len(t.vehicles) >= c.Fleet {
			break
		}
	}
	// Initial supply is spread over accessible roads, not concentrated in one queue.
	for i, v := range t.vehicles {
		stop := t.stops[i%len(t.stops)]
		w := s.graph.Ways[stop.WayID]
		along := 35 + float64(i/len(t.stops))*(s.minimumVehicleSpacing()+15)
		if along < w.Length-35 && s.laneEntryAvailable(w.ID, w.Lanes-1, along, v.p) {
			v.p.wayID = w.ID
			v.p.lane = w.Lanes - 1
			v.p.lanePosition = float64(v.p.lane)
			v.p.along = along
			v.p.route = []string{w.ID}
			v.p.destinationAlong = along
			v.p.position, v.p.heading = w.Sample(along, v.p.lane)
		}
	}
	t.config.Fleet = len(t.vehicles)
	for i := 0; i < min(c.Fleet, 8); i++ {
		s.createTaxiOrder()
	}
	s.nextTaxiArrival()
}
func (s *Simulation) nextTaxiArrival() {
	if s.demandStream != nil {
		s.taxi.nextArrival = math.Inf(1)
		return
	}
	t := s.taxi
	t.nextArrival = math.Inf(1)
	if t.config.Demand > 0 {
		delay := t.demand.ExpFloat64() * 60 / t.config.Demand
		if s.policies.DemandDelay != nil {
			v := s.policies.DemandDelay(t.config.Demand, delay)
			if v >= .05 && !math.IsInf(v, 0) && !math.IsNaN(v) {
				delay = v
			}
		}
		t.nextArrival = s.clock.Time + delay
	}
}
func (s *Simulation) createTaxiOrder() {
	t := s.taxi
	a := t.demand.Intn(len(t.stops))
	b := t.demand.Intn(len(t.stops) - 1)
	if b >= a {
		b++
	}
	t.created++
	s.journal.Record(s.clock.Time, "order", t.created, "Waiting")
	t.orders = append(t.orders, &TaxiOrder{ID: t.created, Pickup: t.stops[a].ID, Destination: t.stops[b].ID, State: "Waiting", Created: s.clock.Time})
}
func (t *taxiRuntime) stop(id string) TaxiStop {
	for _, s := range t.stops {
		if s.ID == id {
			return s
		}
	}
	return TaxiStop{}
}
func (t *taxiRuntime) order(id int) *TaxiOrder {
	for _, o := range t.orders {
		if o.ID == id {
			return o
		}
	}
	return nil
}
func (s *Simulation) taxiRoute(p *personState, target TaxiStop) (taxiPlan, bool) {
	plan := taxiPlan{target: target}
	if p.wayID == target.WayID && target.Along >= p.along {
		plan.route = []string{p.wayID}
		plan.lanes = []int{p.lane}
		plan.entryLanes = []int{p.lane}
	} else {
		result, ok := s.congestionRoute(p.wayID, target.WayID, "", p.lane, p.along)
		if !ok {
			return plan, false
		}
		plan.route, plan.lanes, plan.entryLanes = result.Arrays()
	}
	for i, id := range plan.route {
		w := s.graph.Ways[id]
		d := w.Length
		if i == 0 {
			d -= p.along
		}
		if i == len(plan.route)-1 {
			d -= w.Length - target.Along
		}
		d = math.Max(0, d)
		plan.distance += d * 1000 / s.graph.WorldUnitsPerKM
		plan.eta += d / w.SpeedLimit
	}
	return plan, len(plan.route) > 0
}
func (s *Simulation) taxiSetPlan(v *TaxiVehicle, plan taxiPlan) {
	p := v.p
	delete(s.reservations, p.id)
	p.route = append([]string(nil), plan.route...)
	p.routeLanes = append([]int(nil), plan.lanes...)
	p.routeEntryLanes = append([]int(nil), plan.entryLanes...)
	p.routeIndex = 0
	p.destinationAlong = plan.target.Along
	p.destinationLane = s.graph.Ways[plan.target.WayID].Lanes - 1
	p.destination = plan.target.Position
	p.origin = p.position
	p.state = "driving"
	p.waitingReason = ""
	p.waitingSignal = ""
	p.waitingFor = 0
}
func (s *Simulation) taxiArrive(p *personState) bool {
	if s.taxi == nil {
		return false
	}
	for _, v := range s.taxi.vehicles {
		if v.p != p {
			continue
		}
		if p.laneChanging || p.lane != p.destinationLane {
			p.speed = 0
			p.waitingReason = "aligning with taxi stop"
			return true
		}
		p.state = "waiting"
		p.speed = 0
		p.waitingReason = "taxi stop"
		p.waitingSignal = ""
		o := s.taxi.order(v.OrderID)
		switch v.State {
		case "ToPickup":
			if o == nil || o.State != "Assigned" {
				v.State = "Idle"
				v.OrderID = 0
				break
			}
			if s.clock.Time-o.Created > s.taxi.config.MaxWait {
				s.cancelTaxiOrder(o)
				break
			}
			if !v.Pickup(o, s.clock.Time, s.taxi.config.Dwell) {
				return false
			}

			s.journal.Record(s.clock.Time, "order", o.ID, "Boarding")

			s.taxi.picked++
			s.taxi.waitSum += s.clock.Time - o.Created
		case "WithPassenger":
			v.Dropoff(s.clock.Time, s.taxi.config.Dwell)
		case "Idle":
			v.MarkIdle(s.clock.Time)
		}
		return true
	}
	return false
}
func (s *Simulation) cancelTaxiOrder(o *TaxiOrder) {
	t := s.taxi
	if !taxi.Cancel(o) {
		return
	}

	s.journal.Record(s.clock.Time, "order", o.ID, "Cancelled")
	t.cancelled++
	s.finishJourney(o.JourneyID, true)
	for _, v := range t.vehicles {
		if v.OrderID == o.ID {
			v.Release(s.clock.Time)
		}
	}
}
func (s *Simulation) taxiMaintenance(dt float64) {
	t := s.taxi
	if t == nil {
		return
	}
	for _, v := range t.vehicles {
		v.Account(dt, v.p.speed*dt)
		o := t.order(v.OrderID)
		if v.DwellReady(s.clock.Time) {
			if v.State == "Alighting" {
				if o != nil {
					if !v.Complete(o, s.clock.Time) {
						continue
					}
					s.journal.Record(s.clock.Time, "order", o.ID, "Completed")

					t.completed++
					s.finishJourney(o.JourneyID, false)
				}
				v.Release(s.clock.Time)
			} else if o != nil {
				if plan, ok := s.taxiRoute(v.p, t.stop(o.Destination)); ok {
					if !v.Depart(o, s.clock.Time) {
						continue
					}
					s.journal.Record(s.clock.Time, "order", o.ID, "InService")

					s.taxiSetPlan(v, plan)
				}
			}
		}
	}
	for _, o := range t.orders {
		if (o.State == "Waiting" || o.State == "Assigned") && s.clock.Time-o.Created > t.config.MaxWait {
			s.cancelTaxiOrder(o)
		}
	}
	if s.demandStream == nil && s.clock.Time >= t.nextArrival {
		s.createTaxiOrder()
		s.nextTaxiArrival()
	}
	s.retireTaxiVehicles()
	if len(t.vehicles) < t.config.Fleet && s.clock.Tick%40 == 0 {
		s.addTaxiVehicles(t.config.Fleet)
	}
	if len(t.orders) > 1000 {
		keep := t.orders[:0]
		for _, o := range t.orders {
			if o.State != "Completed" && o.State != "Cancelled" || o.ID > t.created-500 {
				keep = append(keep, o)
			}
		}
		t.orders = keep
	}
}

// Dispatch is computed from detached inputs; pending work holds simulation time, not the mutex or UI.
func (s *Simulation) taxiDispatch() bool {
	t := s.taxi
	if t == nil {
		return true
	}
	if t.pending != nil {
		select {
		case result := <-t.pending:
			t.pending = nil
			t.cancel()
			t.cancel = nil
			s.commitTaxiDecision(result)
			return true
		default:
			return false
		}
	}
	if s.clock.Time < t.nextDispatch {
		return true
	}
	interval := t.config.Interval
	if t.config.Algorithm == "batch" {
		interval = math.Max(interval, 5)
	}
	t.nextDispatch = s.clock.Time + interval
	t.request++
	in := dispatch.Input{Version: 1, Epoch: s.taxiEpoch, Request: t.request, Time: s.clock.Time, Seed: t.config.Seed, Vehicles: []dispatch.Vehicle{}, Orders: []dispatch.Order{}, Candidates: []dispatch.Candidate{}, Parameters: t.config.Parameters}
	t.plans = map[dispatch.Pair]taxiPlan{}
	for _, o := range t.orders {
		if o.State == "Waiting" && len(in.Orders) < 64 {
			in.Orders = append(in.Orders, dispatch.Order{ID: o.ID, Created: o.Created, Deadline: o.Created + t.config.MaxWait})
		}
	}
	for _, v := range t.vehicles {
		if !taxi.Eligible(v.State, v.Retiring, v.p.state == "turning", v.p.laneChanging) {
			continue
		}
		in.Vehicles = append(in.Vehicles, dispatch.Vehicle{ID: v.ID, Idle: v.IdleTime(s.clock.Time), Income: v.Income})
		for _, o := range in.Orders {
			order := t.order(o.ID)
			if plan, ok := s.taxiRoute(v.p, t.stop(order.Pickup)); ok {
				key := dispatch.Pair{VehicleID: v.ID, OrderID: o.ID}
				t.plans[key] = plan
				in.Candidates = append(in.Candidates, dispatch.Candidate{VehicleID: v.ID, OrderID: o.ID, ETA: plan.eta, Distance: plan.distance})
			}
		}
	}
	t.input = in
	t.candidateCount = len(in.Candidates)
	if len(in.Candidates) == 0 {
		s.taxiCruise()
		return true
	}
	if metaIsProcess(t.config.Algorithm) {
		ctx, cancel := context.WithCancel(context.Background())
		t.cancel = cancel
		t.pending = make(chan decisionResult, 1)
		ch, policy := t.pending, t.policy
		go func() {
			start := time.Now()
			p, e := dispatch.Invoke(ctx, policy, in)
			ch <- decisionResult{p, e, float64(time.Since(start).Microseconds()) / 1000}
		}()
		return false
	}
	start := time.Now()
	p, e := dispatch.Invoke(context.Background(), t.policy, in)
	s.commitTaxiDecision(decisionResult{p, e, float64(time.Since(start).Microseconds()) / 1000})
	return true
}
func metaIsProcess(id string) bool {
	for _, m := range dispatch.List() {
		if m.ID == id {
			return m.Kind == "process"
		}
	}
	return false
}
func (s *Simulation) commitTaxiDecision(r decisionResult) {
	t := s.taxi
	t.solverMS = r.ms
	if r.err == nil {
		r.err = dispatch.Validate(t.input, r.proposal)
	}
	if r.err != nil {
		t.failures++
		t.consecutive++
		t.lastError = r.err.Error()
		if t.consecutive >= 3 {
			s.clock.Paused = true
		}
		return
	}
	t.lastError = ""
	t.consecutive = 0
	for _, pair := range r.proposal.Assignments {
		var v *TaxiVehicle
		for _, x := range t.vehicles {
			if x.ID == pair.VehicleID {
				v = x
			}
		}
		o := t.order(pair.OrderID)
		if v == nil || o == nil || v.Retiring || v.OrderID != 0 || !taxi.CanAssign(v.State, o.State) {
			t.lastError = "stale proposal rejected"
			t.failures++
			return
		}
	}
	for _, pair := range r.proposal.Assignments {
		for _, v := range t.vehicles {
			if v.ID == pair.VehicleID {
				o := t.order(pair.OrderID)
				plan := t.plans[pair]
				if !v.Assign(o, s.clock.Time, plan.eta) {
					continue
				}
				s.journal.Record(s.clock.Time, "order", o.ID, "Assigned")
				s.taxiSetPlan(v, plan)
			}
		}
	}
	s.taxiCruise()
}
func (s *Simulation) taxiCruise() {
	t := s.taxi
	for _, v := range t.vehicles {
		if !v.Retiring && v.State == "Idle" && v.p.state != "turning" && v.p.along >= v.p.destinationAlong-.001 && v.p.routeIndex == len(v.p.route)-1 {
			target := t.stops[t.cruise.Intn(len(t.stops))]
			if s.policies.TaxiCruise != nil {
				id := s.policies.TaxiCruise(v.ID, append([]TaxiStop(nil), t.stops...), target.ID)
				if candidate := t.stop(id); candidate.ID != "" {
					target = candidate
				}
			}
			if plan, ok := s.taxiRoute(v.p, target); ok {
				s.taxiSetPlan(v, plan)
			}
		}
	}
}
func (s *Simulation) taxiView() *TaxiView {
	t := s.taxi
	if t == nil {
		return nil
	}
	view := &TaxiView{Config: copyCommand(Command{Taxi: t.config}).Taxi, Stops: append([]TaxiStop(nil), t.stops...), Orders: []TaxiOrder{}, Vehicles: []TaxiVehicle{}, Created: t.created, Completed: t.completed, Cancelled: t.cancelled, Picked: t.picked, Failures: t.failures, LastError: t.lastError, Solving: t.pending != nil, SolverMS: t.solverMS, Candidates: t.candidateCount}
	if t.picked > 0 {
		view.MeanWait = t.waitSum / float64(t.picked)
	}
	for _, o := range t.orders {
		view.Orders = append(view.Orders, *o)
	}
	for _, v := range t.vehicles {
		view.Vehicles = append(view.Vehicles, *v)
	}
	sort.Slice(view.Vehicles, func(i, j int) bool { return view.Vehicles[i].ID < view.Vehicles[j].ID })
	return view
}

// SetTaxiLoad changes supply/demand without resetting time, trips or active orders.
func (s *Simulation) setTaxiLoadLocked(fleet int, demand float64) error {
	t := s.taxi
	if t == nil {
		return fmt.Errorf("switch to Taxi mode first")
	}
	c := t.config
	c.Fleet = fleet
	c.Demand = demand
	if _, err := normalizeTaxi(c); err != nil {
		return err
	}
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	t.pending = nil
	s.taxiEpoch++
	s.journal.Record(s.clock.Time, "epoch", int(s.taxiEpoch), "invalidated")
	rateChanged := c.Demand != t.config.Demand
	t.config = c
	t.nextDispatch = s.clock.Time
	for i, v := range t.vehicles {
		v.Retiring = i >= fleet
	}
	s.retireTaxiVehicles()
	s.addTaxiVehicles(fleet)
	if rateChanged {
		if s.demandStream != nil {
			c := s.demandStream.Config
			c.Rate = demand
			s.demandStream.Update(c, s.clock.Time)
		}
		s.nextTaxiArrival()
	}
	return nil
}
func (s *Simulation) retireTaxiVehicles() {
	t := s.taxi
	removed := map[int]bool{}
	keep := t.vehicles[:0]
	for _, v := range t.vehicles {
		p := v.p
		atStop := p.state != "turning" && !p.laneChanging && p.routeIndex == len(p.route)-1 && p.along >= p.destinationAlong-.001
		if v.Retiring && v.State == "Idle" && atStop {
			removed[v.ID] = true
			continue
		}
		keep = append(keep, v)
	}
	t.vehicles = keep
	if len(removed) > 0 {
		people := s.people[:0]
		for _, p := range s.people {
			if !removed[p.id] {
				people = append(people, p)
			}
		}
		s.people = people
	}
}
func (s *Simulation) addTaxiVehicles(target int) {
	t := s.taxi
	// Add vehicles only where physical road space is available; retry later if full.
	for offset := 0; len(t.vehicles) < target; offset++ {
		added := false
		for i := 0; i < len(t.stops) && len(t.vehicles) < target; i++ {
			stop := t.stops[(i+offset)%len(t.stops)]
			w := s.graph.Ways[stop.WayID]
			lane := w.Lanes - 1
			for along := 35.0; along < w.Length-35; along += s.minimumVehicleSpacing() + 15 {
				if !s.laneEntryAvailable(w.ID, lane, along, nil) {
					continue
				}
				p := &personState{id: s.nextID, state: "waiting", wayID: w.ID, lane: lane, lanePosition: float64(lane), along: along, route: []string{w.ID}, destinationAlong: along, destinationLane: lane}
				p.position, p.heading = w.Sample(along, lane)
				s.nextID++
				s.people = append(s.people, p)
				t.vehicles = append(t.vehicles, &TaxiVehicle{Vehicle: taxi.NewVehicle(p.id, s.clock.Time), p: p})
				added = true
				break
			}
		}
		if !added {
			break
		}
	}
}

func (s *Simulation) setTaxiConfigLocked(c TaxiConfig) error {
	if s.taxi == nil {
		return fmt.Errorf("enable taxi demand first")
	}
	c, err := normalizeTaxi(c)
	if err != nil {
		return err
	}
	policy, err := dispatch.New(c.Algorithm)
	if err != nil {
		return err
	}
	if err = s.setTaxiLoadLocked(c.Fleet, c.Demand); err != nil {
		return err
	}
	s.taxi.config = c
	s.taxi.policy = policy
	return nil
}
