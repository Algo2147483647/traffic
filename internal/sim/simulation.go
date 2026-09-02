package sim

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"traffic/internal/model"
)

type Person struct {
	ID            int           `json:"id"`
	State         string        `json:"state"`
	Position      model.Vec2    `json:"position"`
	Speed         float64       `json:"speed"`
	Heading       float64       `json:"heading_degrees"`
	WayID         string        `json:"way_id,omitempty"`
	Lane          int           `json:"lane"`
	LanePosition  float64       `json:"lane_position"`
	LaneChange    *LaneChange   `json:"lane_change,omitempty"`
	Along         float64       `json:"along"`
	Transform     model.Matrix3 `json:"transform"`
	Reference     Reference     `json:"reference"`
	WaitingFor    int           `json:"waiting_for,omitempty"`
	WaitingSignal string        `json:"waiting_signal,omitempty"`
	WaitingReason string        `json:"waiting_reason,omitempty"`
	Origin        model.Vec2    `json:"origin"`
	Destination   model.Vec2    `json:"destination"`
	Route         []string      `json:"route"`
	RouteIndex    int           `json:"route_index"`
	Trip          int           `json:"trip"`
}

type LaneChange struct {
	FromLane int     `json:"from_lane"`
	ToLane   int     `json:"to_lane"`
	Progress float64 `json:"progress"`
	Reason   string  `json:"reason"`
}

type Reference struct {
	IntersectionID        string        `json:"intersection_id"`
	IntersectionTransform model.Matrix3 `json:"intersection_transform"`
	LocalTransform        model.Matrix3 `json:"local_transform"`
}

type Stats struct {
	Walking      int     `json:"walking"`
	Driving      int     `json:"driving"`
	Waiting      int     `json:"waiting"`
	Arrived      int     `json:"arrived"`
	AverageSpeed float64 `json:"average_speed"`
}

type Snapshot struct {
	City        string             `json:"city"`
	Time        float64            `json:"time"`
	Tick        uint64             `json:"tick"`
	Paused      bool               `json:"paused"`
	TimeScale   float64            `json:"time_scale"`
	People      []Person           `json:"people"`
	Stats       Stats              `json:"stats"`
	Load        LoadState          `json:"load"`
	Spacing     SpacingState       `json:"spacing"`
	Signals     []SignalState      `json:"signals"`
	Performance ControlPerformance `json:"control_performance"`
}

// ControlPerformance exposes cumulative measures that make fixed-time and
// adaptive controllers directly comparable under the same seeded demand.
type ControlPerformance struct {
	Algorithm           string  `json:"algorithm"`
	VehicleDelaySeconds float64 `json:"vehicle_delay_seconds"`
	SignalDelaySeconds  float64 `json:"signal_delay_seconds"`
	MeanSignalDelay     float64 `json:"mean_signal_delay_seconds"`
	SignalStops         int     `json:"signal_stops"`
	SignalReleases      int     `json:"signal_releases"`
	CompletedTrips      int     `json:"completed_trips"`
	ThroughputPerMinute float64 `json:"throughput_per_minute"`
	CurrentSignalQueue  int     `json:"current_signal_queue"`
	MaximumSignalQueue  int     `json:"maximum_signal_queue"`
}

type LoadState struct {
	Population    int     `json:"population"`
	SpawnRate     float64 `json:"spawn_rate_per_second"`
	MaxPopulation int     `json:"max_population"`
}

type SpacingState struct {
	SafeGap     float64 `json:"safe_gap"`
	VehicleSize float64 `json:"vehicle_size"`
}

type Movement struct {
	FromWayID       string                       `json:"from_way_id"`
	ToWayID         string                       `json:"to_way_id"`
	FromLanes       []int                        `json:"from_lanes"`
	ToLanes         []int                        `json:"to_lanes"`
	LaneConnections []model.LaneConnectionConfig `json:"lane_connections"`
	Turn            string                       `json:"turn"`
	DemandScore     float64                      `json:"demand_score"`
}

