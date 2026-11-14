package sim

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
	"traffic/internal/foundation"
	"traffic/internal/infrastructure/interchange"
	"traffic/internal/metro"
	"traffic/internal/operations/demand"

	model "traffic/internal/infrastructure/road"
)

type Person struct {
	JourneyID       int                  `json:"journey_id,omitempty"`
	Entity          foundation.EntityRef `json:"entity"`
	Structure       string               `json:"structure"`
	Elevation       float64              `json:"elevation"`
	ID              int                  `json:"id"`
	State           string               `json:"state"`
	Position        model.Vec2           `json:"position"`
	Speed           float64              `json:"speed"`
	Heading         float64              `json:"heading_degrees"`
	WayID           string               `json:"way_id,omitempty"`
	Lane            int                  `json:"lane"`
	LanePosition    float64              `json:"lane_position"`
	LaneChange      *LaneChange          `json:"lane_change,omitempty"`
	Along           float64              `json:"along"`
	Transform       model.Matrix3        `json:"transform"`
	Reference       Reference            `json:"reference"`
	WaitingFor      int                  `json:"waiting_for,omitempty"`
	WaitingSignal   string               `json:"waiting_signal,omitempty"`
	WaitingReason   string               `json:"waiting_reason,omitempty"`
	Origin          model.Vec2           `json:"origin"`
	Destination     model.Vec2           `json:"destination"`
	Route           []string             `json:"route"`
	RouteEntryLanes []int                `json:"route_entry_lanes,omitempty"`
	RouteExitLanes  []int                `json:"route_exit_lanes,omitempty"`
	RouteIndex      int                  `json:"route_index"`
	Trip            int                  `json:"trip"`
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
	Metro metro.View `json:"metro"`

	Demand *demand.Summary `json:"demand,omitempty"`

	Algorithms AlgorithmView `json:"algorithms"`

	WalkSpeed float64 `json:"walk_speed_mps"`

	Kernel      foundation.Stamp   `json:"kernel"`
	Mode        string             `json:"mode"`
	Taxi        *TaxiView          `json:"taxi,omitempty"`
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
	MacroInterchanges []interchange.Node        `json:"macro_interchanges,omitempty"`
	WorldUnitsPerKM   float64                   `json:"world_units_per_km"`
	Interchanges      []model.Interchange       `json:"interchanges,omitempty"`
	Name              string                    `json:"name"`
	Description       string                    `json:"description"`
	DrivingSide       string                    `json:"driving_side"`
	MovementRules     model.MovementRulesConfig `json:"movement_rules"`
	Bounds            model.Bounds              `json:"bounds"`
	Simulation        model.SimulationConfig    `json:"simulation"`
	Intersections     []*model.Intersection     `json:"intersections"`
	Ways              []*model.Way              `json:"ways"`
	Edges             []model.Edge              `json:"edges"`
}

type personState struct {
	journeyID int

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
	routeEntryLanes    []int
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
	carryDistance      float64
	turnLength         float64
	nextWayID          string
	nextLane           int
	nextAlong          float64
}

