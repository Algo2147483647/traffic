package foundation

import "sort"

type RunID uint64
type EntityID int
type Revision uint64
type Stamp struct {
	Run      RunID    `json:"run"`
	World    Revision `json:"world"`
	Topology Revision `json:"topology"`
}
type EntityRef struct {
	Run RunID    `json:"run"`
	ID  EntityID `json:"id"`
}
type Envelope[T any] struct {
	Sequence    uint64   `json:"sequence"`
	Due         uint64   `json:"due_tick"`
	Priority    int      `json:"priority"`
	Entity      EntityID `json:"entity"`
	Stamp       Stamp    `json:"stamp"`
	StrictWorld bool     `json:"strict_world"`
	Payload     T        `json:"payload"`
}

func (e Envelope[T]) Valid(s Stamp) bool {
	return e.Stamp.Run == s.Run && e.Stamp.Topology == s.Topology && (!e.StrictWorld || e.Stamp.World == s.World)
}

// Queue is owned by the simulation lock; producers cannot mutate live state.
// Stable ordering is due tick, priority, entity identity, then submission order.
type Queue[T any] struct {
	sequence uint64
	pending  []Envelope[T]
}

func (q *Queue[T]) Push(e Envelope[T]) uint64 {
	q.sequence++
	e.Sequence = q.sequence
	q.pending = append(q.pending, e)
	return e.Sequence
}
func (q *Queue[T]) Drain(tick uint64) []Envelope[T] {
	var ready []Envelope[T]
	keep := q.pending[:0]
	for _, e := range q.pending {
		if e.Due <= tick {
			ready = append(ready, e)
		} else {
			keep = append(keep, e)
		}
	}
	q.pending = keep
	sort.Slice(ready, func(i, j int) bool {
		a, b := ready[i], ready[j]
		if a.Due != b.Due {
			return a.Due < b.Due
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.Entity != b.Entity {
			return a.Entity < b.Entity
		}
		return a.Sequence < b.Sequence
	})
	return ready
}
func (q *Queue[T]) Clear() { q.pending = nil }

func (q *Queue[T]) Len() int               { return len(q.pending) }
func (q *Queue[T]) Discard() []Envelope[T] { p := q.pending; q.pending = nil; return p }
