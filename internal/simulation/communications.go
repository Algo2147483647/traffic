package simulation

import (
	"fmt"
	"math"
	radio "traffic/internal/communications_scheduling"
	infra "traffic/internal/infrastructure/communications"
	road "traffic/internal/road_scheduling"
)

// Snapshot extends one locked world observation. Radio evaluation only consumes
// immutable mobility data: it cannot move entities or change passenger tasks.
type Snapshot struct {
	road.Snapshot
	Communications *radio.View `json:"communications,omitempty"`
}

func (w *Simulation) Snapshot() Snapshot {
 var result Snapshot
 w.Simulation.Inspect(func(s road.Snapshot){result.Snapshot=s;if w.radio!=nil {result.Communications=w.radio.Snapshot(s.Kernel,s.Time,mobility(s))}})
 return result
}
func mobility(s road.Snapshot) []radio.Pose {
	if s.RadioLayout == nil {
		return nil
	}
	height := s.RadioLayout.Profile.ReceiverHeight
	poses := []radio.Pose{}
	prefix := fmt.Sprintf("%d:%d:", s.Kernel.Run, s.Kernel.Topology)
	people := map[int]road.Person{}
	id := func(kind string, n int) string { return fmt.Sprintf("%s%s:%d", prefix, kind, n) }
	for _, p := range s.People {
		people[p.ID] = p
		inCar := p.State != "walking_to_lane" && p.State != "walking_to_destination" && p.State != "arrived"
		poses = append(poses, radio.Pose{ID: id("person", p.ID), Kind: "person", EntityID: p.ID, Position: infra.Position{X: p.Position.X, Y: p.Position.Y, Z: p.Elevation + height}, Speed: p.Speed, Heading: p.Heading * math.Pi / 180, IndoorVehicle: inCar, Underground: p.Elevation < 0 || p.Structure == "tunnel"})
	}
	if s.Taxi != nil {
		stops := map[string]infra.Position{}
		for _, stop := range s.Taxi.Stops {
			stops[stop.ID] = infra.Position{X: stop.Position.X, Y: stop.Position.Y, Z: height}
		}
		for _, o := range s.Taxi.Orders {
			if o.State == "Completed" || o.State == "Cancelled" {
				continue
			}
			position, ok := stops[o.Pickup]
			if !ok {
				continue
			}
			u := radio.Pose{ID: id("taxi-passenger", o.ID), Kind: "taxi-passenger", EntityID: o.ID, Position: position}
			if o.State == "InService" || o.State == "Boarding" {
				if p, ok := people[o.VehicleID]; ok {
					u.Position = infra.Position{X: p.Position.X, Y: p.Position.Y, Z: p.Elevation + height}
					u.Carrier = id("person", p.ID)
					u.Speed = p.Speed
					u.Heading = p.Heading * math.Pi / 180
					u.IndoorVehicle = true
					u.Underground = p.Elevation < 0 || p.Structure == "tunnel"
				}
			}
			poses = append(poses, u)
		}
	}
	for _, t := range s.Metro.Trains {
		if t.State == "depot" || t.State == "withdrawn" {
			continue
		}
		poses = append(poses, radio.Pose{ID: id("train", t.ID), Kind: "train", EntityID: t.ID, Position: infra.Position{X: t.Position.X, Y: t.Position.Y, Z: t.Elevation + height}, Speed: t.Speed, Heading: t.Heading, Underground: t.Elevation < 0})
	}
	for _, j := range s.Metro.Journeys {
		u := radio.Pose{ID: id("metro-passenger", j.ID), Kind: "metro-passenger", EntityID: j.ID, Position: infra.Position{X: j.Position.X, Y: j.Position.Y, Z: j.Elevation + height}, Speed: j.Speed, Heading: j.Heading, Underground: j.Elevation < 0, IndoorVehicle: j.Train != 0}
		if j.Train != 0 {
			u.Carrier = fmt.Sprintf("%s/car:%d", id("train", j.Train), j.Car)
		}
		poses = append(poses, u)
	}
	return poses
}
