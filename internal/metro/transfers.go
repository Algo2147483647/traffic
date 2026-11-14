package metro

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

type TransferSpec struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	A       [2]int  `json:"a"`
	B       [2]int  `json:"b"`
	Seconds float64 `json:"seconds"`
}
type TransferSite struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Stations [2]int  `json:"stations"`
	Position Point   `json:"position"`
	Seconds  float64 `json:"walk_seconds"`
}
type JourneyStep struct {
	Line    int     `json:"line"`
	From    int     `json:"from"`
	To      int     `json:"to"`
	Seconds float64 `json:"estimated_seconds"`
}
type NetworkJourney struct {
	State       string        `json:"state"`
	ID          int           `json:"id"`
	JourneyID   int           `json:"journey_id"`
	Origin      int           `json:"origin"`
	Destination int           `json:"destination"`
	Plan        []JourneyStep `json:"plan"`
	Index       int           `json:"step"`
	Ready       float64       `json:"ready_at"`
	Riding      bool          `json:"leg_active"`
}

func (l *Layout) validateTransfers() error {
	ids := map[string]bool{}
	edges := map[[4]int]bool{}
	point := func(p [2]int) (Point, bool) {
		for _, line := range l.Lines {
			if line.ID == p[0] && p[1] >= 0 && p[1] < len(line.Stations) {
				return line.Stations[p[1]], true
			}
		}
		return Point{}, false
	}
	for _, t := range l.Transfers {
		a, ok := point(t.A)
		b, bok := point(t.B)
		key := [4]int{t.A[0], t.A[1], t.B[0], t.B[1]}
		reverse := [4]int{t.B[0], t.B[1], t.A[0], t.A[1]}
		if t.ID == "" || ids[t.ID] || edges[key] || edges[reverse] || !ok || !bok || t.A[0] == t.B[0] || math.Hypot(a.X-b.X, a.Y-b.Y) > 1 || math.IsNaN(t.Seconds) || math.IsInf(t.Seconds, 0) || t.Seconds < 1 || t.Seconds > 1800 {
			return fmt.Errorf("invalid transfer %s", t.ID)
		}
		ids[t.ID] = true
		edges[key] = true
	}
	return nil
}
func (r *Runtime) globalStation(ref [2]int) int {
	offset := 0
	for _, line := range r.lines {
		if line.lineID == ref[0] {
			return offset + ref[1]
		}
		offset += len(line.stations)
	}
	return -1
}
func (r *Runtime) stationOwner(id int) (*Runtime, int) {
	offset := 0
	for _, line := range r.lines {
		if id >= offset && id < offset+len(line.stations) {
			return line, id - offset
		}
		offset += len(line.stations)
	}
	return nil, -1
}
func (r *Runtime) initTransfers() {
	if r.layout == nil || len(r.layout.Transfers) == 0 {
		return
	}
	r.transferPlans = map[[2]int][]JourneyStep{}
	r.travelers = map[int]*NetworkJourney{}
	r.rng = rand.New(rand.NewSource(r.config.Seed))
	r.scheduleArrival()
	edges := map[int][]JourneyStep{}
	offset := 0
	for _, line := range r.lines {
		line.UseSharedDemand()
		n := len(line.stations)
		for a := 0; a < n; a++ {
			for b := 0; b < n; b++ {
				if a == b {
					continue
				}
				distance := math.Abs(line.spec.Stops[b][0] - line.spec.Stops[a][0])
				stops := int(math.Abs(float64(b - a)))
				if line.circular() {
					total := line.chain[len(line.chain)-1]
					distance = math.Min(distance, total-distance)
					stops = min(stops, n-stops)
				}
				seconds := distance/(line.config.MaxSpeed/3.6) + float64(stops)*line.config.Dwell + line.config.Headway/2
				edges[offset+a] = append(edges[offset+a], JourneyStep{Line: line.lineID, From: offset + a, To: offset + b, Seconds: seconds})
			}
		}
		offset += n
	}
	for _, t := range r.layout.Transfers {
		a, b := r.globalStation(t.A), r.globalStation(t.B)
		edges[a] = append(edges[a], JourneyStep{-1, a, b, t.Seconds})
		edges[b] = append(edges[b], JourneyStep{-1, b, a, t.Seconds})
		for _, id := range []int{a, b} {
			line, local := r.stationOwner(id)
			line.stations[local].Transfer = t.ID
			line.stations[local].Name = t.Name
		}
	}
	for source := 0; source < offset; source++ {
		distance := make([]float64, offset)
		visited := make([]bool, offset)
		previous := make([]JourneyStep, offset)
		for i := range distance {
			distance[i] = math.Inf(1)
		}
		distance[source] = 0
		for k := 0; k < offset; k++ {
			at := -1
			for i := range distance {
				if !visited[i] && (at < 0 || distance[i] < distance[at]) {
					at = i
				}
			}
			if at < 0 || math.IsInf(distance[at], 1) {
				break
			}
			visited[at] = true
			for _, edge := range edges[at] {
				v := distance[at] + edge.Seconds
				if v < distance[edge.To]-1e-9 {
					distance[edge.To] = v
					previous[edge.To] = edge
				}
			}
		}
		for dest := 0; dest < offset; dest++ {
			if dest == source || math.IsInf(distance[dest], 1) {
				continue
			}
			path := []JourneyStep{}
			at := dest
			for at != source {
				e := previous[at]
				path = append(path, e)
				at = e.From
			}
			for a, b := 0, len(path)-1; a < b; a, b = a+1, b-1 {
				path[a], path[b] = path[b], path[a]
			}
			r.transferPlans[[2]int{source, dest}] = path
			r.transferPairs = append(r.transferPairs, [2]int{source, dest})
		}
	}
}

