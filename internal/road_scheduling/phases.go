package roadscheduling

// TickPhases grants a world coordinator ordered execution, not mutable road state.
// All closures run under the road command/snapshot lock.
type TickPhases struct {
	Prepare           func() bool
	DemandAndSignals  func()
	MotionAndArrivals func()
	CompleteJourney   func(int)
	Commit            func()
}