type SignalState struct {
	IntersectionID    string          `json:"intersection_id"`
	PhaseID           string          `json:"phase_id"`
	PhaseName         string          `json:"phase_name"`
	Stage             string          `json:"stage"`
	PhaseIndex        int             `json:"phase_index"`
	Remaining         float64         `json:"remaining_seconds"`
	CycleSeconds      float64         `json:"cycle_seconds"`
	PhaseElapsed      float64         `json:"phase_elapsed_seconds"`
	NextPhaseID       string          `json:"next_phase_id"`
	NextPhaseName     string          `json:"next_phase_name"`
	AllowedIncoming   []string        `json:"allowed_incoming_way_ids"`
	AllowedMovements  []Movement      `json:"allowed_movements"`
	Controller        string          `json:"controller"`
	Decision          string          `json:"decision"`
	BaseGreen         float64         `json:"base_green_seconds"`
	TargetGreen       float64         `json:"target_green_seconds"`
	MinGreen          float64         `json:"min_green_seconds"`
	MaxGreen          float64         `json:"max_green_seconds"`
	QueuePressure     float64         `json:"queue_pressure"`
	CompetingPressure float64         `json:"competing_pressure"`
	IncomingQueue     float64         `json:"incoming_queue"`
	OutgoingQueue     float64         `json:"outgoing_queue"`
	PhasePressures    []PhasePressure `json:"phase_pressures"`
	ExtensionSeconds  float64         `json:"extension_seconds"`
}

type CityView struct {
	Name          string                    `json:"name"`
	Description   string                    `json:"description"`
	DrivingSide   string                    `json:"driving_side"`
	MovementRules model.MovementRulesConfig `json:"movement_rules"`
	Bounds        model.Bounds              `json:"bounds"`
	Simulation    model.SimulationConfig    `json:"simulation"`
	Intersections []*model.Intersection     `json:"intersections"`
	Ways          []*model.Way              `json:"ways"`
	Edges         []model.Edge              `json:"edges"`
}

type personState struct {
	id                 int
	state              string
	position           model.Vec2
	walkTarget         model.Vec2
	origin             model.Vec2
	destination        model.Vec2
	exitPoint          model.Vec2
	destinationAlong   float64
	destinationLane    int
	route              []string
	routeLanes         []int
	routeIndex         int
	wayID              string
	lane               int
	lanePosition       float64
	laneChanging       bool
	laneChangeFrom     int
	laneChangeTo       int
	laneChangeProgress float64
	laneChangeDuration float64
	laneChangeReason   string
	lastLaneChangeAt   float64
	along              float64
	speed              float64
	heading            model.Vec2
	waitingFor         int
	waitingSignal      string
	waitingReason      string
	signalWait         float64
	arrivedAt          float64
	trip               int
	turnStart          model.Vec2
	turnControl1       model.Vec2
	turnControl2       model.Vec2
	turnEnd            model.Vec2
	turnProgress       float64
	turnLength         float64
	nextWayID          string
	nextLane           int
	nextAlong          float64
}

type Simulation struct {
	mu             sync.RWMutex
	graph          *model.Graph
	people         []*personState
	time           float64
	tick           uint64
	paused         bool
	timeScale      float64
	nextID         int
	spawnRate      float64
	maxPeople      int
	safeGap        float64
	spawnRemainder float64
	adaptive       bool
	signals        map[string]*signalRuntime
	vehicleDelay   float64
	signalDelay    float64
	releasedDelay  float64
	signalStops    int
	signalReleases int
	completedTrips int
	maxSignalQueue int
}

func New(graph *model.Graph) *Simulation {
	return newSimulation(graph, true)
}

// NewFixedTime runs the same optimized design plan without actuation. It is
// intended for reproducible A/B evaluation of the queue-responsive controller.
func NewFixedTime(graph *model.Graph) *Simulation {
	return newSimulation(graph, false)
}

func newSimulation(graph *model.Graph, adaptive bool) *Simulation {
	s := &Simulation{graph: graph, timeScale: 1,
		spawnRate: graph.Simulation.SpawnRate, maxPeople: graph.Simulation.MaxPeople, safeGap: graph.Simulation.SafeGap, adaptive: adaptive}
	s.resetLocked(graph.Simulation.People)
	return s
}

func (s *Simulation) City() CityView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cityLocked()
}

func (s *Simulation) cityLocked() CityView {
	return CityView{
		Name: s.graph.Name, Description: s.graph.Description, DrivingSide: s.graph.DrivingSide, MovementRules: s.graph.MovementRules, Bounds: s.graph.Bounds,
		Simulation: s.graph.Simulation, Intersections: s.graph.SortedIntersections(),
		Ways: s.graph.SortedWays(), Edges: append([]model.Edge(nil), s.graph.Edges...),
	}
}

// ReplaceGraph atomically switches the running simulation to another validated
// city graph and restores that preset's initial load and control settings.
func (s *Simulation) ReplaceGraph(graph *model.Graph) {
	if graph == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.graph = graph
	s.paused = false
	s.timeScale = 1
	s.spawnRate = graph.Simulation.SpawnRate
	s.maxPeople = graph.Simulation.MaxPeople
	s.safeGap = graph.Simulation.SafeGap
	s.resetLocked(graph.Simulation.People)
}

