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
  2,000 m. Four diagonal junctions now connect the grid to the inner ring. The
  central square at x/y = +/-500 m is connected to the surrounding grid. At each
  axis crossing it descends to a -1 layer, -6.5 m tunnel, with 200 m approach ramps
  and a 100 m underground section. These crossings have no graph connection to
  the overlying cloverleaf roads. All added streets retain one lane per direction.
  No dangling local branches remain.
- Posted speeds are at most 60 km/h. Physical lane and vehicle dimensions are
  retained; the outer-ring lane count does not change the displayed distance scale.

Three metro axes retain Map 1's 60-degree arrangement. Stations lie at signed
radii -4,096, -2,048, 0, 2,048 and 4,096 m on each line. Their outer station pairs
coincide with the six stations of a new radius-4,096 m orbital line. The orbital
line is on layer -13; the axes are on -10, -11 and -12. The six shared sites
are real transfer stations (Exchange 1 through Exchange 6). Map 3 reserves levels
-10 and below for metro; its Settings and backend reject shallower metro floors.

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
lane count and four corner ring entries, no dangling grid vertices, complete directed
road reachability, actual taxi completion, both orbital directions, continuous
physical train motion, and all thirty directed orbital passenger journeys.

Validation: full Go tests and vet passed, seven interchange geometry tests passed, both desktop backend tests passed. Packaged Map 3 smoke verified preset discovery, interchange detail selection, metro passenger completion and the rendered kilometre coordinates. Bundled engine and Map 3 JSON hashes match the current build.

## Transfer execution and accounting

The six transfer links are explicit JSON records with platform references and a
60-second walking duration. The station-level route planner minimizes estimated
travel, dwell, boarding wait and transfer time. It supports axis-to-ring trips and
axis-to-axis trips through the ring. The central co-located axis stations are not
implicitly added as transfer links.

Every passenger keeps one network identity and one external journey ID. Alighting
at an exchange advances the itinerary, then a timed walk leads to the next platform
queue. FIFO boarding and train capacity apply to every leg. Only final alighting
(or completion of a transfer-only trip) completes the original journey. Disabling
metro freezes both trains and transfer walks. Reset removes all associated tasks.

Snapshots include exchange sites, itineraries and waiting/onboard/transferring
states. Network generated/completed counts represent people, not boarding legs.
`waiting` includes platform queues and transfers; `transferring` is a subset.
Thus generated = waiting + onboard + completed throughout the journey. The shared
road/taxi/metro ledger also checks exactly one live executor task per traveler.

Transfer times and road tunnel dimensions are editable in the Map 3 JSON materials.
The layout generator persists explicit connections; loading a preset does not
execute the generator.

Transfer/tunnel verification: all metro package tests passed after fixing compatibility with the legacy in-memory test layout. The remaining full-run packages passed, including road tests, and vet, seven geometry tests and two desktop backend tests passed. Transfer tests exercised 15 cross-line journeys with capacity 3 and 24 shared-demand journeys, checking conservation throughout. All eight directed tunnel passages completed. Packaged desktop smoke recorded six exchanges, 18 transfers, two final metro journey completions and independent road/metro underground visibility. Bundled engine and Map 3 data hashes match the final build.
