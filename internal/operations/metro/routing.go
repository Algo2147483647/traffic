package metro

import "math"

type JourneyStep struct {
	Line    int     `json:"line"`
	From    int     `json:"from"`
	To      int     `json:"to"`
	Seconds float64 `json:"estimated_seconds"`
}
type NetworkJourney struct {
	Position    Point         `json:"position"`
	Elevation   float64       `json:"elevation_m"`
	Train       int           `json:"train_id,omitempty"`
	Car         int           `json:"car_index"`
	Speed       float64       `json:"speed_mps"`
	Heading     float64       `json:"heading_rad"`
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

func PlanAll(offset int, edges map[int][]JourneyStep) (map[[2]int][]JourneyStep, [][2]int) {
	plans := map[[2]int][]JourneyStep{}
	pairs := [][2]int{}
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
			plans[[2]int{source, dest}] = path
			pairs = append(pairs, [2]int{source, dest})
		}
	}
	return plans, pairs
}