func (s *Simulation) Run(ctx context.Context) {
	s.mu.RLock()
	tickMS := s.graph.Simulation.TickMS
	s.mu.RUnlock()
	ticker := time.NewTicker(time.Duration(tickMS) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Tick(float64(tickMS) / 1000)
		}
	}
}

func (s *Simulation) Tick(dt float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paused || dt <= 0 || math.IsNaN(dt) || math.IsInf(dt, 0) {
		return
	}
	dt *= s.timeScale
	if math.IsInf(dt, 0) {
		return
	}
	s.advanceLocked(dt)
}

func (s *Simulation) Step(dt float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if math.IsNaN(dt) || math.IsInf(dt, 0) {
		return
	}
	if dt <= 0 {
		dt = float64(s.graph.Simulation.TickMS) / 1000
	}
	dt *= s.timeScale
	if math.IsInf(dt, 0) {
		return
	}
	s.advanceLocked(dt)
}

func (s *Simulation) advanceLocked(dt float64) {
	queuedAtSignals := 0
	for _, person := range s.people {
		if person.state == "waiting" {
			s.vehicleDelay += dt
		}
		if person.waitingSignal != "" {
			s.signalDelay += dt
			person.signalWait += dt
			queuedAtSignals++
		}
	}
	if queuedAtSignals > s.maxSignalQueue {
		s.maxSignalQueue = queuedAtSignals
	}
	s.spawnLocked(dt)
	for _, person := range s.people {
		if person.state == "walking_to_lane" || person.state == "walking_to_destination" {
			s.advanceWalking(person, dt)
		} else if person.state == "turning" {
			s.advanceTurning(person, dt)
		} else if person.state == "arrived" && s.time-person.arrivedAt >= s.graph.Simulation.ArrivalDwell {
			s.startTripLocked(person)
		}
	}
	s.advanceLaneChangesLocked(dt)
	s.updateSignalsLocked(dt)
	// Signal runtimes have now advanced through this entire interval. Advance the
	// shared simulation clock before stop-line decisions so adaptive and fixed
	// controllers are both queried at the same end-of-step instant.
	s.time += dt
	s.planLaneChangesLocked()
	s.advanceRoadVehiclesLocked(dt)

	for _, person := range s.people {
		if person.state != "driving" && person.state != "waiting" {
			continue
		}
		way := s.graph.Ways[person.wayID]
		if person.routeIndex == len(person.route)-1 && person.along >= person.destinationAlong-0.0001 {
			s.exitLane(person)
		} else if person.along >= s.stopLine(way)-0.0001 {
			s.transition(person)
		}
	}
	s.tick++
}

func (s *Simulation) advanceWalking(person *personState, dt float64) {
	delta := person.walkTarget.Sub(person.position)
	distance := delta.Length()
	person.heading = delta.Normalize()
	person.speed = s.graph.Simulation.WalkSpeed
	step := person.speed * dt
	if distance <= step || distance < 0.001 {
		person.position = person.walkTarget
		if person.state == "walking_to_destination" {
			person.state, person.speed, person.arrivedAt, person.waitingReason = "arrived", 0, s.time, ""
			s.completedTrips++
			return
		}
		wayID := person.route[0]
		if !s.laneEntryAvailable(wayID, person.lane, person.along, person) {
			person.speed, person.waitingReason = 0, "road entry"
			return
		}
		person.wayID = wayID
		way := s.graph.Ways[wayID]
		person.position, person.heading = way.Sample(person.along, person.lane)
		person.state, person.speed, person.waitingReason = "driving", math.Min(s.graph.Simulation.DriveSpeed, way.SpeedLimit), ""
		return
	}
	person.position = person.position.Add(person.heading.Mul(step))
}

