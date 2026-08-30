package model

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func BuildGraph(cfg CityConfig) (*Graph, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("city name is required")
	}
	if cfg.Bounds.MaxX <= cfg.Bounds.MinX || cfg.Bounds.MaxY <= cfg.Bounds.MinY {
		return nil, fmt.Errorf("city bounds are invalid")
	}
	cfg.DrivingSide = strings.ToLower(strings.TrimSpace(cfg.DrivingSide))
	if cfg.DrivingSide == "" {
		cfg.DrivingSide = DrivingSideRight
	}
	if cfg.DrivingSide != DrivingSideRight && cfg.DrivingSide != DrivingSideLeft {
		return nil, fmt.Errorf("unsupported driving_side %q", cfg.DrivingSide)
	}
	applyMovementRuleDefaults(&cfg.MovementRules)
	applySimulationDefaults(&cfg.Simulation)

	graph := &Graph{
		Name:          cfg.Name,
		Description:   cfg.Description,
		DrivingSide:   cfg.DrivingSide,
		MovementRules: cfg.MovementRules,
		Bounds:        cfg.Bounds,
		Simulation:    cfg.Simulation,
		Intersections: make(map[string]*Intersection, len(cfg.Intersections)),
		Ways:          make(map[string]*Way, len(cfg.Ways)),
		Outgoing:      make(map[string][]*Way),
		Incoming:      make(map[string][]*Way),
	}

	trafficInputs := make(map[string]TrafficControlConfig, len(cfg.Intersections))
	directedPairs := make(map[string]bool, len(cfg.Ways))
	for _, way := range cfg.Ways {
		directedPairs[way.From+"\x00"+way.To] = true
	}
	for _, intersection := range cfg.Intersections {
		if intersection.ID == "" {
			return nil, fmt.Errorf("intersection id is required")
		}
		if _, exists := graph.Intersections[intersection.ID]; exists {
			return nil, fmt.Errorf("duplicate intersection %q", intersection.ID)
		}
		angle := intersection.Rotation * math.Pi / 180
		tangent := Vec2{X: math.Cos(angle), Y: math.Sin(angle)}
		graph.Intersections[intersection.ID] = &Intersection{
			ID: intersection.ID, Name: intersection.Name, Position: intersection.Position,
			Transform: Transform(intersection.Position, tangent),
		}
		trafficInputs[intersection.ID] = intersection.Traffic
	}
	if len(graph.Intersections) < 2 {
		return nil, fmt.Errorf("at least two intersections are required")
	}

	for _, input := range cfg.Ways {
		if input.ID == "" || input.From == "" || input.To == "" {
			return nil, fmt.Errorf("each way requires id, from, and to")
		}
		if _, exists := graph.Ways[input.ID]; exists {
			return nil, fmt.Errorf("duplicate way %q", input.ID)
		}
		from, fromOK := graph.Intersections[input.From]
		to, toOK := graph.Intersections[input.To]
		if !fromOK || !toOK {
			return nil, fmt.Errorf("way %q references an unknown intersection", input.ID)
		}
		if input.From == input.To {
			return nil, fmt.Errorf("way %q cannot loop to the same intersection", input.ID)
		}
		if input.Lanes < 1 {
			return nil, fmt.Errorf("way %q must have at least one lane", input.ID)
		}
		if input.LaneWidth <= 0 {
			input.LaneWidth = 7
		}
		input.RoadClass = strings.ToLower(strings.TrimSpace(input.RoadClass))
		if input.RoadClass == "" {
			input.RoadClass = "local"
		}
		if input.RoadClass != "local" && input.RoadClass != "collector" && input.RoadClass != "ramp" && input.RoadClass != "arterial" {
			return nil, fmt.Errorf("way %q has unsupported road_class %q", input.ID, input.RoadClass)
		}
		if input.Demand <= 0 {
			input.Demand = defaultDemand(input.RoadClass)
		}
		if input.SpeedLimit <= 0 {
			input.SpeedLimit = defaultSpeed(input.RoadClass)
		}
		points := append([]Vec2(nil), input.Points...)
		if len(points) == 0 {
			points = []Vec2{from.Position, to.Position}
		} else {
			if distance(points[0], from.Position) > 0.001 || distance(points[len(points)-1], to.Position) > 0.001 {
				return nil, fmt.Errorf("way %q points must begin at %s and end at %s", input.ID, input.From, input.To)
			}
		}
		if len(points) < 2 {
			return nil, fmt.Errorf("way %q needs at least two points", input.ID)
		}

		carriagewayOffset := 0.0
		if directedPairs[input.To+"\x00"+input.From] {
			// In screen-oriented coordinates the tangent normal (-ty, tx) points
			// visually to the driver's right. Reversed tangents put the paired
			// carriageways on opposite world-space sides using the same sign.
			carriagewayOffset = float64(input.Lanes)*input.LaneWidth/2 + 1.5
			if cfg.DrivingSide == DrivingSideLeft {
				carriagewayOffset = -carriagewayOffset
			}
		}
		way := &Way{ID: input.ID, Name: input.Name, From: input.From, To: input.To, Lanes: input.Lanes,
			LaneWidth: input.LaneWidth, CarriagewayOffset: carriagewayOffset,
			SpeedLimit: input.SpeedLimit, RoadClass: input.RoadClass, Demand: input.Demand,
			Weight: input.Lanes, Points: points}
		way.cumulative = make([]float64, len(points))
		for i := 0; i < len(points)-1; i++ {
			segmentLength := distance(points[i], points[i+1])
			if segmentLength < 0.001 {
				return nil, fmt.Errorf("way %q contains a zero-length segment", input.ID)
			}
			way.Length += segmentLength
			way.cumulative[i+1] = way.Length
			way.Segments = append(way.Segments, makeSegment(way, i, points[i], points[i+1]))
		}
		graph.Ways[way.ID] = way
		graph.Outgoing[way.From] = append(graph.Outgoing[way.From], way)
		graph.Incoming[way.To] = append(graph.Incoming[way.To], way)
		graph.Edges = append(graph.Edges, Edge{WayID: way.ID, From: way.From, To: way.To, Weight: way.Lanes})
	}

	if len(graph.Ways) == 0 {
		return nil, fmt.Errorf("at least one directed way is required")
	}
	for _, intersection := range graph.Intersections {
		for _, way := range graph.Incoming[intersection.ID] {
			segment := way.Segments[len(way.Segments)-1]
			intersection.Approaches = append(intersection.Approaches, Approach{
				WayID: way.ID, Direction: "inbound", Lanes: way.Lanes,
				Transform: segment.Transform, SubSegments: segment.SubSegments,
			})
		}
		for _, way := range graph.Outgoing[intersection.ID] {
			segment := way.Segments[0]
			intersection.Approaches = append(intersection.Approaches, Approach{
				WayID: way.ID, Direction: "outbound", Lanes: way.Lanes,
				Transform: segment.Transform, SubSegments: segment.SubSegments,
			})
		}
		sort.Slice(intersection.Approaches, func(i, j int) bool {
			return intersection.Approaches[i].WayID < intersection.Approaches[j].WayID
		})
		control, err := buildTrafficControl(intersection, trafficInputs[intersection.ID], graph.Incoming[intersection.ID], graph.Outgoing[intersection.ID], graph.MovementRules, graph.DrivingSide, graph.Simulation.Intersection)
		if err != nil {
			return nil, err
		}
		intersection.TrafficControl = control
	}
	sort.Slice(graph.Edges, func(i, j int) bool { return graph.Edges[i].WayID < graph.Edges[j].WayID })
	return graph, nil
}

