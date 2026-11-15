package simulation

import road "traffic/internal/road_scheduling"

// World owns cross-mode order. Signals advance before road motion; rail completion
// is committed to the same demand ledger before publishing a boundary snapshot.
func (w *Simulation) advanceSubstep(dt float64, p road.TickPhases) {
	if !p.Prepare() {
		return
	}
	p.DemandAndSignals()
	w.clock.Time += dt
	p.MotionAndArrivals()
	w.metro.Advance(dt)
	for _, id := range w.metro.DrainCompletedJourneys() {
		p.CompleteJourney(id)
	}
	w.clock.Tick++
 p.Commit()
 if w.radio!=nil&&w.radio.Due(w.clock.Time){s:=p.Observe();w.radio.Advance(s.Kernel,s.Time,mobility(s))}
}