func (s *Simulation) transition(person *personState) {
	current := s.graph.Ways[person.wayID]
	intersection := s.graph.Intersections[current.To]
	if person.routeIndex+1 >= len(person.route) {
		s.exitLane(person)
		return
	}
	nextIndex := person.routeIndex + 1
	next := s.graph.Ways[person.route[nextIndex]]
	if person.laneChanging {
		person.state, person.speed, person.waitingFor = "waiting", 0, 0
		person.waitingReason = "lane change"
		return
	}
	activeMovement, movementAllowed := s.signalMovement(intersection.ID, current.ID, next.ID, person.lane)
	if !movementAllowed {
		if person.waitingSignal == "" {
			s.signalStops++
			person.signalWait = 0
		}
		person.state, person.speed, person.waitingSignal = "waiting", 0, intersection.ID
		person.waitingReason = "signal"
		return
	}
	preferredLane := person.id % next.Lanes
	if len(person.routeLanes) == len(person.route) {
		preferredLane = person.routeLanes[nextIndex]
	} else if nextIndex+1 < len(person.route) {
		preferredLane = s.laneForMovement(next.ID, person.route[nextIndex+1], person.id)
	}
	if nextIndex == len(person.route)-1 {
		preferredLane = person.destinationLane
	}
	nextAlong := math.Min(s.graph.Simulation.Intersection, next.Length)
	if nextIndex == len(person.route)-1 {
		nextAlong = math.Min(nextAlong, person.destinationAlong)
	}
	receivingLanes := s.routeCompatibleReceivingLanes(person, next, nextIndex, activeMovement.TargetLanes(person.lane))
	if len(receivingLanes) == 0 {
		if person.waitingSignal == "" {
			s.signalStops++
			person.signalWait = 0
		}
		person.state, person.speed, person.waitingSignal = "waiting", 0, intersection.ID
		person.waitingReason = "signal"
		return
	}
	nextLane, entryAvailable := s.selectReceivingLane(person, next, receivingLanes, preferredLane, nextAlong)
	if !entryAvailable {
		if s.rerouteAroundCongestion(person, next.ID) {
			s.releaseSignalWait(person)
			person.state, person.speed, person.waitingFor = "waiting", 0, 0
			person.waitingReason = "rerouting"
			return
		}
		s.releaseSignalWait(person)
		person.state, person.speed, person.waitingFor = "waiting", 0, 0
		person.waitingReason = "downstream storage"
		return
	}
	s.releaseSignalWait(person)
	start, incomingTangent := current.Sample(s.stopLine(current), person.lane)
	end, outgoingTangent := next.Sample(nextAlong, nextLane)
	handle := math.Min(s.graph.Simulation.Intersection*.9, start.Sub(end).Length()*.45)
	person.turnStart, person.turnEnd = start, end
	person.turnControl1 = start.Add(incomingTangent.Mul(handle))
	person.turnControl2 = end.Sub(outgoingTangent.Mul(handle))
	person.turnProgress = 0
	person.turnLength = cubicLength(start, person.turnControl1, person.turnControl2, end)
	person.nextWayID, person.nextLane, person.nextAlong = next.ID, nextLane, nextAlong
	person.state, person.speed = "turning", math.Min(s.graph.Simulation.DriveSpeed, math.Min(current.SpeedLimit, next.SpeedLimit))
	person.position = start
	person.heading = person.turnControl1.Sub(start).Normalize()
	person.waitingFor, person.waitingSignal, person.waitingReason, person.signalWait = 0, "", "", 0
	person.routeIndex = nextIndex
	// The next update follows a tangent-continuous cubic connector rather than
	// teleporting between the two lane centre lines.
}

func (s *Simulation) advanceTurning(person *personState, dt float64) {
	person.turnProgress = math.Min(1, person.turnProgress+person.speed*dt/person.turnLength)
	t := person.turnProgress
	u := 1 - t
	person.position = person.turnStart.Mul(u * u * u).
		Add(person.turnControl1.Mul(3 * u * u * t)).
		Add(person.turnControl2.Mul(3 * u * t * t)).
		Add(person.turnEnd.Mul(t * t * t))
	person.heading = person.turnControl1.Sub(person.turnStart).Mul(3 * u * u).
		Add(person.turnControl2.Sub(person.turnControl1).Mul(6 * u * t)).
		Add(person.turnEnd.Sub(person.turnControl2).Mul(3 * t * t)).Normalize()
	if t < 1 {
		return
	}
	person.wayID, person.lane, person.along = person.nextWayID, person.nextLane, person.nextAlong
	person.lanePosition = float64(person.lane)
	person.position, person.heading = s.graph.Ways[person.wayID].SampleLane(person.along, person.lanePosition)
	person.state, person.waitingReason = "driving", ""
}

func (s *Simulation) releaseSignalWait(person *personState) {
	if person.waitingSignal == "" {
		return
	}
	s.signalReleases++
	s.releasedDelay += person.signalWait
	person.waitingSignal, person.signalWait = "", 0
}

