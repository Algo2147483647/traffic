> Current ownership: facilities and geometry are in `infrastructure/metro`; train execution is in `metro_scheduling`; passenger planning/transfer transitions are in `operations/metro`. Fleet and timetable execution are implemented in `metro_scheduling/service.go`; depot storage is logical.

# Metro runtime

The current implementation and limitations are documented in
[JSON examples and physical metro circuits](example-layouts.md) and
[Map 3 transfer stations and tunnels](map3-radial-grid.md).

The executor is `internal/metro_scheduling/runtime.go`; generic JSON circuit loading, physical
route sampling and underground level configuration are in `internal/infrastructure/metro/layout.go`.
It runs on the road simulation's fixed 50 ms clock. `set_metro` applies validated
settings through the common command boundary. Snapshots expose per-car passengers,
physical car poses, train speeds and positions, station queues, reservations,
closed track geometry and numeric layers. Preset-specific materials live only in
`example/Map1`, `example/Map2` and `example/Map3` JSON files.

Shared journey IDs, passenger/task accounting, short-horizon external train controls
and terminal berth release are described in [three-mode execution](three-mode-algorithms.md).

### Exchange station inspector

Click a visible exchange marker to open its live details. Platforms list line, layer, each served direction, actual queue size and the next train estimate. Transfer walkers are counted separately from platform queues. Select a waiting or transferring passenger to inspect the full itinerary and current leg. Estimates use current progress, 1 m/s² acceleration/braking, scheduled stops and terminal circulation; future control contention and boarding can delay them. Disabled service has no ETA.

Selected passenger itineraries are drawn along directional track chainages, including the correct orbital arc. Completed steps are muted blue-gray, the current step is yellow, and upcoming steps are dashed blue. Transfer sites use double rings. The selection follows the passenger after boarding and retains the completed itinerary until another selection or reset. The overlay respects the selected underground layer.

Height update: the user-selected migration is 5 m per legacy level, without structural thickness. `elevation_m` and `line_elevations_m` are the physical inputs; the map accepts an inclusive metre interval. Legacy level fields remain only for compatibility. Numeric settings are directly editable; finite/positive values, valid shares and capacity/occupancy invariants still apply.
