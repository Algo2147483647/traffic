package sim

import (
	"strings"
	"testing"
	"traffic/internal/foundation"
)

func TestCommandsRunAtBoundaryAndRejectStaleWorld(t *testing.T) {
	s := New(simulationGraph(t))
	stamp := s.Snapshot().Kernel
	_, e := s.QueueCommand(foundation.Envelope[Command]{Stamp: stamp, Due: 1, Payload: Command{Kind: "set_spacing", Value: 8}})
	if e != nil {
		t.Fatal(e)
	}
	s.Step(.05)
	if s.safeGap == 8 {
		t.Fatal("command committed early")
	}
	s.Step(.05)
	if s.safeGap != 8 {
		t.Fatal("command not consumed")
	}
	old := s.stamp
	s.SetSpacing(9)
	_, e = s.QueueCommand(foundation.Envelope[Command]{Stamp: old, Due: s.clock.Tick, StrictWorld: true, Payload: Command{Kind: "set_spacing", Value: 10}})
	if e == nil {
		t.Fatal("stale world accepted")
	}
	s.Reset(1)
	if s.stamp.Run == stamp.Run {
		t.Fatal("reset reused run identity")
	}
	_, e = s.QueueCommand(foundation.Envelope[Command]{Stamp: stamp, Payload: Command{Kind: "pause"}})
	if e == nil {
		t.Fatal("stale run accepted")
	}
	before := s.stamp
	s.ReplaceGraph(simulationGraph(t))
	if s.stamp.Topology <= before.Topology {
		t.Fatal("graph revision unchanged")
	}
}
func TestPausedCommandsAndResetInvalidateSameBatch(t *testing.T) {
	s := New(simulationGraph(t))
	s.SetPaused(true)
	stamp := s.stamp
	for _, kind := range []string{"reset", "set_spacing"} {
		_, err := s.QueueCommand(foundation.Envelope[Command]{Stamp: stamp, Due: s.clock.Tick, Payload: Command{Kind: kind, Population: 1, Value: 9}})
		if err != nil {
			t.Fatal(err)
		}
	}
	s.Tick(.05)
	if s.safeGap == 9 {
		t.Fatal("old-run command survived reset")
	}
	found := false
	for _, e := range s.Events() {
		found = found || strings.Contains(e.State, "stale command")
	}
	if !found {
		t.Fatal("missing rejection event")
	}
}
func TestStaleArrivalCannotCompleteReusedEntity(t *testing.T) {
	s := New(simulationGraph(t))
	old := s.stamp
	s.Reset(1)
	p := s.people[0]
	state := p.state
	s.arrivalsQueue.Push(foundation.Envelope[Arrival]{Stamp: old, Entity: foundation.EntityID(p.id), Payload: Arrival{Kind: "destination", Way: p.wayID, RouteIndex: p.routeIndex}})
	s.commitArrivalsLocked()
	if p.state != state {
		t.Fatal("stale arrival mutated reused entity")
	}
}

func TestScheduledPauseStopsLongAdvance(t *testing.T) {
	s := New(simulationGraph(t))
	_, err := s.QueueCommand(foundation.Envelope[Command]{Stamp: s.stamp, Due: 2, Payload: Command{Kind: "pause"}})
	if err != nil {
		t.Fatal(err)
	}
	s.Step(1)
	if s.clock.Tick != 2 || !s.clock.Paused {
		t.Fatalf("pause boundary: %+v", s.clock)
	}
}
func TestTaxiSnapshotAndCommandParametersAreDetached(t *testing.T) {
	s := New(taxiGraph(t))
	c := DefaultTaxiConfig()
	c.Fleet = 1
	c.Parameters = map[string]float64{"wait_weight": .25}
	if err := s.SetMode("taxi", c); err != nil {
		t.Fatal(err)
	}
	c.Parameters["wait_weight"] = 99
	snap := s.Snapshot()
	snap.Taxi.Config.Parameters["wait_weight"] = 88
	if s.taxi.config.Parameters["wait_weight"] != .25 {
		t.Fatal("mutable input or snapshot leaked into state")
	}
}
