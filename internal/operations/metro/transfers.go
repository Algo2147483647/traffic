package metro

import "sort"

// StartStep advances an operational itinerary without knowing trains, tracks or locks.
// Admission is supplied by the train executor after applying capacity/platform rules.
func StartStep(j *NetworkJourney, now float64, admit func(JourneyStep) bool) bool {
	if j.Index == len(j.Plan) {
		return true
	}
	step := j.Plan[j.Index]
	j.Riding = false
	if step.Line < 0 {
		j.Ready = now + step.Seconds
		return false
	}
	if !admit(step) {
		panic("validated metro itinerary failed admission")
	}
	j.Riding = true
	j.Ready = 0
	return false
}
func ReadyTransfers(journeys map[int]*NetworkJourney, now float64) []int {
	ids := []int{}
	for id, j := range journeys {
		if !j.Riding && now >= j.Ready {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}
