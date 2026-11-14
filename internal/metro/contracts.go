// Package metro includes the running metro executor and future strategy contracts.
package metro

import "context"

type Train struct {
	ID, Section          string
	Capacity, Passengers int
}
type Input struct {
	Time             float64
	Trains           []Train
	OccupiedSections map[string]string
}
type Departure struct {
	TrainID, Section string
	At               float64
}
type Timetable interface {
	Schedule(context.Context, Input) ([]Departure, error)
}
type Interlocking interface {
	Permit(context.Context, Input, Departure) (bool, error)
}
type Operation interface {
	Advance(context.Context, Input, float64) ([]Train, error)
}

// Boarding is constrained by actual remaining capacity regardless of policy output.
func Board(train Train, waiting int) (Train, int) {
	if waiting < 0 {
		waiting = 0
	}
	space := max(0, train.Capacity-train.Passengers)
	count := min(space, waiting)
	train.Passengers += count
	return train, count
}
