# Meridian City Traffic Simulator

A traffic simulation with a Go backend, a directed lane-weighted road graph,
N-way intersection geometry and signal plans, live Server-Sent Events, and an
interactive full-screen HTML canvas UI.

## Desktop application (Windows / macOS)

The standalone application follows the `math-agent-platform` desktop approach:
an Electron window owns a bundled computation engine. Traffic uses its existing
Go engine and embedded canvas UI, so installed users need neither Go, Node.js,
an external browser, nor a separately started server. All city presets ship with
the application. The optional web preview remains a separate project.

For development, install **Node.js 22.13+** and **Go 1.24+**:

```bash
cd desktop
npm ci
npm start
```

From the repository root, `npm run desktop` also starts the desktop application
after its dependencies have been installed. Windows uses a native standalone
window; on macOS, closing the window keeps the app and simulation running until
Quit (Cmd+Q), and clicking the Dock icon reopens it. Launching the app again
focuses the existing instance. View provides reload and fullscreen controls.

### Packaging

Run on the corresponding operating system:

```bash
# Windows x64: creates an NSIS installer
npm --prefix desktop run dist:win

# macOS: creates separate Intel (x64) and Apple Silicon (arm64) DMGs
npm --prefix desktop run dist:mac

# Local unpacked application for checking packaged resources
npm --prefix desktop run pack
```

Outputs are in `desktop/dist/`. A packaging hook compiles the Go executable for
each target architecture before copying it and `configs/` into application
resources. No dependency on the source checkout or its working directory is
required at runtime. Build macOS DMGs on a Mac; Windows cannot produce or
validate a signed macOS application. Signing/notarization credentials are not
included: configure electron-builder signing credentials for public distribution.
The manual/tag-triggered workflow in `.github/workflows/desktop.yml` checks and
packages both systems, uploading unsigned build artifacts without publishing.

### Desktop checks and lifecycle

```bash
go test ./cmd/... ./internal/...
npm --prefix desktop test
npm --prefix desktop run smoke
```

The smoke check opens a real Electron window, verifies map loading, pause/resume,
and advancing simulation time, captures a PNG in the OS temporary directory,
and quits. Set `TRAFFIC_SMOKE_OUTPUT` to choose its screenshot path. The packaged
executable also accepts `--smoke-test` to test bundled resources directly.