func chooseExitLane(allowed []int, preferred, fromLane, fromLanes, toLanes int) int {
	if len(allowed) == 0 {
		return max(0, min(toLanes-1, preferred))
	}
	for _, lane := range allowed {
		if lane == preferred {
			return lane
		}
	}
	mapped := 0
	if fromLanes > 1 && toLanes > 1 {
		mapped = int(math.Round(float64(fromLane) * float64(toLanes-1) / float64(fromLanes-1)))
	}
	best, bestDistance := allowed[0], math.MaxInt
	for _, lane := range allowed {
		if distance := absInt(lane - mapped); distance < bestDistance {
			best, bestDistance = lane, distance
		}
	}
	return best
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func cubicLength(start, control1, control2, end model.Vec2) float64 {
	previous, length := start, 0.0
	for step := 1; step <= 12; step++ {
		t := float64(step) / 12
		u := 1 - t
		point := start.Mul(u * u * u).Add(control1.Mul(3 * u * u * t)).Add(control2.Mul(3 * u * t * t)).Add(end.Mul(t * t * t))
		length += point.Sub(previous).Length()
		previous = point
	}
	return math.Max(1, length)
}

func (s *Simulation) laneForMovement(fromWayID, toWayID string, personID int) int {
	from := s.graph.Ways[fromWayID]
	if from == nil {
		return 0
	}
	control := s.graph.Intersections[from.To].TrafficControl
	var available []int
	for lane := 0; lane < from.Lanes; lane++ {
		if _, ok := control.Movement(fromWayID, toWayID, lane); ok {
			available = append(available, lane)
		}
	}
	if len(available) == 0 {
		return personID % from.Lanes
	}
	return available[personID%len(available)]
}

func (s *Simulation) roadTarget(person *personState, way *model.Way) float64 {
	if person.routeIndex == len(person.route)-1 {
		return person.destinationAlong
	}
	return s.stopLine(way)
}

func (s *Simulation) stopLine(way *model.Way) float64 {
	return math.Max(0, way.Length-s.graph.Simulation.Intersection)
}

func (s *Simulation) exitLane(person *personState) {
	way := s.graph.Ways[person.wayID]
	person.position, person.heading = way.SampleLane(person.destinationAlong, person.lanePosition)
	person.exitPoint = person.position
	person.walkTarget = person.destination
	person.wayID = ""
	person.state = "walking_to_destination"
	person.speed = s.graph.Simulation.WalkSpeed
	person.laneChanging = false
	person.waitingFor, person.waitingSignal, person.waitingReason, person.signalWait = 0, "", "", 0
	person.heading = person.destination.Sub(person.position).Normalize()
}

func (s *Simulation) Reset(population int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if population <= 0 {
		population = s.graph.Simulation.People
	}
	s.resetLocked(population)
}

func (s *Simulation) resetLocked(population int) {
	s.people = make([]*personState, 0, population)
	s.time, s.tick, s.nextID, s.spawnRemainder = 0, 0, 1, 0
	s.vehicleDelay, s.signalDelay, s.releasedDelay = 0, 0, 0
	s.signalStops, s.signalReleases, s.completedTrips, s.maxSignalQueue = 0, 0, 0, 0
	s.initSignalsLocked()
	if population > s.maxPeople {
		s.maxPeople = population
	}
	s.appendPeopleLocked(population)
	// Populate deterministic next-phase and pressure diagnostics immediately;
	// callers should not need to advance time before the first snapshot.
	s.updateSignalsLocked(0)
}

func (s *Simulation) spawnLocked(dt float64) {
	if s.spawnRate <= 0 || len(s.people) >= s.maxPeople {
		return
	}
	s.spawnRemainder += s.spawnRate * dt
	if s.spawnRemainder < 1 {
		return
	}
	loads := s.currentCongestionLoads()
	for s.spawnRemainder >= 1 && len(s.people) < s.maxPeople {
		person := &personState{id: s.nextID}
		s.nextID++
		s.startTripWithLoadsLocked(person, loads)
		s.people = append(s.people, person)
		s.addPersonCongestionLoad(loads, person)
		s.spawnRemainder--
	}
}

func (s *Simulation) SetLoad(population int, spawnRate float64, maxPopulation int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if maxPopulation > 0 {
		s.maxPeople = maxPopulation
	}
	if spawnRate >= 0 {
		s.spawnRate = spawnRate
	}
	if population < 1 {
		population = len(s.people)
	}
	if population > s.maxPeople {
		population = s.maxPeople
	}
	s.appendPeopleLocked(population)
	if len(s.people) > population {
		s.people = s.people[:population]
	}
}

// appendPeopleLocked plans larger population increases in bounded parallel
// waves. Every wave reads one immutable congestion snapshot; results are then
// committed in ID order before the next wave, preserving deterministic output
// while allowing route searches to use all available CPU cores.
func (s *Simulation) appendPeopleLocked(target int) {
	remaining := target - len(s.people)
	if remaining <= 0 {
		return
	}
	loads := s.currentCongestionLoads()
	const parallelThreshold = 64
	for remaining > 0 {
		if remaining < parallelThreshold {
			for remaining > 0 {
				person := &personState{id: s.nextID}
				s.nextID++
				s.startTripWithLoadsLocked(person, loads)
				s.people = append(s.people, person)
				s.addPersonCongestionLoad(loads, person)
				remaining--
			}
			return
		}
		batchSize := min(remaining, 256)
		batch := make([]*personState, batchSize)
		for index := range batch {
			batch[index] = &personState{id: s.nextID}
			s.nextID++
		}
		parallelFor(batchSize, parallelThreshold, func(start, end int) {
			for index := start; index < end; index++ {
				s.startTripWithLoadsLocked(batch[index], loads)
			}
		})
		for _, person := range batch {
			s.people = append(s.people, person)
			s.addPersonCongestionLoad(loads, person)
		}
		remaining -= batchSize
	}
}

func (s *Simulation) SetSpacing(safeGap float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if safeGap >= 2 && safeGap <= 200 {
		s.safeGap = safeGap
	}
}

func (s *Simulation) startTripLocked(person *personState) {
	s.startTripWithLoadsLocked(person, s.currentCongestionLoads())
}

func (s *Simulation) startTripWithLoadsLocked(person *personState, loads map[string]wayCongestionLoad) {
	person.trip++
	rng := rand.New(rand.NewSource(tripSeed(s.graph.Simulation.Seed, person.id, person.trip)))
	for attempt := 0; attempt < 32; attempt++ {
		origin, destination := s.randomPoint(rng), s.randomPoint(rng)
		diagonal := math.Hypot(s.graph.Bounds.MaxX-s.graph.Bounds.MinX, s.graph.Bounds.MaxY-s.graph.Bounds.MinY)
		if origin.Sub(destination).Length() < diagonal*.18 {
			continue
		}
		start, finish := s.graph.NearestLane(origin), s.graph.NearestLane(destination)
		start.Along = math.Min(start.Along, s.stopLine(start.Way))
		finish.Along = math.Min(finish.Along, s.stopLine(finish.Way))
		start.Position, start.Tangent = start.Way.Sample(start.Along, start.Lane)
		finish.Position, finish.Tangent = finish.Way.Sample(finish.Along, finish.Lane)
		direct := start.Way.ID == finish.Way.ID && finish.Along > start.Along+1
		var connected []*model.Way
		ok := false
		var plannedLanes []int
		if direct {
			connected, ok = s.graph.RouteBetweenWays(start.Way.ID, finish.Way.ID, true)
		} else if start.Way.ID != finish.Way.ID {
			if balanced, lanes, found := s.congestionRouteWithLoads(start.Way.ID, finish.Way.ID, "", -1, loads); found {
				connected = make([]*model.Way, len(balanced))
				for index, wayID := range balanced {
					connected[index] = s.graph.Ways[wayID]
				}
				plannedLanes = lanes
				ok = true
			}
		}
		if !ok {
			connected, ok = s.graph.RouteBetweenWays(start.Way.ID, finish.Way.ID, false)
		}
		if !ok {
			continue
		}
		route := make([]string, 0, len(connected))
		for _, way := range connected {
			route = append(route, way.ID)
		}
		person.origin, person.destination = origin, destination
		person.position, person.walkTarget = origin, start.Position
		person.exitPoint, person.destinationAlong, person.destinationLane = finish.Position, finish.Along, finish.Lane
		person.route, person.routeLanes, person.routeIndex = route, plannedLanes, 0
		person.state, person.wayID = "walking_to_lane", ""
		person.lane, person.along = start.Lane, start.Along
		if len(route) > 1 {
			if len(person.routeLanes) == len(route) {
				person.lane = person.routeLanes[0]
			} else {
				person.lane = s.laneForMovement(route[0], route[1], person.id)
			}
			start.Position, start.Tangent = start.Way.Sample(start.Along, person.lane)
			person.walkTarget = start.Position
		}
		person.lanePosition = float64(person.lane)
		person.laneChanging, person.laneChangeProgress, person.laneChangeReason = false, 0, ""
		person.speed = s.graph.Simulation.WalkSpeed
		person.heading = start.Position.Sub(origin).Normalize()
		person.waitingFor, person.waitingSignal, person.waitingReason, person.signalWait = 0, "", "", 0
		return
	}
	panic("city graph cannot produce a reachable origin-to-destination trip")
}

func (s *Simulation) randomPoint(rng *rand.Rand) model.Vec2 {
	return model.Vec2{
		X: s.graph.Bounds.MinX + rng.Float64()*(s.graph.Bounds.MaxX-s.graph.Bounds.MinX),
		Y: s.graph.Bounds.MinY + rng.Float64()*(s.graph.Bounds.MaxY-s.graph.Bounds.MinY),
	}
}

// tripSeed makes every person's nth trip independent of iteration order and
// completion time. Controller A/B runs therefore see the same route sequence
// for an agent even when one controller completes earlier trips sooner.
func tripSeed(base int64, personID, trip int) int64 {
	value := uint64(base) ^ uint64(personID)*0x9e3779b97f4a7c15 ^ uint64(trip)*0xbf58476d1ce4e5b9
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return int64(value ^ (value >> 31))
}

func (s *Simulation) SetPaused(paused bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = paused
}

func (s *Simulation) SetTimeScale(scale float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if scale > 0 && scale <= 10 {
		s.timeScale = scale
	}
}

func (s *Simulation) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := Snapshot{City: s.graph.Name, Time: s.time, Tick: s.tick, Paused: s.paused, TimeScale: s.timeScale,
		Load:    LoadState{Population: len(s.people), SpawnRate: s.spawnRate, MaxPopulation: s.maxPeople},
		Spacing: SpacingState{SafeGap: s.safeGap, VehicleSize: s.graph.Simulation.VehicleSize},
		People:  make([]Person, len(s.people)), Signals: s.signalStates()}
	parallelFor(len(s.people), 256, func(start, end int) {
		for index := start; index < end; index++ {
			state := s.people[index]
			pose := model.Transform(state.position, state.heading)
			reference := s.referenceFor(state.position, state.wayID)
			person := Person{
				ID: state.id, State: state.state, Position: state.position, Speed: state.speed,
				Heading: math.Atan2(state.heading.Y, state.heading.X) * 180 / math.Pi,
				WayID:   state.wayID, Lane: state.lane, LanePosition: state.lanePosition, Along: state.along, Transform: pose,
				Reference: Reference{IntersectionID: reference.ID, IntersectionTransform: reference.Transform,
					LocalTransform: model.RelativeTo(reference.Position, pose)}, WaitingFor: state.waitingFor,
				WaitingSignal: state.waitingSignal, WaitingReason: state.waitingReason, Origin: state.origin, Destination: state.destination,
				Route: append([]string(nil), state.route...), RouteIndex: state.routeIndex, Trip: state.trip,
			}
			if state.laneChanging {
				person.LaneChange = &LaneChange{FromLane: state.laneChangeFrom, ToLane: state.laneChangeTo, Progress: state.laneChangeProgress, Reason: state.laneChangeReason}
			}
			snapshot.People[index] = person
		}
	})
	for _, person := range snapshot.People {
		snapshot.Stats.AverageSpeed += person.Speed
		switch person.State {
		case "walking_to_lane", "walking_to_destination":
			snapshot.Stats.Walking++
		case "waiting":
			snapshot.Stats.Waiting++
		case "arrived":
			snapshot.Stats.Arrived++
		default:
			snapshot.Stats.Driving++
		}
	}
	if len(snapshot.People) > 0 {
		snapshot.Stats.AverageSpeed /= float64(len(snapshot.People))
	}
	algorithm := "optimized Webster fixed-time"
	if s.adaptive {
		algorithm = "Webster + max-pressure adaptive"
	}
	currentQueue := 0
	for _, person := range s.people {
		if person.waitingSignal != "" {
			currentQueue++
		}
	}
	meanSignalDelay := 0.0
	if s.signalReleases > 0 {
		meanSignalDelay = s.releasedDelay / float64(s.signalReleases)
	}
	throughput := 0.0
	if s.time > 0 {
		throughput = float64(s.completedTrips) * 60 / s.time
	}
	snapshot.Performance = ControlPerformance{
		Algorithm: algorithm, VehicleDelaySeconds: s.vehicleDelay, SignalDelaySeconds: s.signalDelay,
		MeanSignalDelay: meanSignalDelay, SignalStops: s.signalStops, SignalReleases: s.signalReleases,
		CompletedTrips: s.completedTrips, ThroughputPerMinute: throughput,
		CurrentSignalQueue: currentQueue, MaximumSignalQueue: s.maxSignalQueue,
	}
	return snapshot
}

