package metro

import (
	"fmt"
	"math"
	"math/rand"
)

type Config struct {
	LineLayers map[string]int `json:"line_layers,omitempty"`
	Cars       int            `json:"cars"`
	MaxSpeed   float64        `json:"max_speed_kmh"`
	Enabled    bool           `json:"enabled"`
	Headway    float64        `json:"headway_s"`
	Demand     float64        `json:"passengers_per_minute"`
	Capacity   int            `json:"capacity"`
	Dwell      float64        `json:"dwell_s"`
	Seed       int64          `json:"seed"`
}

func DefaultConfig() Config {
	return Config{Cars: 6, MaxSpeed: 300, Enabled: true, Headway: 180, Demand: 12, Capacity: 120, Dwell: 20, Seed: 4181}
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type StationView struct {
	Transfer string `json:"transfer,omitempty"`
	Layer    int    `json:"layer"`
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Position Point  `json:"position"`
	Waiting  int    `json:"waiting"`
}
type Passenger struct {
	NetworkID   int
	JourneyID   int
	Car         int
	ID          int
	Destination int
}
type TrainView struct {
	Layer         int       `json:"layer"`
	CarPoses      []CarPose `json:"car_poses,omitempty"`
	CarLength     float64   `json:"car_length_m"`
	CarWidth      float64   `json:"car_width_m"`
	Line          int       `json:"line"`
	Cars          []int     `json:"cars"`
	CarCapacities []int     `json:"car_capacities"`
	Heading       float64   `json:"heading"`
	ID            int       `json:"id"`
	Station       int       `json:"station"`
	Next          int       `json:"next"`
	Direction     int       `json:"direction"`
	State         string    `json:"state"`
	Position      Point     `json:"position"`
	Speed         float64   `json:"speed_mps"`
	Length        float64   `json:"length_m"`
	Capacity      int       `json:"capacity"`
	Passengers    int       `json:"passengers"`
	Departure     float64   `json:"departure_s"`
	Delay         float64   `json:"delay_s"`
	Distance      float64   `json:"distance_m"`
	Authority     float64   `json:"authority_m"`
}
type Block struct {
	ID       string  `json:"id"`
	From     int     `json:"from"`
	To       int     `json:"to"`
	Start    float64 `json:"start_m"`
	End      float64 `json:"end_m"`
	Owner    int     `json:"reserved_by"`
	Occupied int     `json:"occupied_by"`
}
type Route struct {
	Station int `json:"station"`
	Owner   int `json:"owner"`
}
type LineView struct {
	ReverseTrack []Point `json:"reverse_track,omitempty"`
	Circular     bool    `json:"circular"`
	Layer        int     `json:"layer"`
	Track        []Point `json:"track"`
	TunnelWidth  float64 `json:"tunnel_inner_diameter_m"`
	TunnelOuter  float64 `json:"tunnel_outer_diameter_m"`
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Stations     []int   `json:"stations"`
}
type View struct {
	HighestLayer       int              `json:"highest_layer"`
	Transferring       int              `json:"transferring"`
	TransfersCompleted int              `json:"transfers_completed"`
	Interchanges       []TransferSite   `json:"interchanges,omitempty"`
	Journeys           []NetworkJourney `json:"journeys,omitempty"`
	CrossingOwner      int              `json:"crossing_owner"`
	Lines              []LineView       `json:"lines"`
	Layer              int              `json:"layer"`
	Config             Config           `json:"config"`
	Time               float64          `json:"time"`
	Stations           []StationView    `json:"stations"`
	Trains             []TrainView      `json:"trains"`
	Blocks             []Block          `json:"blocks"`
	Routes             []Route          `json:"routes"`
	Generated          int              `json:"generated"`
	Waiting            int              `json:"waiting"`
	Onboard            int              `json:"onboard"`
	Completed          int              `json:"completed"`
	Departures         int              `json:"departures"`
}
type train struct {
	control       Control
	controlUntil  float64
	needsLoop     bool
	routeDistance float64
	TrainView
	onboard   []Passenger
	carCount  int
	segment   int
	service   float64
	arrived   float64
	scheduled float64
}
type crossingGate struct{ owner int }
type Runtime struct {
	transferPlans    map[[2]int][]JourneyStep
	travelers        map[int]*NetworkJourney
	transferArrivals []int
	transferCount    int
	transferPairs    [][2]int
	layout           *Layout
	spec             *LineSpec
	reverseChain     []float64
	chain            []float64
	lines            []*Runtime
	gate             *crossingGate
	lineID           int

	config                           Config
	time                             float64
	stations                         []StationView
	trains                           []*train
	queues                           [][2][]Passenger
	segments                         []int
	platforms                        [][2]int
	routes                           []int
	lengths                          []float64
	rng                              *rand.Rand
	nextArrival                      float64
	finishedJourneys                 []int
	externalDemand                   bool
	generated, completed, departures int
}

func New(points []Point, c Config) *Runtime {
	if c.MaxSpeed == 0 {
		c.MaxSpeed = 300
	}
	if c.Cars == 0 {
		c.Cars = 6
	}
	r := &Runtime{config: c, rng: rand.New(rand.NewSource(c.Seed)), nextArrival: math.Inf(1)}
	for i, p := range points {
		r.stations = append(r.stations, StationView{ID: i, Name: fmt.Sprintf("M%d", i+1), Position: p})
	}
	if len(points) < 2 {
		r.config.Enabled = false
		return r
	}
	r.queues = make([][2][]Passenger, len(points))
	r.platforms = make([][2]int, len(points))
	r.routes = make([]int, len(points))
	r.segments = make([]int, len(points)-1)
	for i := 1; i < len(points); i++ {
		r.lengths = append(r.lengths, math.Max(100, math.Hypot(points[i].X-points[i-1].X, points[i].Y-points[i-1].Y)))
	}
	for i, station := range []int{0, len(points) - 1} {
		dir := 1
		if i == 1 {
			dir = -1
		}
		t := &train{TrainView: TrainView{ID: i + 1, Station: station, Next: station + dir, Direction: dir, State: "dwelling", Position: points[station], Length: float64(c.Cars) * 22.8, Capacity: c.Capacity, Departure: c.Dwell}, carCount: c.Cars, segment: -1, scheduled: c.Dwell}
		r.trains = append(r.trains, t)
		r.platforms[station][r.platform(station, dir)] = t.ID
	}
	r.scheduleArrival()
	return r
}
func (r *Runtime) Configure(c Config) error {
	if c.MaxSpeed == 0 {
		c.MaxSpeed = 300
	}
	if c.Cars == 0 {
		c.Cars = 6
	}
	for _, floor := range c.LineLayers {
		if floor >= 0 || floor < -16 || (r.layout != nil && r.layout.HighestLayer < 0 && floor > r.layout.HighestLayer) {
			return fmt.Errorf("metro levels must be -1 to -16")
		}
	}
	if c.Cars < 1 || c.Cars > 12 {
		return fmt.Errorf("cars must be 1-12")
	}
	if math.IsNaN(c.MaxSpeed) || math.IsInf(c.MaxSpeed, 0) || c.MaxSpeed < 10 || c.MaxSpeed > 300 {
		return fmt.Errorf("metro maximum speed must be 10-300 km/h")
	}

	if math.IsNaN(c.Headway) || math.IsNaN(c.Demand) || math.IsNaN(c.Dwell) || math.IsInf(c.Headway, 0) || math.IsInf(c.Demand, 0) || math.IsInf(c.Dwell, 0) || c.Headway < 30 || c.Headway > 900 || c.Demand < 0 || c.Demand > 120 || c.Capacity < 1 || c.Capacity > 1000 || c.Dwell < 5 || c.Dwell > 120 {
		return fmt.Errorf("invalid metro settings")
	}
	for _, t := range r.trains {
		if c.Capacity < len(t.onboard) {
			return fmt.Errorf("capacity cannot be below current occupancy")
		}
	}
	changed := c.Demand != r.config.Demand
	c.Seed = r.config.Seed
	for _, line := range r.lines {
		for _, t := range line.trains {
			if c.Capacity < len(t.onboard) {
				return fmt.Errorf("capacity below occupancy")
			}
		}
	}
	if len(r.lines) > 0 {
		if err := r.configureLayers(c); err != nil {
			return err
		}
	}
	r.config = c
	for _, t := range r.trains {
		t.Capacity = c.Capacity
		r.configureCars(t)
		// Rebalance capacity allocation without changing passenger identity or destination.
		for i := range t.onboard {
			t.onboard[i].Car = i % t.carCount
		}
	}
	for _, line := range r.lines {
		local := c
		local.Demand = c.Demand / float64(len(r.lines))
		_ = line.Configure(local)
	}
	if changed && (len(r.lines) == 0 || r.transferPlans != nil) {
		r.scheduleArrival()
	}
	if r.transferPlans != nil {
		replanned := NewNetwork(r.layout, c)
		r.transferPlans = replanned.transferPlans
		r.transferPairs = replanned.transferPairs
	}
	return nil
}

// Consist changes take effect at an empty terminal; active journeys keep their cars.
func (r *Runtime) configureCars(t *train) {
	if t.State != "moving" && t.State != "turning" && len(t.onboard) == 0 && (t.Station == 0 || t.Station == len(r.stations)-1) {
		t.carCount = r.config.Cars
		t.Length = float64(t.carCount) * r.carLength()
	}
}
func (r *Runtime) platform(st, dir int) int {
	if (r.spec == nil && (st == 0 || st == len(r.stations)-1)) || dir == 1 {
		return 0
	}
	return 1
}
func (r *Runtime) scheduleArrival() {
	r.nextArrival = math.Inf(1)
	if r.config.Demand > 0 {
		r.nextArrival = r.time + r.rng.ExpFloat64()*60/r.config.Demand
	}
}
func (r *Runtime) AddPassenger(origin, destination int) bool {
	return r.AddJourney(origin, destination, 0)
}
func (r *Runtime) AddJourney(origin, destination, journey int) bool {
	if r.transferPlans != nil {
		return r.addNetworkJourney(origin, destination, journey)
	}
	if len(r.lines) > 0 {
		offset := 0
		for _, line := range r.lines {
			n := len(line.stations)
			if origin >= offset && origin < offset+n && destination >= offset && destination < offset+n {
				return line.AddJourney(origin-offset, destination-offset, journey)
			}
			offset += n
		}
		return false
	}

	if origin < 0 || destination < 0 || origin >= len(r.stations) || destination >= len(r.stations) || origin == destination {
		return false
	}
	r.generated++
	d := 0
	if (!r.circular() && destination < origin) || (r.circular() && (origin-destination+len(r.stations))%len(r.stations) < (destination-origin+len(r.stations))%len(r.stations)) {
		d = 1
	}
	r.queues[origin][d] = append(r.queues[origin][d], Passenger{ID: r.generated, Destination: destination, JourneyID: journey})
	return true
}
func (r *Runtime) Advance(dt float64) {
	if len(r.lines) > 0 {
		r.time += dt
		for _, line := range r.lines {
			line.Advance(dt)
		}
		r.advanceTransfers(dt)
		return
	}
	r.time += dt
	if len(r.stations) < 2 {
		return
	}
	if !r.config.Enabled {
		r.nextArrival += dt
		for _, t := range r.trains {
			t.scheduled += dt
			t.arrived += dt
			t.Departure += dt
		}
		return
	}
	for !r.externalDemand && r.time >= r.nextArrival {
		a := r.rng.Intn(len(r.stations))
		b := r.rng.Intn(len(r.stations) - 1)
		if b >= a {
			b++
		}
		r.AddPassenger(a, b)
		r.scheduleArrival()
	}
	for _, t := range r.trains {
		r.configureCars(t)
		if t.State == "turning" {
			r.turn(t, dt)
			continue
		}
		if t.State == "moving" {
			r.move(t, dt)
			continue
		}
		t.service += dt
		for t.service >= .5 {
			t.service -= .5
			idx := -1
			for i, p := range t.onboard {
				if p.Destination == t.Station {
					idx = i
					break
				}
			}
			if idx >= 0 {
				if network := t.onboard[idx].NetworkID; network > 0 {
					r.transferArrivals = append(r.transferArrivals, network)
				} else if id := t.onboard[idx].JourneyID; id > 0 {
					r.finishedJourneys = append(r.finishedJourneys, id)
				}
				t.onboard = append(t.onboard[:idx], t.onboard[idx+1:]...)
				r.completed++
				r.configureCars(t)
				continue
			}
			d := 0
			if t.Direction < 0 {
				d = 1
			}
			q := &r.queues[t.Station][d]
			if len(*q) > 0 && len(t.onboard) < t.Capacity {
				p := (*q)[0]
				counts := make([]int, t.carCount)
				for _, o := range t.onboard {
					counts[o.Car]++
				}
				car := 0
				for i := range counts {
					cap := t.Capacity / t.carCount
					if i < t.Capacity%t.carCount {
						cap++
					}
					if counts[i] < cap {
						car = i
						break
					}
				}
				p.Car = car
				t.onboard = append(t.onboard, p)
				*q = (*q)[1:]
			}
		}
		t.Passengers = len(t.onboard)
		alighting := false
		for _, p := range t.onboard {
			alighting = alighting || p.Destination == t.Station
		}
		t.Departure = math.Max(t.scheduled, t.arrived+r.config.Dwell)
		if r.time < t.controlUntil {
			t.Departure = math.Max(t.Departure, t.control.DepartAfter)
		}
		t.Delay = math.Max(0, r.time-t.Departure)
		d := 0
		if t.Direction < 0 {
			d = 1
		}
		boardingDone := len(r.queues[t.Station][d]) == 0 || len(t.onboard) >= t.Capacity
		if r.time >= t.Departure && !alighting && boardingDone {
			if t.needsLoop {
				exitPlatform := r.platform(t.Station, t.Direction)
				if owner := r.platforms[t.Station][exitPlatform]; owner != 0 && owner != t.ID {
					continue
				}
				r.platforms[t.Station][exitPlatform] = t.ID
				t.State = "turning"
				t.Speed = 0
				t.Distance = 0
				end := r.spec.Stops[t.Station][directionIndex(t.Direction)]
				total := r.chain[len(r.chain)-1]
				t.Authority = math.Mod(end-math.Mod(t.routeDistance, total)+total, total)
				continue
			}
			if !r.reserve(t) {
				t.State = "waiting authority"
			} else {
				t.State = "moving"
				t.Distance = 0
				t.Speed = 0
				t.service = 0
				r.departures++
				t.scheduled = math.Max(t.scheduled+r.config.Headway, r.time)
			}
		}
	}
}
func (r *Runtime) reserve(t *train) bool {
	a, b := t.Station, t.Station+t.Direction
	if r.circular() {
		b = (b + len(r.stations)) % len(r.stations)
	}
	if b < 0 || b >= len(r.stations) {
		return false
	}
	seg := min(a, b)
	if r.circular() {
		seg = a
		if t.Direction < 0 {
			seg = len(r.stations) + b
		}
	}
	p := r.platform(b, t.Direction)
	if r.segments[seg] != 0 || r.platforms[b][p] != 0 || r.routes[a] != 0 || r.routes[b] != 0 {
		return false
	}
	if r.gate != nil && (a == r.crossing() || b == r.crossing()) {
		owner := r.lineID*100 + t.ID
		if r.gate.owner != 0 && r.gate.owner != owner {
			return false
		}
		r.gate.owner = owner
	}
	r.segments[seg] = t.ID
	r.platforms[b][p] = t.ID
	r.routes[a] = t.ID
	r.routes[b] = t.ID
	t.segment = seg
	t.Next = b
	t.Authority = r.lengths[seg]
	return true
}
func (r *Runtime) move(t *train, dt float64) {
	remaining := math.Max(0, t.Authority-t.Distance)
	brake := 1.0
	target := math.Min(r.config.MaxSpeed/3.6, math.Sqrt(brake*brake*dt*dt+2*brake*remaining)-brake*dt)
	if r.time < t.controlUntil {
		target = math.Min(target, t.control.Speed)
	}
	t.Speed = math.Max(0, math.Min(t.Speed+dt, math.Max(t.Speed-brake*dt, target)))
	t.Speed = math.Min(t.Speed, remaining/dt)
	t.Distance += t.Speed * dt
	t.routeDistance += t.Speed * dt
	fraction := math.Min(1, t.Distance/t.Authority)
	a, b := r.stations[t.Station].Position, r.stations[t.Next].Position
	t.Position = Point{a.X + (b.X-a.X)*fraction, a.Y + (b.Y-a.Y)*fraction}
	if r.spec != nil {
		t.Position = r.directionPose(t.Direction, t.routeDistance).Position
	}
	if t.Distance >= t.Length {
		if r.gate != nil && t.Station == r.crossing() && r.gate.owner == r.lineID*100+t.ID {
			r.gate.owner = 0
		}
		if r.platforms[t.Station][r.platform(t.Station, t.Direction)] == t.ID {
			r.platforms[t.Station][r.platform(t.Station, t.Direction)] = 0
		}
		if r.routes[t.Station] == t.ID {
			r.routes[t.Station] = 0
		}
	}
	if remaining > 1e-5 && t.Authority-t.Distance > 1e-5 {
		return
	}
	r.segments[t.segment] = 0
	r.routes[t.Next] = 0
	t.Station = t.Next
	t.Position = r.stations[t.Station].Position
	if r.spec != nil {
		t.routeDistance = r.spec.Stops[t.Station][directionIndex(t.Direction)]
		t.Position = r.directionPose(t.Direction, t.routeDistance).Position
	}
	t.State = "dwelling"
	t.Speed = 0
	t.arrived = r.time
	t.service = 0
	t.segment = -1
	t.Authority = 0
	if !r.circular() && (t.Station == 0 || t.Station == len(r.stations)-1) {
		t.Direction = -t.Direction
		if r.spec != nil {
			t.needsLoop = true
		}
		t.scheduled = math.Ceil((r.time+r.config.Dwell)/r.config.Headway) * r.config.Headway
	} else {
		t.scheduled = r.time + r.config.Dwell
	}
	t.Next = t.Station + t.Direction
	if r.circular() {
		t.Next = (t.Next + len(r.stations)) % len(r.stations)
	}
}
func (r *Runtime) Snapshot() View {
	if len(r.lines) > 0 {
		return r.networkView()
	}
	v := View{Lines: []LineView{}, Layer: -1, Config: r.config, Time: r.time, Stations: append([]StationView(nil), r.stations...), Generated: r.generated, Completed: r.completed, Departures: r.departures}
	if len(r.stations) == 0 {
		return v
	}
	for i := range r.stations {
		n := len(r.queues[i][0]) + len(r.queues[i][1])
		v.Stations[i].Waiting = n
		v.Stations[i].Layer = r.floor()
		v.Waiting += n
		v.Routes = append(v.Routes, Route{i, r.routes[i]})
	}
	for _, t := range r.trains {
		x := t.TrainView
		x.Passengers = len(t.onboard)
		x.Line = r.lineID
		x.Layer = r.floor()
		x.CarLength = r.carLength()
		x.CarWidth = 3
		if r.layout != nil {
			x.CarWidth = r.layout.CarWidth
		}
		if r.spec != nil {
			for i := 0; i < t.carCount; i++ {
				x.CarPoses = append(x.CarPoses, r.directionPose(t.Direction, t.routeDistance+(float64(t.carCount)/2-float64(i)-.5)*r.carLength()))
			}
		}
		x.Cars = make([]int, t.carCount)
		x.CarCapacities = make([]int, t.carCount)
		for i := range x.Cars {
			x.CarCapacities[i] = t.Capacity / t.carCount
			if i < t.Capacity%t.carCount {
				x.CarCapacities[i]++
			}
		}
		for _, p := range t.onboard {
			x.Cars[p.Car]++
		}
		a, b := r.stations[t.Station].Position, r.stations[t.Next].Position
		x.Heading = math.Atan2(b.Y-a.Y, b.X-a.X)
		if r.spec != nil {
			x.Heading = r.directionPose(t.Direction, t.routeDistance).Heading
		}
		v.Trains = append(v.Trains, x)
		v.Onboard += x.Passengers
	}
	for seg, distance := range r.lengths {
		L := distance - float64(r.config.Cars)*r.carLength()
		for _, t := range r.trains {
			if t.segment == seg {
				L = distance - t.Length
			}
		}
		L = math.Max(1, L)
		for b := 0; b < 2; b++ {
			start, end := float64(b)*L/2, float64(b+1)*L/2
			block := Block{ID: fmt.Sprintf("B%d.%d", seg, b), From: seg, To: seg + 1, Start: start, End: end, Owner: r.segments[seg]}
			if r.circular() {
				n := len(r.stations)
				block.From = seg % n
				block.To = (seg + 1) % n
				if seg >= n {
					block.From, block.To = block.To, block.From
				}
			}
			for _, t := range r.trains {
				if t.segment != seg {
					continue
				}
				head, tail := t.Distance, t.Distance-t.Length
				if t.Direction < 0 {
					head, tail = L-t.Distance, L-t.Distance+t.Length
				}
				if math.Max(head, tail) > start && math.Min(head, tail) < end {
					block.Occupied = t.ID
				}
			}
			v.Blocks = append(v.Blocks, block)
		}
	}
	indices := []int{}
	for i := range r.stations {
		indices = append(indices, i)
	}
	v.Lines = []LineView{{ID: r.lineID, Name: fmt.Sprintf("Line %d", r.lineID+1), Stations: indices}}
	if r.spec != nil {
		v.Layer = r.floor()
		v.Lines[0].Name = r.spec.Name
		v.Lines[0].Layer = r.floor()
		v.Lines[0].Track = r.spec.Route
		v.Lines[0].ReverseTrack = r.spec.ReverseRoute
		v.Lines[0].Circular = r.spec.Circular
		v.Lines[0].TunnelWidth = r.layout.TunnelWidth
		v.Lines[0].TunnelOuter = r.layout.TunnelOuter
	}
	return v
}

// UseSharedDemand stops only future local arrivals, preserving all existing riders.
func (r *Runtime) UseSharedDemand() {
	r.externalDemand = true
	for _, line := range r.lines {
		line.UseSharedDemand()
	}
}
func (r *Runtime) DrainCompletedJourneys() []int {
	ids := r.finishedJourneys
	r.finishedJourneys = nil
	for _, line := range r.lines {
		ids = append(ids, line.DrainCompletedJourneys()...)
	}
	return ids
}

// ServedPairs enumerates directed, same-line station pairs in stable order.
func (r *Runtime) ServedPairs() [][2]int {
	if r.transferPlans != nil {
		return append([][2]int(nil), r.transferPairs...)
	}
	pairs := [][2]int{}
	offset := 0
	if len(r.lines) == 0 {
		for a := range r.stations {
			for b := range r.stations {
				if a != b {
					pairs = append(pairs, [2]int{a, b})
				}
			}
		}
		return pairs
	}
	for _, line := range r.lines {
		for _, pair := range line.ServedPairs() {
			pairs = append(pairs, [2]int{pair[0] + offset, pair[1] + offset})
		}
		offset += len(line.stations)
	}
	return pairs
}

func (r *Runtime) ActiveJourneys() map[int]int {
	out := map[int]int{}
	for _, j := range r.travelers {
		if !j.Riding && j.JourneyID > 0 {
			out[j.JourneyID]++
		}
	}
	for _, line := range r.lines {
		for id, n := range line.ActiveJourneys() {
			out[id] += n
		}
	}
	for _, q := range r.queues {
		for _, direction := range q {
			for _, p := range direction {
				if p.JourneyID > 0 {
					out[p.JourneyID]++
				}
			}
		}
	}
	for _, t := range r.trains {
		for _, p := range t.onboard {
			if p.JourneyID > 0 {
				out[p.JourneyID]++
			}
		}
	}
	return out
}