The engine binds only to a random `127.0.0.1` port in desktop mode. A per-launch
secret is injected into requests by the Electron session and is not exposed to
page JavaScript. Renderer Node integration is disabled, context isolation and
sandboxing are enabled, and navigation/new windows are restricted following
[Electron security guidance](https://www.electronjs.org/docs/latest/tutorial/security).
The parent pipe closes on app exit or crash, which shuts down the simulation and
SSE streams; a bounded forced-stop fallback handles unresponsive engines.
Startup/engine errors appear in a native error dialog.

## Run the local HTTP service

Requirements: Go 1.24 or newer.

```bash
go run ./cmd/traffic -config configs/meridian-city.json -addr 127.0.0.1:8080
```

Open [http://localhost:8080](http://localhost:8080). The HTML, CSS, and
JavaScript are embedded in the Go binary, so the application does not need a
separate frontend process.

The top-bar map selector exposes two live presets:

- **Preset 1 — Meridian Hierarchy** keeps the JSON map supplied through
  `-config` (the existing `configs/meridian-city.json` by default).
- **Preset 2 — Four-Cell Interchange Network** has a central full cloverleaf
  with four loop ramps, four direct right-turn ramps and four two-lane
  collector-distributor carriageways. The four peripheral nodes retain their
  three-arm folded diamonds. There are 88 nodes and 128 directed ways.
  Mainline through traffic bypasses the C/D weaving sections without signals.

Settings → **LAYER** selects **All layers / Ground / Bridges / Ramps**.
Click the central interchange or one of its nodes for the interchange overview;
**Movement** selects any of the 12 directional straight/left/right routes.
Peripheral signal terminals retain phase diagrams. The original Meridian preset
is unchanged. Layer selection changes display only, not simulation state.

See [Interchange system specification](docs/interchange-system.md) for the
standards register, common-type catalogue, implemented architecture, parameters,
validation and limitations. [Implementation summary](docs/interchange-summary.md)
records the delivered scope and checks.

Authoring and checks:

```bash
python scripts/generate_interchange_map.py
python scripts/test_interchange_geometry.py
go test ./cmd/... ./internal/...
npm --prefix desktop test
```

The compiler reads `configs/interchange-design.json`. Generation is deterministic.
Only the diamond/folded-diamond and cloverleaf templates are implemented; other
common types are documented design references. Road paths sampled by the compiler
are shared by rendering and simulation. Central flow nodes use explicit lane
connections. Trip endpoints cannot attach to limited-access mainline/C/D roads.
Ramp elevations use smooth endpoint transitions; slope-dependent dynamics and
engineering clearance/superelevation checks are not part of the simulation.

Additional presets are ordinary city JSON files in `configs/presets/`. The
server discovers all `*.json` files in filename order at startup, using each
filename (without `.json`) as its preset ID. Add a file to extend the selector,
or point `-preset-dir` at another directory; no Go code change is required.

```bash
go test ./cmd/... ./internal/...
go build -o traffic ./cmd/traffic
```

The optional `app/` project is a publishable version of the visualization. It
connects to the Go service on port 8080 during local development and presents a
self-running preview when the backend is unavailable.

```bash
npm install
npm run dev
npm run build
```

## Model

### Intersections and transforms

Each intersection vertex may have any number `N` of inbound and outbound
approaches. A directed way `i` has `Mi` lanes. A polyline way is split into
segments, and every segment contains exactly `Mi` lane sub-segments.

World coordinates follow the canvas convention: `x` increases to the right and
`y` increases downward. Consequently, positive signed angles turn visually to
the right and negative signed angles turn visually to the left. This convention
is shared by backend geometry, the embedded UI, and the publishable preview.

Every segment and sub-segment exposes a homogeneous 2D local-to-world matrix:

```text
[ tx  -ty   x ]
[ ty   tx   y ]
[  0    0   1 ]
```

`(tx, ty)` is the unit tangent and `(x, y)` is the segment origin. Lane origins
are offset along the tangent's normal by the configured lane width. The city
response groups the first or last segment under the relevant intersection as
an inbound or outbound approach.

Every person snapshot also has:

- a world pose matrix;
- the nearest reference intersection and its matrix;
- a local pose matrix relative to that intersection.

### Directed graph

Intersections are vertices and ways are directed edges. Edge weight is exactly
the number of lanes. Route search operates on directed ways and permits a road
transition only when their shared intersection contains at least one explicit
lane-to-lane connector. Opposite directions are represented by separate ways,
so lane counts and speed limits may be asymmetric. Paired directions receive separate
carriageway offsets, keeping their lane centers and vehicles physically distinct.
`driving_side` is explicit (`right` by default, or `left`). Lane indices always
run from the driver's left to right, so lane `0` is the leftmost lane regardless
of driving side.

### Traffic control

When phases are omitted, the backend derives a protected movement plan from the
actual lane connectors through the junction. Each connector is traced along the
same tangent-continuous cubic path used by turning vehicles. Connectors conflict
only when those paths cross or violate lane-width clearance, or when they merge
into the same receiving lane; approach direction and turn labels alone do not
force separate phases. The planner finds maximal compatible connector sets and
selects a compact, demand-weighted cover, allowing opposing flows, corner turns,
and safe lane-separated transfers to run together at both four-way and three-way
junctions. Green time is allocated from capacity-normalized critical flow ratios
using Webster's cycle formula. Each generated phase has a minimum, design, and maximum green. At run
time, independent intersection controllers measure incoming and downstream lane
queues in a 180-unit detector zone. For every permitted movement they calculate
the non-negative upstream-minus-downstream queue differential, sum those values
for every feasible phase, and select the largest phase pressure. Empty approaches
gap out and a uniquely dominant active phase can extend only to its configured
maximum green. Equal pressures rotate deterministically in configured phase
order. A maximum-red guard forces a starved phase after one design cycle so
sustained demand cannot permanently suppress a lower-pressure movement.
Controller state separately tracks the active, last-served, and pending
phases. Pending selection can respond throughout clearance, is committed once
all-red ends, and cannot immediately reselect the completed phase when another
phase exists. Stage timers consume simulation time exactly, including steps that
cross multiple boundaries. Every phase has amber and all-red clearance
intervals. Vehicles wait
until an exact `(incoming way, outgoing way, incoming lane, receiving lane)`
connector receives green. Route planning sees the union of connectors across all
phases, while intersection entry is restricted to connectors displayed by the
active phase. Snapshots expose
the current stage, countdown, demand and critical-flow scores, controller
decision, every phase's incoming queue, outgoing queue, pressure breakdown,
green target, lane-level movements, and planning basis; the UI highlights only
live movements in light green. See [Max-Pressure scheduling](docs/max-pressure.md)
for the formula, assumptions, data flow, and tie policy.

Movement classification uses the signed approach-to-exit angle. The configurable
`movement_rules` tolerances define the angular windows for through and U-turn
movements; everything else is classified by its signed left/right direction.
Every movement contains an explicit `lane_connections` list of
`{from_lane,to_lane}` paths. For each generated approach, target directions are
ordered clockwise from the driver's left to right. With at least `N-1` lanes,
every target receives a dedicated contiguous lane block and surplus lanes
reinforce the targets farthest from the current direction first. With fewer
lanes, adjacent target directions are merged into balanced contiguous groups,
keeping farther targets independent where the monotone order permits. Each
movement collectively reaches every downstream lane. Explicit plans can provide
their own exact connection list; omitted lists are derived from their declared
lane sets. Invalid, duplicate, uncovered, or out-of-range connections are
rejected during loading. Dead-end U-turns retain the median-side lane for the
configured driving side.

### Controller efficiency test

`traffic-bench` runs the legacy arbitrary fixed-time plan, fixed Webster plan,
and actuated Webster plan against three identical seeded copies of the city,
then emits machine-readable JSON:

```bash
go run ./cmd/traffic-bench -duration 360 -step 0.1
```

The benchmark reports cumulative vehicle and signal delay, signal stops and
releases, completed trips, throughput, and current/maximum signal queue. The
same seeded Meridian demand is also protected by a regression test that requires
the adaptive controller to reduce signal delay without reducing completed trips,
while keeping peak queue within 10% of the fixed-time baseline. The bounded
queue allowance avoids treating additional traffic delivered to downstream
signals as a regression when adaptive control increases completed throughput.

### Wall-clock congestion observation

`traffic-observe` runs the live ticker for an exact real-time observation
window and samples immutable simulation snapshots throughout the run. The JSON
report attributes delay and queues to intersections, lane-to-lane movements,
and directed ways, and includes a ten-real-second timeline for checking how
congestion develops:

```bash
go run ./cmd/traffic-observe \
  -duration 120s \
  -time-scale 4 \
  -sample-interval 1s \
  -output artifacts/traffic-observation-120s-4x.json
```

The report records both requested and actual wall time, simulated time advanced,
the achieved time scale, population and throughput, direct signal delay, phase
changes and controller decisions, downstream queues, movement pressure, and
road-level stopped vehicle-seconds.

### Door-to-door trips

Origins and destinations are independently sampled from a seeded uniform random
distribution covering the complete configured bounds. Each person receives both
points at trip creation, walks to the nearest physical lane, and becomes a
vehicle. A capacity-aware lane-connector path leads to the destination's nearest
lane. Planned demand raises the cost of roads approaching their storage
capacity, so initial and recurring trips share usable alternatives instead of
saturating one directed cycle. If the immediate downstream road later fills, a
vehicle may replan the remaining lane-valid path around it.
The person leaves that lane before the final intersection, becomes a pedestrian
again, and walks to the exact destination. Arrivals remain visible briefly
before a new random trip begins. Meridian starts as a closed 320-person system
(`spawn_rate_per_second: 0`) so its default workload is stationary; positive
spawn rates remain available through configuration or the load-control API for
deliberate ramp and overload tests.

Drivers are grouped by `(way, lane)` and ordered by distance along the way on
every tick. A moving follower reduces speed or waits when the leading vehicle
would violate `safe_gap + vehicle_size`; a stopped queue compacts to a two-unit
bumper gap so storage vacancies can propagate through congestion. `safe_gap`
can be changed live from the CITY overlay or API. At an intersection, deterministic routing
follows the precomputed route while also obeying the intersection signal plan.

On multi-lane roads, vehicles plan adjacent-lane maneuvers dynamically. Required
turn and destination lanes take priority; otherwise a vehicle may pass a slow or
blocked leader when the neighboring lane has safe front and rear gaps. A lane
change follows a smooth time-based lateral path, reserves both source and target
lanes for car-following, and is exposed as `lane_position` plus `lane_change` in
person snapshots. Near an intersection, opportunistic passing is suppressed so
vehicles remain in a lane compatible with their next route movement.

The live population controls accept up to 10,000 active people. Large load
increases plan routes in deterministic parallel waves, congestion costs use a
single per-way load snapshot, and both road-state commits and public snapshot
conversion are split across the available CPU cores.

The edge-to-edge canvas supports mouse or touch dragging, cursor-centered wheel zoom,
pinch-to-zoom, dedicated zoom controls, fit-to-city, and browser full-screen
mode. The camera range is 3%–6400% of the fitted city view. Lane boundaries,
direction arrows, turn-lane markings, stop lines, signal lamps, ramp gore
hatching, cubic Bézier road paths and lane connectors, and vehicle bodies all
use world units, so they scale together. The top bar reports measured simulation
updates per second alongside its configured target. Junction surfaces are
approach-shaped polygons instead of generic circles.

## JSON input

The included [Meridian Hierarchy configuration](configs/meridian-city.json)
defines 21 named intersections and 48 long directed ways. Four neighborhood
trees feed collectors, then curved ramps, and finally four-lane cross-city
arterials; local streets never connect directly to the central exchange.

Minimal shape:

```json
{
  "name": "Example City",
  "driving_side": "right",
  "movement_rules": {
    "through_tolerance_degrees": 40,
    "u_turn_tolerance_degrees": 35
  },
  "bounds": { "min_x": 0, "min_y": 0, "max_x": 800, "max_y": 600 },
  "simulation": {
    "people": 240,
    "spawn_rate_per_second": 1.5,
    "max_people": 900,
    "tick_ms": 50,
    "walk_speed": 24,
    "drive_speed": 90,
    "safe_gap": 4
  },
  "intersections": [
    { "id": "A", "name": "Alpha", "position": { "x": 100, "y": 300 } },
    {
      "id": "B",
      "name": "Beta",
      "position": { "x": 700, "y": 300 },
      "traffic_control": {
        "offset_seconds": 2,
        "phases": [
          {
            "id": "westbound",
            "name": "Westbound through",
            "duration_seconds": 10,
            "allowed_incoming_way_ids": ["AB"]
          }
        ]
      }
    }
  ],
  "ways": [
    {
      "id": "AB",
      "from": "A",
      "to": "B",
      "lanes": 3,
      "lane_width": 7,
      "speed_limit": 100,
      "road_class": "arterial",
      "demand_weight": 2.1,
      "points": [{ "x": 100, "y": 300 }, { "x": 700, "y": 300 }]
    }
  ]
}
```

`points` is optional; a straight segment between the two intersections is used
when it is omitted. When supplied, the first and last points must exactly match
the referenced intersections.

## HTTP API

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/health` | Health and current simulation time |
| `GET` | `/api/city` | Graph, edges, intersections, segments, and transforms |
| `GET` | `/api/snapshot` | People, trips, poses, reference matrices, live signals, and flow stats |
| `GET` | `/api/stream` | `city` and `snapshot` Server-Sent Events |
| `POST` | `/api/control` | Pause, resume, step, reset, change time scale/load, or set vehicle spacing |

Control examples:

```json
{ "action": "pause" }
{ "action": "step", "delta": 0.05 }
{ "action": "reset", "population": 120 }
{ "action": "set_time_scale", "time_scale": 10 }
{ "action": "set_load", "population": 500, "spawn_rate_per_second": 4, "max_population": 10000 }
{ "action": "set_spacing", "safe_gap": 4 }
```

## Project layout

```text
cmd/traffic/                 executable entry point
desktop/                     Electron window, engine lifecycle, installers and smoke checks
cmd/traffic-bench/           seeded fixed-time/adaptive A/B benchmark
cmd/traffic-observe/         real-time congestion observer and JSON report
configs/                     primary JSON city input
configs/presets/             auto-discovered JSON map presets
internal/model/              graph, intersection, segment, and matrix model
internal/sim/                spawning, walking, driving, queuing, snapshots
internal/server/             HTTP/SSE API and embedded HTML UI
internal/server/web/         standalone HTML, CSS, and canvas renderer
app/                         publishable visualization surface
```