func (s *Simulation) signalStates() []SignalState {
	states := make([]SignalState, 0, len(s.graph.Intersections))
	for _, intersection := range s.graph.SortedIntersections() {
		active := s.activeSignal(intersection)
		nextIndex := 0
		nextPhase := model.TrafficPhaseConfig{}
		if len(intersection.TrafficControl.Phases) > 0 {
			nextIndex = (active.Index + 1) % len(intersection.TrafficControl.Phases)
			if runtime := s.signals[intersection.ID]; runtime != nil {
				nextIndex = runtime.SelectedPhase
				if nextIndex < 0 || nextIndex >= len(intersection.TrafficControl.Phases) {
					nextIndex = (active.Index + 1) % len(intersection.TrafficControl.Phases)
				}
			}
			nextPhase = intersection.TrafficControl.Phases[nextIndex]
		}
		controller := "fixed-time"
		decision := "configured schedule"
		targetGreen := active.Phase.DurationSeconds
		phaseElapsed := active.Phase.DurationSeconds - active.Remaining
		queuePressure, competingPressure := 0.0, 0.0
		incomingQueue, outgoingQueue := 0.0, 0.0
		phasePressures := make([]PhasePressure, 0)
		if runtime := s.signals[intersection.ID]; runtime != nil {
			controller = "adaptive max-pressure"
			decision = runtime.Decision
			targetGreen = runtime.GreenTarget
			phaseElapsed = runtime.StageElapsed
			queuePressure, competingPressure = runtime.ActivePressure, runtime.CompetingPressure
			phasePressures = clonePhasePressures(runtime.PhasePressures)
			if active.Index >= 0 && active.Index < len(runtime.PhasePressures) {
				incomingQueue = runtime.PhasePressures[active.Index].IncomingQueue
				outgoingQueue = runtime.PhasePressures[active.Index].OutgoingQueue
			}
		}
		state := SignalState{
			IntersectionID: intersection.ID, PhaseIndex: active.Index, PhaseID: active.Phase.ID,
			PhaseName: active.Phase.Name, Stage: active.Stage, Remaining: active.Remaining,
			CycleSeconds: intersection.TrafficControl.CycleSeconds,
			PhaseElapsed: phaseElapsed,
			NextPhaseID:  nextPhase.ID, NextPhaseName: nextPhase.Name,
			AllowedIncoming: []string{}, AllowedMovements: []Movement{},
			Controller: controller, Decision: decision, BaseGreen: active.Phase.DurationSeconds,
			TargetGreen: targetGreen, MinGreen: active.Phase.MinGreenSeconds, MaxGreen: active.Phase.MaxGreenSeconds,
			QueuePressure: queuePressure, CompetingPressure: competingPressure,
			IncomingQueue: incomingQueue, OutgoingQueue: outgoingQueue, PhasePressures: phasePressures,
			ExtensionSeconds: math.Max(0, targetGreen-active.Phase.DurationSeconds),
		}
		if active.Stage == "green" {
			state.AllowedIncoming = append([]string(nil), active.Phase.AllowedIncoming...)
			for _, movement := range active.Phase.AllowedMovements {
				state.AllowedMovements = append(state.AllowedMovements, Movement{
					FromWayID: movement.FromWayID, ToWayID: movement.ToWayID,
					FromLanes: append([]int(nil), movement.FromLanes...), ToLanes: append([]int(nil), movement.ToLanes...),
					LaneConnections: append([]model.LaneConnectionConfig(nil), movement.LaneConnections...),
					Turn:            movement.Turn, DemandScore: movement.DemandScore,
				})
			}
		}
		states = append(states, state)
	}
	return states
}