// PlanJourney returns a detached station-level itinerary, including timed transfers.
func (r *Runtime) PlanJourney(origin, destination int) []JourneyStep {
	return append([]JourneyStep(nil), r.transferPlans[[2]int{origin, destination}]...)
}
func (r *Runtime) addNetworkJourney(origin, destination, journey int) bool {
	plan := r.PlanJourney(origin, destination)
	if len(plan) == 0 {
		return false
	}
	r.generated++
	j := &NetworkJourney{ID: r.generated, JourneyID: journey, Origin: origin, Destination: destination, Plan: plan}
	r.travelers[j.ID] = j
	r.startJourneyStep(j)
	return true
}
func (r *Runtime) startJourneyStep(j *NetworkJourney) {
	if j.Index == len(j.Plan) {
		r.completed++
		if j.JourneyID > 0 {
			r.finishedJourneys = append(r.finishedJourneys, j.JourneyID)
		}
		delete(r.travelers, j.ID)
		return
	}
	step := j.Plan[j.Index]
	j.Riding = false
	if step.Line < 0 {
		j.Ready = r.time + step.Seconds
		return
	}
	line, origin := r.stationOwner(step.From)
	_, dest := r.stationOwner(step.To)
	if !line.AddJourney(origin, dest, j.JourneyID) {
		panic("validated metro itinerary failed admission")
	}
	for d := 0; d < 2; d++ {
		q := line.queues[origin][d]
		if len(q) > 0 && q[len(q)-1].ID == line.generated {
			q[len(q)-1].NetworkID = j.ID
			break
		}
	}
	j.Riding = true
	j.Ready = 0
}
func (r *Runtime) advanceTransfers(dt float64) {
	if r.transferPlans == nil {
		return
	}
	if !r.config.Enabled {
		r.nextArrival += dt
		for _, j := range r.travelers {
			if !j.Riding {
				j.Ready += dt
			}
		}
		return
	}
	if !r.externalDemand && len(r.transferPairs) > 0 {
		for r.time >= r.nextArrival {
			pair := r.transferPairs[r.rng.Intn(len(r.transferPairs))]
			r.AddJourney(pair[0], pair[1], 0)
			r.scheduleArrival()
		}
	}
	for _, line := range r.lines {
		for _, id := range line.transferArrivals {
			j := r.travelers[id]
			if j == nil {
				continue
			}
			j.Index++
			r.startJourneyStep(j)
		}
		line.transferArrivals = nil
	}
	ids := []int{}
	for id, j := range r.travelers {
		if !j.Riding && r.time >= j.Ready {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	for _, id := range ids {
		j := r.travelers[id]
		j.Index++
		r.transferCount++
		r.startJourneyStep(j)
	}
}
func (r *Runtime) transferView(v *View) {
	if r.transferPlans == nil {
		return
	}
	v.Generated = r.generated
	v.Completed = r.completed
	v.TransfersCompleted = r.transferCount
	onboard := map[int]bool{}
	for _, line := range r.lines {
		for _, t := range line.trains {
			for _, p := range t.onboard {
				onboard[p.NetworkID] = true
			}
		}
	}
	ids := []int{}
	for id := range r.travelers {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		j := r.travelers[id]
		copy := *j
		copy.State = "waiting"
		if !j.Riding {
			copy.State = "transferring"
		} else if onboard[j.ID] {
			copy.State = "onboard"
		}
		copy.Plan = append([]JourneyStep(nil), j.Plan...)
		v.Journeys = append(v.Journeys, copy)
		if !j.Riding {
			v.Transferring++
			v.Waiting++
		}
	}
	for _, t := range r.layout.Transfers {
		a, b := r.globalStation(t.A), r.globalStation(t.B)
		line, local := r.stationOwner(a)
		v.Interchanges = append(v.Interchanges, TransferSite{t.ID, t.Name, [2]int{a, b}, line.stations[local].Position, t.Seconds})
	}
}
