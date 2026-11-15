package roadscheduling

import metro "traffic/internal/operations/metro"

// MetroPort exposes cross-mode commands and detached observations; road code
// cannot own tracks, train state, platform locks or construct metro runtimes.
type MetroPort interface {
	Configure(metro.Config) error
	Snapshot() metro.View
	Advance(float64)
	DrainCompletedJourneys() []int
	UseSharedDemand()
	ServedPairs() [][2]int
	AddJourney(int, int, int) bool
	ActiveJourneys() map[int]int
	ApplyControls([]metro.Control) error
	ClearControls()
}
