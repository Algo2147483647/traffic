package foundation

// Clock is advanced only by the owning simulation, never by a renderer or policy.
type Clock struct {
	Time   float64
	Tick   uint64
	Paused bool
	Rate   float64
}
type Event struct {
	Sequence uint64  `json:"sequence"`
	Time     float64 `json:"time_s"`
	Kind     string  `json:"kind"`
	Entity   int     `json:"entity_id"`
	State    string  `json:"state"`
}
type Journal struct {
	sequence uint64
	events   []Event
}

func (j *Journal) Record(at float64, kind string, id int, state string) {
	j.sequence++
	j.events = append(j.events, Event{j.sequence, at, kind, id, state})
	if len(j.events) > 2048 {
		copy(j.events, j.events[len(j.events)-2048:])
		j.events = j.events[:2048]
	}
}
func (j *Journal) Events() []Event { return append([]Event(nil), j.events...) }
