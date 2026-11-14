# Map 3 - Radial grid and orbital metro

Map 3 is an independent preset derived from Map 1. Map 1 and Map 2 are unchanged.
The cloverleaf centre is the world origin `(0, 0)`. Coordinates are metres, with
positive X east/right and positive Y south/down. Both axes use the same scale.

- Central two-level cloverleaf: existing validated collector-distributor geometry,
  6.5 m deck clearance, explicit lane connectors and twelve through/turn routes.
- Inner arterial ring: radius 2,000 m, two lanes per direction.
- Outer arterial ring: radius 4,000 m, four lanes per direction (eight total).
- Cardinal axes reach the outer ring; four lanes per direction outside the junction.
- Local streets: 500 m spacing, one lane per direction, strictly inside radius
  2,000 m. They attach to axis nodes only, never to either ring. The central
  interchange envelope is excluded. Iterative degree pruning removes dangling grid
  branches. Every remaining local node has at least two distinct road neighbors.
- Posted speeds are at most 60 km/h. Physical lane and vehicle dimensions are
  retained; the outer-ring lane count does not change the displayed distance scale.

Three metro axes retain Map 1's 60-degree arrangement. Stations lie at signed
radii -4,096, -2,048, 0, 2,048 and 4,096 m on each line. Their outer station pairs
coincide with the six stations of a new radius-4,096 m orbital line. The orbital
line is on layer -4; the axes remain on -1, -2 and -3. These are co-located station
sites with separate platforms; automatic passenger transfers between lines remain
outside the current single-line journey model.

The orbital line has separate closed clockwise/counterclockwise physical tracks,
directional block reservations, wrapped station indices and shortest-direction
boarding. Trains do not reverse or teleport at a synthetic terminal. The three
linear lines retain their physical terminal circuits. Ring track centre offsets
preserve the existing 15 m track spacing and 5.8/6.4 m tunnel dimensions.

Materials are `example/Map3/preset.json`, `road.json`, `metro.json`, and
`generation.json`. `python scripts/generate_radial_grid_map.py` regenerates the
JSON from that specification and the existing Map 1 / cloverleaf source materials.
Production loads the generated JSON directly, without executing the generator.

Regression coverage includes metre/radius checks, six shared station sites, grid
lane count and no ring attachment, no dangling grid vertices, complete directed
road reachability, actual taxi completion, both orbital directions, continuous
physical train motion, and all thirty directed orbital passenger journeys.

Validation: full Go tests and vet passed, seven interchange geometry tests passed, both desktop backend tests passed. Packaged Map 3 smoke verified preset discovery, interchange detail selection, metro passenger completion and the rendered kilometre coordinates. Bundled engine and Map 3 JSON hashes match the current build.