func applySimulationDefaults(cfg *SimulationConfig) {
	if cfg.People <= 0 {
		cfg.People = 180
	}
	if cfg.MaxPeople <= 0 {
		cfg.MaxPeople = cfg.People * 3
	}
	if cfg.MaxPeople < cfg.People {
		cfg.MaxPeople = cfg.People
	}
	if cfg.TickMS <= 0 {
		cfg.TickMS = 50
	}
	if cfg.WalkSpeed <= 0 {
		cfg.WalkSpeed = 24
	}
	if cfg.DriveSpeed <= 0 {
		cfg.DriveSpeed = 90
	}
	if cfg.SafeGap <= 0 {
		cfg.SafeGap = 18
	}
	if cfg.SnapshotFPS <= 0 {
		cfg.SnapshotFPS = 12
	}
	if cfg.VehicleSize <= 0 {
		cfg.VehicleSize = 8
	}
	if cfg.Intersection <= 0 {
		cfg.Intersection = 22
	}
	if cfg.ArrivalDwell <= 0 {
		cfg.ArrivalDwell = 2.5
	}
}

func applyMovementRuleDefaults(cfg *MovementRulesConfig) {
	if cfg.ThroughToleranceDegrees <= 0 {
		cfg.ThroughToleranceDegrees = 40
	}
	if cfg.UTurnToleranceDegrees <= 0 {
		cfg.UTurnToleranceDegrees = 35
	}
	// Values at or above 90 degrees make left/right classification ambiguous.
	cfg.ThroughToleranceDegrees = math.Min(89, cfg.ThroughToleranceDegrees)
	cfg.UTurnToleranceDegrees = math.Min(89, cfg.UTurnToleranceDegrees)
}

