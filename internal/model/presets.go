package model

import "fmt"

// MetropolitanGridConfig builds the second bundled map preset. Coordinates
// are metres: five-by-five core junctions create four grid cells in each
// direction, spaced 4 km apart for a 16 km square city core. All supporting
// roads stay inside those 16 cells.
func MetropolitanGridConfig() CityConfig {
	const (
		coreMin = 1000.0
		step    = 4000.0
	)
	cfg := CityConfig{
		Name:        "Atlas Metropolitan Grid",
		Description: "A 16 km four-by-four expressway grid whose internal districts connect through independent mid-block ramps, collectors, and local branches.",
		DrivingSide: DrivingSideRight,
		MovementRules: MovementRulesConfig{
			ThroughToleranceDegrees: 40,
			UTurnToleranceDegrees:   35,
		},
		Bounds: Bounds{MinX: 0, MinY: 0, MaxX: 18000, MaxY: 18000},
		Simulation: SimulationConfig{
			People: 320, SpawnRate: 0, MaxPeople: 3000, TickMS: 50,
			WalkSpeed: 18, DriveSpeed: 160, SafeGap: 4, SpawnMargin: 160,
			Seed: 20260903, SnapshotFPS: 12, VehicleSize: 18,
			Intersection: 70, ArrivalDwell: 6,
		},
	}

	addIntersection := func(id, name string, position Vec2) {
		cfg.Intersections = append(cfg.Intersections, IntersectionConfig{
			ID: id, Name: name, Position: position,
			Traffic: TrafficControlConfig{OffsetSeconds: float64(len(cfg.Intersections) * 7 % 31)},
		})
	}
	gridID := func(row, column int) string { return fmt.Sprintf("G%d%d", row, column) }
	for row := 0; row < 5; row++ {
		for column := 0; column < 5; column++ {
			addIntersection(gridID(row, column), fmt.Sprintf("Core %d-%d Exchange", row+1, column+1), Vec2{
				X: coreMin + float64(column)*step,
				Y: coreMin + float64(row)*step,
			})
		}
	}

	// Every grid cell has two mid-block arterial gates plus a compact internal
	// district: a ramp hub, a collector hub, and two local access nodes. Small
	// deterministic offsets avoid making all 16 districts visually identical.
	for row := 0; row < 4; row++ {
		for column := 0; column < 4; column++ {
			prefix := fmt.Sprintf("%d%d", row, column)
			x, y := coreMin+float64(column)*step, coreMin+float64(row)*step
			variation := float64((row*3+column)%3-1) * 180
			addIntersection("XH"+prefix, fmt.Sprintf("District %d-%d North Gate", row+1, column+1), Vec2{X: x + step/2, Y: y})
			addIntersection("XV"+prefix, fmt.Sprintf("District %d-%d West Gate", row+1, column+1), Vec2{X: x, Y: y + step/2})
			addIntersection("H"+prefix, fmt.Sprintf("District %d-%d Ramp Hub", row+1, column+1), Vec2{X: x + 1550 + variation, Y: y + 1550 - variation*.3})
			addIntersection("D"+prefix, fmt.Sprintf("District %d-%d Collector", row+1, column+1), Vec2{X: x + 2600 - variation*.4, Y: y + 2500 + variation*.3})
			addIntersection("A"+prefix, fmt.Sprintf("District %d-%d East Access", row+1, column+1), Vec2{X: x + 3250, Y: y + 1450 + variation})
			addIntersection("B"+prefix, fmt.Sprintf("District %d-%d South Access", row+1, column+1), Vec2{X: x + 1750 + variation, Y: y + 3250})
		}
	}

	reversePoints := func(points []Vec2) []Vec2 {
		if len(points) == 0 {
			return nil
		}
		reversed := make([]Vec2, len(points))
		for index := range points {
			reversed[len(points)-1-index] = points[index]
		}
		return reversed
	}
	addPair := func(name, from, to string, lanes int, laneWidth, speed float64, roadClass string, demand float64, points []Vec2) {
		cfg.Ways = append(cfg.Ways,
			WayConfig{ID: from + "-" + to, Name: name, From: from, To: to, Lanes: lanes, LaneWidth: laneWidth, SpeedLimit: speed, RoadClass: roadClass, Demand: demand, Points: points},
			WayConfig{ID: to + "-" + from, Name: name, From: to, To: from, Lanes: lanes, LaneWidth: laneWidth, SpeedLimit: speed, RoadClass: roadClass, Demand: demand, Points: reversePoints(points)},
		)
	}
	addArterial := func(name, from, to string) {
		addPair(name, from, to, 4, 8, 160, "arterial", 2.5, nil)
	}

	// The north edge of every cell contains its horizontal access gate. The
	// bottom boundary stays continuous because the final row already has access
	// from its northern edge.
	for row := 0; row < 5; row++ {
		for column := 0; column < 4; column++ {
			from, to := gridID(row, column), gridID(row, column+1)
			name := fmt.Sprintf("Atlas East-West %d", row+1)
			if row < 4 {
				gate := fmt.Sprintf("XH%d%d", row, column)
				addArterial(name, from, gate)
				addArterial(name, gate, to)
			} else {
				addArterial(name, from, to)
			}
		}
	}
	// The west edge of every cell contains its vertical access gate. The right
	// boundary remains continuous for the same reason.
	for column := 0; column < 5; column++ {
		for row := 0; row < 4; row++ {
			from, to := gridID(row, column), gridID(row+1, column)
			name := fmt.Sprintf("Atlas North-South %d", column+1)
			if column < 4 {
				gate := fmt.Sprintf("XV%d%d", row, column)
				addArterial(name, from, gate)
				addArterial(name, gate, to)
			} else {
				addArterial(name, from, to)
			}
		}
	}

	for row := 0; row < 4; row++ {
		for column := 0; column < 4; column++ {
			prefix := fmt.Sprintf("%d%d", row, column)
			x, y := coreMin+float64(column)*step, coreMin+float64(row)*step
			variation := float64((row*3+column)%3-1) * 180
			northGate := Vec2{X: x + step/2, Y: y}
			westGate := Vec2{X: x, Y: y + step/2}
			hub := Vec2{X: x + 1550 + variation, Y: y + 1550 - variation*.3}
			distributor := Vec2{X: x + 2600 - variation*.4, Y: y + 2500 + variation*.3}
			eastAccess := Vec2{X: x + 3250, Y: y + 1450 + variation}
			southAccess := Vec2{X: x + 1750 + variation, Y: y + 3250}

			// The ramps curve into the cell and meet only at the district hub;
			// neither ramp terminates at a core Gxx intersection.
			addPair("District North Ramp", "XH"+prefix, "H"+prefix, 2, 7.5, 90, "ramp", 1.2, []Vec2{
				northGate,
				{X: x + 2000, Y: y + 620},
				{X: x + 1700 + variation*.5, Y: y + 1080},
				hub,
			})
			addPair("District West Ramp", "XV"+prefix, "H"+prefix, 2, 7.5, 90, "ramp", 1.2, []Vec2{
				westGate,
				{X: x + 620, Y: y + 2000},
				{X: x + 1080, Y: y + 1700 - variation*.2},
				hub,
			})

			addPair("District Distributor", "H"+prefix, "D"+prefix, 2, 7.5, 105, "collector", 1.4, []Vec2{
				hub,
				{X: x + 2000, Y: y + 1800},
				{X: x + 2300, Y: y + 2200},
				distributor,
			})
			addPair("East Access Branch", "D"+prefix, "A"+prefix, 1, 7, 55, "local", .7, []Vec2{
				distributor, {X: x + 2850, Y: eastAccess.Y}, eastAccess,
			})
			addPair("South Access Branch", "D"+prefix, "B"+prefix, 1, 7, 55, "local", .7, []Vec2{
				distributor, {X: southAccess.X, Y: y + 2850}, southAccess,
			})
			addPair("District Local Connector", "A"+prefix, "B"+prefix, 1, 7, 50, "local", .6, []Vec2{
				eastAccess,
				{X: x + 3420, Y: y + 2600},
				{X: x + 2750, Y: y + 3380},
				southAccess,
			})
		}
	}

	return cfg
}
