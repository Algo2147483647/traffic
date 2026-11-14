package sim

import (
	"math"
	"testing"
)

func TestAllInterchangeRoutesMaintainUnobstructedCruise(t *testing.T) {
	g := taxiGraph(t)
	for _, group := range g.Interchanges {
		for _, route := range group.Routes {
			t.Run(group.ID+"/"+route.Origin+"_"+route.Destination, func(t *testing.T) {
				s := New(g)
				s.spawnRate = 0
				first, last := g.Ways[route.WayIDs[0]], g.Ways[route.WayIDs[len(route.WayIDs)-1]]
				speed := g.Simulation.DriveSpeed
				for _, id := range route.WayIDs {
					speed = math.Min(speed, g.Ways[id].SpeedLimit)
				}
				lane := s.laneForMovement(first.ID, route.WayIDs[1], 1)
				p := &personState{id: 1, state: "driving", wayID: first.ID, route: route.WayIDs, lane: lane, lanePosition: float64(lane), along: 25, speed: speed, destinationAlong: last.Length - 30}
				s.people = []*personState{p}
				crossings := 0
				index := 0
				for i := 0; i < 24000 && p.routeIndex < len(p.route)-1; i++ {
					s.Step(.05)
					if p.routeIndex > index {
						crossings++
						index = p.routeIndex
					}
					if p.speed < speed-.01 && p.routeIndex < len(p.route)-1 {
						t.Fatalf("unobstructed slowdown at %s (%s): %.3f, expected >= %.3f", p.wayID, p.state, p.speed, speed)
					}
				}
				if crossings != len(p.route)-1 {
					t.Fatalf("only %d crossings", crossings)
				}
			})
		}
	}
}
func TestReservationProtectsEntryAndReleasesAfterRouteChange(t *testing.T) {
	g := taxiGraph(t)
	s := New(g)
	route := g.Interchanges[0].Routes[0].WayIDs
	first := g.Ways[route[0]]
	lane := s.laneForMovement(first.ID, route[1], 1)
	p := &personState{id: 1, state: "driving", wayID: first.ID, route: route, lane: lane, along: s.stopLine(first) - 10, speed: 10}
	s.people = []*personState{p}
	s.reserveFlowLocked()
	r := s.flowReservation(p)
	if r == nil {
		t.Fatal("no advance reservation")
	}
	if s.laneEntryAvailable(r.to, r.lane, r.along, nil) {
		t.Fatal("reserved entry allowed insertion")
	}
	p.route = []string{first.ID}
	s.reserveFlowLocked()
	if len(s.reservations) != 0 {
		t.Fatal("route change leaked reservation")
	}
}

func TestFlowMergeQueueSafetyAndRelease(t *testing.T) {
	g := taxiGraph(t)
	s := New(g)
	s.spawnRate = 0
	var from []string
	to := ""
	for _, node := range g.SortedIntersections() {
		if node.TrafficControl.Mode != "flow" {
			continue
		}
		var incoming, outgoing []string
		for _, w := range g.SortedWays() {
			if w.To == node.ID {
				incoming = append(incoming, w.ID)
			}
			if w.From == node.ID {
				outgoing = append(outgoing, w.ID)
			}
		}
		if len(incoming) == 2 && len(outgoing) == 1 && g.Ways[outgoing[0]].Length > 200 {
			from, to = incoming, outgoing[0]
			break
		}
	}
	if to == "" {
		t.Fatal("missing merge fixture")
	}
	next := g.Ways[to]
	entry := math.Min(g.Simulation.Intersection, next.Length)
	for i, id := range from {
		w := g.Ways[id]
		lane := s.laneForMovement(id, to, i+1)
		p := &personState{id: i + 1, state: "driving", wayID: id, route: []string{id, to}, lane: lane, lanePosition: float64(lane), along: s.stopLine(w) - 25, speed: 8, destinationAlong: next.Length - 30}
		s.people = append(s.people, p)
	}
	s.people = s.people[len(s.people)-2:]
	for _, p := range s.people {
		m, ok := s.signalMovement(g.Ways[p.wayID].To, p.wayID, to, p.lane)
		if !ok {
			t.Fatal("movement")
		}
		p.destinationLane = m.TargetLanes(p.lane)[0]
	}
	// Occupy each receiving lane so storage remains blocked until the barrier moves.
	blockers := []*personState{}
	for lane := 0; lane < next.Lanes; lane++ {
		b := &personState{id: 10 + lane, state: "waiting", wayID: to, route: []string{to}, lane: lane, lanePosition: float64(lane), along: entry + 20, destinationAlong: entry + 20}
		blockers = append(blockers, b)
		s.people = append(s.people, b)
	}
	for step := 0; step < 2400; step++ {
		if step == 300 {
			for _, b := range blockers {
				b.destinationAlong = next.Length - 20
			}
		}
		s.reserveFlowLocked()
		s.advanceRoadVehiclesLocked(.05)
		for _, p := range s.people {
			if p.state == "turning" {
				s.advanceTurning(p, .05)
			}
		}
		for _, p := range s.people[:2] {
			if (p.state == "driving" || p.state == "waiting") && p.wayID != to && p.along >= s.stopLine(g.Ways[p.wayID])-.0001 {
				s.transition(p)
			}
		}
		turning := 0
		for _, p := range s.people[:2] {
			if p.state == "turning" {
				turning++
			}
		}
		if turning > 1 {
			t.Fatal("merge admitted conflicting connectors")
		}
		for i, a := range s.people {
			for _, b := range s.people[i+1:] {
				if a.wayID == b.wayID && a.state != "turning" && b.state != "turning" && a.lane == b.lane && math.Abs(a.along-b.along) < s.minimumStoppedVehicleSpacing()-1e-6 {
					t.Fatalf("overlap: %d %d", a.id, b.id)
				}
			}
		}
		s.clock.Time += .05
	}
	for _, p := range s.people[:2] {
		if p.wayID != to {
			t.Fatalf("merge failed to release: %d %s %.2f", p.id, p.state, p.along)
		}
	}
}