func buildTrafficControl(intersection *Intersection, input TrafficControlConfig, incoming, outgoing []*Way, rules MovementRulesConfig, drivingSide string, intersectionRadius float64) (TrafficControl, error) {
	return planTrafficControl(intersection, input, incoming, outgoing, rules, drivingSide, intersectionRadius)
}

func defaultDemand(roadClass string) float64 {
	switch roadClass {
	case "arterial":
		return 1.8
	case "collector":
		return 1.2
	case "ramp":
		return 1
	default:
		return .65
	}
}

func defaultSpeed(roadClass string) float64 {
	switch roadClass {
	case "arterial":
		return 115
	case "collector":
		return 82
	case "ramp":
		return 92
	default:
		return 58
	}
}

func makeSegment(way *Way, index int, start, end Vec2) Segment {
	tangent := end.Sub(start).Normalize()
	segment := Segment{
		ID: fmt.Sprintf("%s:%d", way.ID, index), WayID: way.ID, Index: index,
		Start: start, End: end, Length: distance(start, end), Transform: Transform(start, tangent),
	}
	for lane := 0; lane < way.Lanes; lane++ {
		offset := way.CarriagewayOffset + laneOffset(lane, way.Lanes, way.LaneWidth)
		normal := Vec2{X: -tangent.Y, Y: tangent.X}
		laneStart := start.Add(normal.Mul(offset))
		laneEnd := end.Add(normal.Mul(offset))
		segment.SubSegments = append(segment.SubSegments, SubSegment{
			ID: fmt.Sprintf("%s:%d:lane-%d", way.ID, index, lane), Lane: lane,
			Start: laneStart, End: laneEnd, Length: segment.Length, Transform: Transform(laneStart, tangent),
		})
	}
	return segment
}

func laneOffset(lane, lanes int, width float64) float64 {
	return lanePositionOffset(float64(lane), lanes, width)
}

func lanePositionOffset(lane float64, lanes int, width float64) float64 {
	return (lane - float64(lanes-1)/2) * width
}

func distance(a, b Vec2) float64 { return a.Sub(b).Length() }

func (g *Graph) SortedWays() []*Way {
	ways := make([]*Way, 0, len(g.Ways))
	for _, way := range g.Ways {
		ways = append(ways, way)
	}
	sort.Slice(ways, func(i, j int) bool { return ways[i].ID < ways[j].ID })
	return ways
}

func (g *Graph) SortedIntersections() []*Intersection {
	items := make([]*Intersection, 0, len(g.Intersections))
	for _, intersection := range g.Intersections {
		items = append(items, intersection)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}
