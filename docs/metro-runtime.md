# Metro runtime

The current implementation and limitations are documented in
[JSON examples and physical metro circuits](example-layouts.md) and
[Map 3 transfer stations and tunnels](map3-radial-grid.md).

The executor is `internal/metro/runtime.go`; generic JSON circuit loading, physical
route sampling and underground level configuration are in `internal/metro/layout.go`.
It runs on the road simulation's fixed 50 ms clock. `set_metro` applies validated
settings through the common command boundary. Snapshots expose per-car passengers,
physical car poses, train speeds and positions, station queues, reservations,
closed track geometry and numeric layers. Preset-specific materials live only in
`example/Map1`, `example/Map2` and `example/Map3` JSON files.

Shared journey IDs, passenger/task accounting, short-horizon external train controls
and terminal berth release are described in [three-mode execution](three-mode-algorithms.md).