type Simulation struct {
	metro *metro.Runtime

	mode         string
	demandStream *demand.Stream

	algorithm algorithmRuntime

	walkSpeed float64

	stamp         foundation.Stamp
	commands      foundation.Queue[Command]
	arrivalsQueue foundation.Queue[Arrival]
	reservations  map[int]*flowReservation

	clock          foundation.Clock
	stepRemainder  float64
	journal        foundation.Journal
	policies       Strategies
	taxi           *taxiRuntime
	taxiEpoch      uint64
	mu             sync.RWMutex
	graph          *model.Graph
	router         *interchange.Planner
	people         []*personState
	nextID         int
	spawnRate      float64
	maxPeople      int
	safeGap        float64
	arrivals       demand.Arrivals
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
	s := &Simulation{walkSpeed: graph.Simulation.WalkSpeed, graph: graph, router: interchange.NewPlanner(graph), clock: foundation.Clock{Rate: 1},
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
	return CityView{MacroInterchanges: interchange.Build(s.graph), Interchanges: s.graph.Interchanges,
		WorldUnitsPerKM: s.graph.WorldUnitsPerKM, Name: s.graph.Name, Description: s.graph.Description, DrivingSide: s.graph.DrivingSide, MovementRules: s.graph.MovementRules, Bounds: s.graph.Bounds,
		Simulation: s.graph.Simulation, Intersections: s.graph.SortedIntersections(),
		Ways: s.graph.SortedWays(), Edges: append([]model.Edge(nil), s.graph.Edges...),
	}
}

// ReplaceGraph atomically switches the running simulation to another validated
// city graph and restores that preset's initial load and control settings.
func (s *Simulation) replaceGraphLocked(graph *model.Graph) {
	if graph == nil {
		return
	}
	s.stamp.Topology++
	s.graph = graph
	s.walkSpeed = graph.Simulation.WalkSpeed
	s.router = interchange.NewPlanner(graph)
	s.clock.Paused = false
	s.clock.Rate = 1
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
	s.drainCommandsLocked()
	if s.clock.Paused || dt <= 0 || math.IsNaN(dt) || math.IsInf(dt, 0) {
		return
	}
	dt *= s.clock.Rate
	if math.IsInf(dt, 0) {
		return
	}
	s.advanceLocked(dt)
}

func (s *Simulation) stepLocked(dt float64) {
	if math.IsNaN(dt) || math.IsInf(dt, 0) {
		return
	}
	if dt <= 0 {
		dt = float64(s.graph.Simulation.TickMS) / 1000
	}
	dt *= s.clock.Rate
	if math.IsInf(dt, 0) {
		return
	}
	s.advanceLocked(dt)
}

// Both scenarios advance on the same 50 ms physical clock. Fractional input
// time is carried into the next call; playback rate changes only the step count.
func (s *Simulation) advanceLocked(dt float64) {
	s.stepRemainder += dt
	for s.stepRemainder >= .05-1e-9 {
		before := s.clock.Time
		s.advanceSubstep(.05)
		if s.clock.Time == before {
			s.stepRemainder = 0
			return
		}
		s.stepRemainder = math.Max(0, s.stepRemainder-.05)
	}
}

// Tick pipeline: commands -> due decisions -> demand/admission -> controls and
// reservations -> road/connector motion -> ordered arrivals -> accounting.
// Snapshot readers acquire the same lock and only see a completed boundary.
func (s *Simulation) advanceSubstep(dt float64) {
	wasPaused, run := s.clock.Paused, s.stamp.Run
	s.drainCommandsLocked()
	if run != s.stamp.Run || (!wasPaused && s.clock.Paused) {
		return
	}
	s.bindJournal()
	if !s.advanceAlgorithmsLocked() {
		return
	}
	if !s.taxiDispatch() {
		return
	}
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
	s.advanceSharedDemand()
	s.spawnLocked(dt)
	for _, person := range s.people {
		if person.state == "walking_to_lane" || person.state == "walking_to_destination" {
			s.advanceWalking(person, dt)
		} else if s.demandStream == nil && person.state == "arrived" && s.clock.Time-person.arrivedAt >= s.graph.Simulation.ArrivalDwell {
			s.startTripLocked(person)
		}
	}
	s.advanceLaneChangesLocked(dt)
	s.updateSignalsLocked(dt)
	// Signal runtimes have now advanced through this entire interval. Advance the
	// shared simulation clock before stop-line decisions so adaptive and fixed
	// controllers are both queried at the same end-of-step instant.
	s.clock.Time += dt
	s.planLaneChangesLocked()
	s.reserveFlowLocked()
	s.advanceRoadVehiclesLocked(dt)
	for _, p := range s.people {
		if p.state == "turning" {
			s.advanceTurning(p, dt)
		}
	}

	for _, person := range s.people {
		if person.state != "driving" && person.state != "waiting" {
			continue
		}
		way := s.graph.Ways[person.wayID]
		if person.routeIndex == len(person.route)-1 && person.along >= person.destinationAlong-0.0001 {
			s.queueArrival(person, "destination")
		} else if person.along >= s.stopLine(way)-0.0001 {
			s.queueArrival(person, "connector")
		}
	}
	s.commitArrivalsLocked()
	s.taxiMaintenance(dt)
	s.removeFinishedPrivateCars()
	s.metro.Advance(dt)
	s.clock.Tick++
	s.stamp.World++
	s.bindJournal()
}

func (s *Simulation) advanceWalking(person *personState, dt float64) {
	delta := person.walkTarget.Sub(person.position)
	distance := delta.Length()
	person.heading = delta.Normalize()
	person.speed = s.walkSpeed
	step := person.speed * dt
	if distance <= step || distance < 0.001 {
		person.position = person.walkTarget
		if person.state == "walking_to_destination" {
			person.state, person.speed, person.arrivedAt, person.waitingReason = "arrived", 0, s.clock.Time, ""
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
		person.state, person.speed, person.waitingReason = "driving", 0, ""
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
		if len(person.routeEntryLanes) == len(person.route) {
			preferredLane = person.routeEntryLanes[nextIndex]
		}
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
	if r := s.flowReservation(person); r != nil {
		nextLane, nextAlong, entryAvailable = r.lane, r.along, true
	}
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
	person.state, person.speed = "turning", math.Min(person.speed, math.Min(current.SpeedLimit, next.SpeedLimit))
	person.position = start
	person.heading = person.turnControl1.Sub(start).Normalize()
	person.waitingFor, person.waitingSignal, person.waitingReason, person.signalWait = 0, "", "", 0
	person.routeIndex = nextIndex
	if person.carryDistance > 0 {
		s.moveConnector(person, person.carryDistance)
		person.carryDistance = 0
	}
	// The next update follows a tangent-continuous cubic connector rather than
	// teleporting between the two lane centre lines.
}

func (s *Simulation) advanceTurning(person *personState, dt float64) {
	limit := math.Min(s.graph.Ways[person.wayID].SpeedLimit, s.graph.Ways[person.nextWayID].SpeedLimit)
	safety := math.Inf(1)
	if r := s.flowReservation(person); r != nil {
		safety = s.downstreamSafety(person, r, person.turnLength*(1-person.turnProgress), dt)
		limit = math.Min(limit, s.downstreamSpeed(person, r, person.turnLength*(1-person.turnProgress), dt))
	}
	person.speed = advanceSpeed(person.speed, limit, safety, dt)
	s.moveConnector(person, person.speed*dt)
}
func (s *Simulation) moveConnector(person *personState, distance float64) {
	person.turnProgress = math.Min(1, person.turnProgress+distance/math.Max(.001, person.turnLength))
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
	delete(s.reservations, person.id)
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
	s.finishJourney(person.journeyID, false)
	if s.taxiArrive(person) {
		return
	}
	way := s.graph.Ways[person.wayID]
	person.position, person.heading = way.SampleLane(person.destinationAlong, person.lanePosition)
	person.exitPoint = person.position
	person.walkTarget = person.destination
	person.wayID = ""
	person.state = "walking_to_destination"
	person.speed = s.walkSpeed
	person.laneChanging = false
	person.waitingFor, person.waitingSignal, person.waitingReason, person.signalWait = 0, "", "", 0
	person.heading = person.destination.Sub(person.position).Normalize()
}

func (s *Simulation) resetLockedCommand(population int) {
	if population <= 0 {
		population = s.graph.Simulation.People
	}
	s.resetLocked(population)
}

func (s *Simulation) resetLocked(population int) {
	mc := metro.DefaultConfig()
	if s.metro != nil {
		mc = s.metro.Snapshot().Config
	}
	if s.graph.Metro != nil {
		if s.metro == nil {
			mc = s.graph.Metro.Defaults
		}
		s.metro = metro.NewNetwork(s.graph.Metro, mc)
	} else {
		s.metro = metro.New(nil, mc)
	}
	if s.demandStream != nil {
		s.demandStream = demand.New(s.demandStream.Config, 0)
	}
	s.cancelAlgorithms()
	s.stamp.Run++
	s.stamp.World++
	if s.stamp.Topology == 0 {
		s.stamp.Topology = 1
	}
	for _, e := range s.commands.Discard() {
		s.journal.Record(s.clock.Time, "command", int(e.Entity), fmt.Sprintf("%d:%s:invalidated by reset", e.Sequence, e.Payload.Kind))
	}
	s.arrivalsQueue.Clear()
	s.reservations = make(map[int]*flowReservation)
	s.bindJournal()
	var taxiConfig *TaxiConfig
	if s.taxi != nil {
		c := s.taxi.config
		taxiConfig = &c
	}
	s.clearTaxi()
	s.people = make([]*personState, 0, population)
	s.stepRemainder = 0
	s.clock.Time, s.clock.Tick, s.nextID = 0, 0, 1
	s.bindJournal()
	s.arrivals = demand.Arrivals{}
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
	if taxiConfig != nil && len(s.taxiStops()) >= 2 {
		if s.demandStream != nil {
			s.startTaxiLive(*taxiConfig)
		} else {
			s.startTaxi(*taxiConfig)
		}
	}
}

func (s *Simulation) spawnLocked(dt float64) {
	count := s.arrivals.Advance(s.spawnRate, dt, s.maxPeople-len(s.people))
	if count == 0 {
		return
	}
	loads := s.currentCongestionLoads()
	for i := 0; i < count; i++ {
		person := &personState{id: s.nextID}
		s.nextID++
		s.startTripWithLoadsLocked(person, loads)
		s.people = append(s.people, person)
		s.addPersonCongestionLoad(loads, person)
	}
}

func (s *Simulation) setLoadLocked(population int, spawnRate float64, maxPopulation int) {
	if s.taxi != nil {
		return
	}
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

func (s *Simulation) setSpacingLocked(safeGap float64) {
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
		var plan interchange.Route
		var ok bool
		if direct {
			plan = interchange.Route{Steps: []interchange.Step{{WayID: start.Way.ID, EntryLane: start.Lane, ExitLane: start.Lane}}}
			ok = true
		} else {
			plan, ok = s.congestionRouteWithLoads(start.Way.ID, finish.Way.ID, "", -1, start.Along, loads)
		}
		if !ok {
			continue
		}
		route, plannedLanes, entryLanes := plan.Arrays()
		person.origin, person.destination = origin, destination
		person.position, person.walkTarget = origin, start.Position
		person.exitPoint, person.destinationAlong, person.destinationLane = finish.Position, finish.Along, finish.Lane
		person.route, person.routeLanes, person.routeEntryLanes, person.routeIndex = route, plannedLanes, entryLanes, 0
		person.state, person.wayID = "walking_to_lane", ""
		person.lane, person.along = start.Lane, start.Along
		if len(route) > 1 {
			if len(person.routeLanes) == len(route) {
				person.lane = person.routeEntryLanes[0]
			} else {
				person.lane = s.laneForMovement(route[0], route[1], person.id)
			}
			start.Position, start.Tangent = start.Way.Sample(start.Along, person.lane)
			person.walkTarget = start.Position
		}
		person.lanePosition = float64(person.lane)
		person.laneChanging, person.laneChangeProgress, person.laneChangeReason = false, 0, ""
		person.speed = s.walkSpeed
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

func (s *Simulation) setPausedLocked(paused bool) {
	s.clock.Paused = paused
}

func (s *Simulation) setTimeScaleLocked(scale float64) {
	if scale > 0 && scale <= 10 {
		s.clock.Rate = scale
	}
}

func (s *Simulation) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := Snapshot{Metro: s.metro.Snapshot(), Algorithms: s.algorithmView(), WalkSpeed: s.walkSpeed, Kernel: s.stamp, City: s.graph.Name, Time: s.clock.Time, Tick: s.clock.Tick, Paused: s.clock.Paused, TimeScale: s.clock.Rate,
		Load:    LoadState{Population: len(s.people), SpawnRate: s.spawnRate, MaxPopulation: s.maxPeople},
		Spacing: SpacingState{SafeGap: s.safeGap, VehicleSize: s.graph.Simulation.VehicleSize},
		People:  make([]Person, len(s.people)), Signals: s.signalStates()}
	parallelFor(len(s.people), 256, func(start, end int) {
		for index := start; index < end; index++ {
			state := s.people[index]
			pose := model.Transform(state.position, state.heading)
			reference := s.referenceFor(state.position, state.wayID)
			person := Person{
				JourneyID: state.journeyID, Entity: foundation.EntityRef{Run: s.stamp.Run, ID: foundation.EntityID(state.id)}, ID: state.id, State: state.state, Position: state.position, Speed: state.speed,
				Heading: math.Atan2(state.heading.Y, state.heading.X) * 180 / math.Pi,
				WayID:   state.wayID, Lane: state.lane, LanePosition: state.lanePosition, Along: state.along, Transform: pose,
				Reference: Reference{IntersectionID: reference.ID, IntersectionTransform: reference.Transform,
					LocalTransform: model.RelativeTo(reference.Position, pose)}, WaitingFor: state.waitingFor,
				WaitingSignal: state.waitingSignal, WaitingReason: state.waitingReason, Origin: state.origin, Destination: state.destination,
				Route: append([]string(nil), state.route...), RouteIndex: state.routeIndex, Trip: state.trip,
				RouteEntryLanes: append([]int(nil), state.routeEntryLanes...), RouteExitLanes: append([]int(nil), state.routeLanes...),
			}
			person.Structure = "ground"
			if way := s.graph.Ways[state.wayID]; way != nil {
				person.Structure = way.Structure
				fraction := math.Max(0, math.Min(1, state.along/way.Length))
				fraction = fraction * fraction * (3 - 2*fraction)
				person.Elevation = way.StartElevation + (way.EndElevation-way.StartElevation)*fraction
				if state.state == "turning" {
					person.Elevation = way.EndElevation
					if next := s.graph.Ways[state.nextWayID]; next != nil && state.turnProgress >= .5 {
						person.Structure = next.Structure
					}
				}
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
	if s.clock.Time > 0 {
		throughput = float64(s.completedTrips) * 60 / s.clock.Time
	}
	snapshot.Performance = ControlPerformance{
		Algorithm: algorithm, VehicleDelaySeconds: s.vehicleDelay, SignalDelaySeconds: s.signalDelay,
		MeanSignalDelay: meanSignalDelay, SignalStops: s.signalStops, SignalReleases: s.signalReleases,
		CompletedTrips: s.completedTrips, ThroughputPerMinute: throughput,
		CurrentSignalQueue: currentQueue, MaximumSignalQueue: s.maxSignalQueue,
	}

	snapshot.Mode = s.mode
	if snapshot.Mode == "" {
		snapshot.Mode = "traffic"
	}
	if s.demandStream != nil {
		d := s.demandStream.Summary()
		snapshot.Demand = &d
	}
	if s.taxi != nil {
		snapshot.Taxi = s.taxiView()
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

func (s *Simulation) Events() []foundation.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.journal.Events()
}