func clonePhasePressures(source []PhasePressure) []PhasePressure {
	clone := make([]PhasePressure, len(source))
	for index, phase := range source {
		clone[index] = phase
		clone[index].Movements = append([]MovementPressure(nil), phase.Movements...)
		for movementIndex := range clone[index].Movements {
			clone[index].Movements[movementIndex].FromLanes = append([]int(nil), phase.Movements[movementIndex].FromLanes...)
			clone[index].Movements[movementIndex].ToLanes = append([]int(nil), phase.Movements[movementIndex].ToLanes...)
			clone[index].Movements[movementIndex].LaneConnections = append([]model.LaneConnectionConfig(nil), phase.Movements[movementIndex].LaneConnections...)
		}
	}
	return clone
}

func (s *Simulation) referenceFor(position model.Vec2, wayID string) *model.Intersection {
	if way, ok := s.graph.Ways[wayID]; ok {
		from, to := s.graph.Intersections[way.From], s.graph.Intersections[way.To]
		if position.Sub(from.Position).Length() <= position.Sub(to.Position).Length() {
			return from
		}
		return to
	}
	var nearest *model.Intersection
	best := math.Inf(1)
	for _, intersection := range s.graph.Intersections {
		if distance := position.Sub(intersection.Position).Length(); distance < best {
			best, nearest = distance, intersection
		}
	}
	return nearest
}
