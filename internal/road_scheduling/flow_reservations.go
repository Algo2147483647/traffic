package roadscheduling

import (
	"math"
	"sort"
	"traffic/internal/foundation"
	"traffic/internal/road_scheduling/motion"
)

// A reservation owns the connector and receiving-lane entry corridor until the
// vehicle has entered the road. It is invalidated by route/run/topology changes.
type flowReservation struct {
	stamp                 foundation.Stamp
	node, from, to        string
	index, fromLane, lane int
	along, speed          float64
}

func (s *Simulation) flowReservation(p *personState) *flowReservation {
	r := s.reservations[p.id]
	if r == nil || r.stamp.Run != s.stamp.Run || r.stamp.Topology != s.stamp.Topology {
		return nil
	}
	if p.state == "turning" {
		if p.nextWayID == r.to && p.nextLane == r.lane {
			return r
		}
		return nil
	}
	if p.wayID != r.from || p.routeIndex != r.index || p.lane != r.fromLane || p.laneChanging || p.routeIndex+1 >= len(p.route) || p.route[p.routeIndex+1] != r.to {
		return nil
	}
	return r
}
func (s *Simulation) reservedEntry(way string, lane int, ignore *personState) bool {
	for id, r := range s.reservations {
		if ignore != nil && id == ignore.id {
			continue
		}
		if r.to == way && r.lane == lane {
			return true
		}
	}
	return false
}
func (s *Simulation) reserveFlowLocked() {
	alive := map[int]*personState{}
	for _, p := range s.people {
		alive[p.id] = p
	}
	for id := range s.reservations {
		p := alive[id]
		if p == nil || s.flowReservation(p) == nil {
			delete(s.reservations, id)
		}
	}
	candidates := append([]*personState(nil), s.people...)
	// Stable nearest-to-entry priority avoids dependence on slice/map iteration.
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if s.clock.Tick < s.algorithm.motionUntil {
			pa, pb := s.algorithm.motion[a.id].MergePriority, s.algorithm.motion[b.id].MergePriority
			if pa != pb {
				return pa > pb
			}
		}
		wa, wb := s.graph.Ways[a.wayID], s.graph.Ways[b.wayID]
		da, db := math.Inf(1), math.Inf(1)
		if wa != nil {
			da = s.stopLine(wa) - a.along
		}
		if wb != nil {
			db = s.stopLine(wb) - b.along
		}
		if math.Abs(da-db) > 1e-9 {
			return da < db
		}
		return a.id < b.id
	})
	for _, p := range candidates {
		if p.state != "driving" && p.state != "waiting" || p.laneChanging || s.flowReservation(p) != nil || p.routeIndex+1 >= len(p.route) {
			continue
		}
		w := s.graph.Ways[p.wayID]
		if w == nil || s.graph.Intersections[w.To].TrafficControl.Mode != "flow" {
			continue
		}
		// Request before comfortable braking would otherwise begin, including a
		// full update margin. A stopped contender also retries without starvation.
		look := math.Max(30, p.speed*p.speed/(2*motion.ComfortableBraking)+p.speed*.1+5)
		if s.stopLine(w)-p.along > look {
			continue
		}
		if leader, _ := s.nearestLeader(p, p.lane); leader != nil {
			continue
		}
		next := s.graph.Ways[p.route[p.routeIndex+1]]
		movement, ok := s.signalMovement(w.To, w.ID, next.ID, p.lane)
		if !ok {
			continue
		}
		continuation := len(s.graph.Incoming[w.To]) == 2 && len(s.graph.Outgoing[w.To]) == 2
		blocked := false
		for id, r := range s.reservations {
			if id != p.id && ((r.node == w.To && !continuation) || (r.to == next.ID)) {
				blocked = true
				break
			}
		}
		// Turning signal-controlled traffic also owns its receiving lane/node.
		for _, other := range s.people {
			if other != p && other.state == "turning" && s.graph.Ways[other.wayID].To == w.To && (!continuation || other.nextWayID == next.ID) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		allowed := s.routeCompatibleReceivingLanes(p, next, p.routeIndex+1, movement.TargetLanes(p.lane))
		along := math.Min(s.graph.Simulation.Intersection, next.Length)
		if p.routeIndex+1 == len(p.route)-1 {
			along = math.Min(along, p.destinationAlong)
		}
		lane, ok := s.selectReceivingLane(p, next, allowed, p.lane, along)
		if !ok {
			continue
		}
		// Keep enough downstream storage for a full stop after entering. This
		// corridor is also protected against admission and lane changes.
		limit := math.Min(w.SpeedLimit, next.SpeedLimit)
		r := &flowReservation{stamp: s.stamp, node: w.To, from: w.ID, to: next.ID, index: p.routeIndex, fromLane: p.lane, lane: lane, along: along, speed: limit}
		s.reservations[p.id] = r
	}
}

// downstreamSpeed anticipates a stopped leader or the next road's terminal stop.
// It is re-evaluated each tick; an owned reservation itself is never revoked
// merely because another contender arrives at the same merge.
func (s *Simulation) downstreamSpeed(p *personState, r *flowReservation, remaining float64, dt float64) float64 {
	next := s.graph.Ways[r.to]
	end := s.stopLine(next)
	if r.index+1 == len(p.route)-1 {
		end = p.destinationAlong
	}
	room := math.Max(0, end-r.along)
	limit := math.Min(r.speed, motion.StoppingSpeed(remaining+room, 0, motion.ComfortableBraking, dt))
	for _, other := range s.people {
		if other == p || other.wayID != r.to || !occupiesLane(other, r.lane) || (other.state != "driving" && other.state != "waiting") {
			continue
		}
		gap := other.along - r.along - s.minimumVehicleSpacing()
		if gap < 0 {
			limit = 0
			continue
		}
		limit = math.Min(limit, motion.StoppingSpeed(remaining+gap, other.speed, motion.ComfortableBraking, dt))
	}
	return limit
}

// Hard distance bound is independent of desired-speed and comfort policies.
func (s *Simulation) downstreamSafety(p *personState, r *flowReservation, remaining, dt float64) float64 {
	bound := math.Inf(1)
	for _, other := range s.people {
		if other == p || other.wayID != r.to || !occupiesLane(other, r.lane) || (other.state != "driving" && other.state != "waiting") {
			continue
		}
		distance := math.Max(0, remaining+other.along-r.along-s.minimumVehicleSpacing())
		bound = math.Min(bound, math.Min(distance/dt, motion.StoppingSpeed(distance, other.speed, motion.EmergencyBraking, dt)))
	}
	return bound
}
