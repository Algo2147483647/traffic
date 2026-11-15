package metroscheduling

import (
	"math"
	"math/rand"
	"sort"
	metroops "traffic/internal/operations/metro"
)

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
		edges[a] = append(edges[a], JourneyStep{Line: -1, From: a, To: b, Seconds: t.Seconds})
		edges[b] = append(edges[b], JourneyStep{Line: -1, From: b, To: a, Seconds: t.Seconds})
		for _, id := range []int{a, b} {
			line, local := r.stationOwner(id)
			line.stations[local].Transfer = t.ID
			line.stations[local].Name = t.Name
		}
	}
	r.transferPlans, r.transferPairs = metroops.PlanAll(offset, edges)
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
	if metroops.StartStep(j, r.time, func(step JourneyStep) bool {
		line, origin := r.stationOwner(step.From)
		_, dest := r.stationOwner(step.To)
		if !line.AddJourney(origin, dest, j.JourneyID) {
			return false
		}
		for d := 0; d < 2; d++ {
			q := line.queues[origin][d]
			if len(q) > 0 && q[len(q)-1].ID == line.generated {
				q[len(q)-1].NetworkID = j.ID
				break
			}
		}
		return true
	}) {
		r.completed++
		if j.JourneyID > 0 {
			r.finishedJourneys = append(r.finishedJourneys, j.JourneyID)
		}
		delete(r.travelers, j.ID)
	}
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
	for _, id := range metroops.ReadyTransfers(r.travelers, r.time) {
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
		v.Interchanges = append(v.Interchanges, TransferSite{ID: t.ID, Name: t.Name, Stations: [2]int{a, b}, Position: line.stations[local].Position, Seconds: t.Seconds})
	}
}
