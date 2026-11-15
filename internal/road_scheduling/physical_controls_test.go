package roadscheduling

import (
	"math"
	"testing"
	model "traffic/internal/infrastructure/road"
)

func TestWalkingSpeedControl(t *testing.T) {
	s := New(simulationGraph(t))
	if e := s.Execute(Command{Kind: "set_walk_speed", Value: 1.8}); e != nil {
		t.Fatal(e)
	}
	p := &personState{state: "walking_to_destination", position: model.Vec2{}, walkTarget: model.Vec2{X: 100}}
	s.advanceWalking(p, .5)
	if math.Abs(p.position.X-.9) > 1e-8 {
		t.Fatal(p.position)
	}
	if e := s.Execute(Command{Kind: "set_walk_speed", Value: 50}); e != nil {
		t.Fatal("finite experimental speed was rejected", e)
	}
	for _, v := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if e := s.Execute(Command{Kind: "set_walk_speed", Value: v}); e == nil {
			t.Fatal("invalid walking speed", v)
		}
	}
}
func TestSpeedLimitBelongsToRoad(t *testing.T) {
	g := simulationGraph(t)
	g.Simulation.DriveSpeed = 1
	g.Ways["AB"].SpeedLimit = 3
	s := New(g)
	p := &personState{id: 1, state: "driving", wayID: "AB", route: []string{"AB"}, destinationAlong: 90}
	s.people = []*personState{p}
	for i := 0; i < 80; i++ {
		s.advanceRoadVehiclesLocked(.05)
	}
	if math.Abs(p.speed-3) > 1e-8 {
		t.Fatal("legacy global cap still applied", p.speed)
	}
}
