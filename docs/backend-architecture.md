# Backend architecture and refactoring plan

Status: road/taxi module migration implemented; metro contracts reserved, not a
running metro simulator. This document distinguishes the target architecture from
runtime capabilities. Existing Windows executable location and HTTP/UI contracts
are retained. Project documentation and UI remain English.

## Design constraints

- One simulation owns physical time, tick order, IDs, entity mutation and commits.
- Traffic and Taxi remain selectable scenarios. A trip uses one transport mode;
  no road/metro transfer is assumed.
- A taxi becomes unavailable immediately on accepting a task, before pickup.
  ToPickup, Boarding, WithPassenger and Alighting are never matching candidates.
  Candidate filtering and atomic commit both enforce the restriction.
- Pooling is a future grouped-task contract. Dynamic insertion into an already
  accepted task is not permitted by the current assignment API.
- Infrastructure defines legal topology. Policies cannot override legal connectors,
  speed limits, following constraints, receiving space or clearance intervals.
- Modules are in-process packages. Experimental dispatch workers and experiment
  programs use separate processes; there is no distributed clock or state store.

## Package ownership

| Area | Package / source | Responsibility and runtime status |
|---|---|---|
| Presentation | internal/server/web | Render immutable snapshots, UI controls; unchanged ownership |
| API adapter | internal/server | Validate commands, submit to simulation owner, stream state |
| Foundation | internal/foundation | Single clock value, ordered bounded event journal |
| Road infrastructure | internal/infrastructure/road | Former internal/model; map reading, lane graph, geometry, validated conflict-free phase generation |
| Interchange infrastructure | internal/infrastructure/interchange | Macro nodes and ports, lane-sensitive transfer search, full internal path expansion |
| Metro infrastructure | internal/infrastructure/metro | Station/section graph schema and validation; not enabled in the UI |
| Road execution | internal/road | Former internal/sim; vehicles, entry/exit, following, lanes, signal/flow control, routing, state owner |
| Taxi domain | internal/operations/taxi | Config, stop/order DTOs, authoritative eligibility rules |
| Taxi execution adapter | internal/road/taxi.go | Translate operational tasks into shared road agents and handle real arrival events |
| Demand | internal/operations/demand | Mutually exclusive car/taxi/metro shares and validation; mixed selection is not wired into current separate modes |
| Metro execution | internal/metro | Timetable, interlocking, running contracts and capacity guard; no train loop yet |
| Algorithm runtime | internal/dispatch | Existing native registry and bounded external JSON worker execution |
| Experiments | internal/experiments, cmd/taxi-lab | Isolated CLI runs, paired demand seeds and JSON/CSV export |

The historical Go package identifiers `model` and `sim` are retained inside their
new directories to reduce migration risk; imports explicitly identify their new
ownership. Old source directories are removed rather than duplicated. Existing
configuration files and the frontend protocol remain compatible.

## State and execution sequence

The road Simulation is the aggregate owner under a single mutex. Operations may
propose tasks but do not own a second copy of vehicle position or clock. One physical
substep handles: pending dispatch commit; walking/turn events; lane-change progress;
signal evolution; time advance; lane preparation; constrained road motion; arrival;
taxi service/demand maintenance; tick increment. Taxi time is integrated in bounded
0.05-second physical steps. External decision waits hold simulation time without
holding the mutex across worker execution.

The foundation journal records order creation, assignment, boarding, service,
completion, cancellation and epoch invalidation with a monotonic sequence. Same-time
events therefore have deterministic ordering within a run. GET /api/events returns
a detached copy of the latest 2048 events; history is not retransmitted in each
high-frequency frame. This is an audit stream, not an exact arbitrary-state restore
checkpoint. Paired seeded reruns remain supported by taxi-lab. Full checkpoint and
external-policy state replay remain later work.

Entity IDs remain allocated by the aggregate owner. Retiring drivers finish accepted
service before removal. Live fleet/demand changes invalidate pending proposals but
preserve time, orders and existing tasks. Map/mode changes start a fresh scenario.

## Road and interchange graphs

The detailed graph remains authoritative for movement and legality. Each interchange
also has one macro node with directional entry/exit ports. A macro node owns its
internal road-ID set; the original micrograph remains available for path expansion.
The city API exposes this structure as macro_interchanges. Tests cover eight central
ports and six ports for each three-arm trumpet.

Runtime routing now uses a two-level search implemented in
`internal/infrastructure/interchange/routing.go`. The outer Dijkstra search treats
an interchange as a directional macro node with lane-specific ingress/egress states.
On reaching an ingress, a local Dijkstra search computes legal transfers through
that interchange only. It stops at its exits or at a destination inside the node.
Every macro arc retains its exact internal predecessor path and accumulated cost.
Backtracking expands the selected arcs into actual directed roads with distinct
entry and exit lanes. Vehicles follow those roads through the ordinary turn,
merge, signal, lane-change and storage checks.

Macro transfer costs include the exit road and exclude the already-entered source
road, so adjacent transfers do not double-charge a connecting link. A fresh weight
snapshot incorporates current congestion and the selected RouteCost policy.
Only topology is cached per graph; changing maps rebuilds it. Internal transfer
paths are query-local. This provides real hierarchical routing, without claiming
that lazy transfer recomputation always improves total wall-clock performance.

Both ordinary demand and taxi pickup, delivery, cruising and congestion rerouting
use this planner. There is no road-only fallback in vehicle task planning. The
host first seeks a connector-continuous lane path; if unavailable, it permits
physical lane preparation only on roads with enough usable length (including the
current vehicle position on its first road). This preserves the preference for
avoiding lane changes; it is not an optimum over every possible lane-changing
trajectory. Every receiving lane is explicit and every proposed connector is legal.
The executor still waits for safe lane-change gaps and downstream storage.

Queries support endpoints inside interchanges, direct same-road travel, nonempty
same-road return cycles and exclusion of the first blocked downstream road.
`interchange.Route` includes expanded Steps, macro Leg index ranges, total cost and
outer/internal search counts. Snapshot people expose `route_entry_lanes` and
`route_exit_lanes` aligned with the existing `route` road-ID sequence. Lane indices
remain zero based. A taxi's final stop alignment is handled by the stop controller.
The old Graph.RouteBetweenWays utility remains available for infrastructure tests;
it is no longer the runtime fallback.

Verification compares 826 hierarchy queries with an uncontracted lane-graph search
for identical reachability/cost, including internal endpoints, blocked branches,
cycles and custom weights. Physical taxi tests ask the planner for all 36 catalogue
movements and then drive every advertised road. A separate dispatched order checks
real pickup through C/N and delivery through E/S, including ramp and deck travel.
See [Hierarchical routing](hierarchical-routing.md) for the contract and acceptance
checks.

Geometry and signal-plan generators are separate configuration-stage plugins.
Their output is revalidated through BuildGraph. Generated internal roads use the
same vehicle engine and signal/flow controller as the rest of the map.

## Algorithm extension contracts

### Road policy profiles

Register a versioned per-simulation factory with RegisterStrategies(id, version,
factory). Select it through `-road-policy ID -road-parameters '{...}'` on the backend.
The factory receives a copied finite numeric parameter map. A nil hook uses the
existing baseline. Register factories during program bootstrap, not during a tick.

| Strategies hook | Decision | Host enforcement |
|---|---|---|
| RouteCost | Per-directed-road objective for lane-aware shortest-path search | Positive finite cost; legal graph search remains mandatory |
| DesiredSpeed | Desired running speed | Clamped to the road/vehicle limit, then following/storage constraints apply |
| LaneChoice | Optional passing-lane choice | Adjacent lane, bounds and laneChangeSafe; mandatory route preparation remains enforced |
| PhaseScores | Priorities over prevalidated phases | Finite nonnegative values; green bounds, maximum-red handling and clearance remain in the controller |
| FlowPermit | Additional unsignalized/merge admission veto | Cannot grant an illegal lane connector or override downstream reservations |
| TaxiCruise | Idle supply's next stop | Only an existing accessible stop, followed by normal route planning |
| DemandDelay | Next arrival interval | Finite interval >= 0.05 seconds; zero configured frequency still disables new demand |

These are native code strategy interfaces, not arbitrary executable callbacks on
every physical tick. Register a profile package and import it in the backend's
bootstrap. The factory returns fresh policy state for each simulation. Native hooks
are trusted/cooperative and must return promptly; hostile-code isolation is not
claimed. Full replacement of the collision solver requires preserving the execution
invariants and its tests, rather than bypassing them with a speed policy.

Example:

```go
package research
import sim "traffic/internal/road"
func init() {
    err := sim.RegisterStrategies("research.distance", "1", func(parameters map[string]float64) sim.Strategies {
        return sim.Strategies{RouteCost: func(r sim.RoadCost) float64 { return r.Length }}
    })
    if err != nil { panic(err) }
}
```

### Infrastructure planning

Use model.RegisterPlanner(id, fn) from traffic/internal/infrastructure/road.
A planner receives a JSON-detached CityConfig and returns a proposed CityConfig.
Select `geometry_algorithm` and/or `phase_algorithm` in the map JSON. Geometry runs
first, phase generation second, then the ordinary full graph validation runs.
Unknown IDs and invalid outputs fail map loading; planners cannot mutate the caller's
input. Existing Python cloverleaf/trumpet generators remain reproducible authoring
tools and their generated maps enter through the same validator.

### Taxi matching and task routing

Keep the dispatch.Policy/Factory interface and the documented one-shot external
JSON protocol. Matching returns pairs, not physical positions. The operations/taxi
module owns CanAssign and Eligible; commit rechecks them even if a plugin forges a
candidate or returns a stale response. Busy vehicles cannot accept another order.
Taxi task routing and idle repositioning call the shared road graph; there is no
second undirected taxi router. Current standard service remains one order per task.

### Metro contracts

Timetable.Schedule proposes departures, Interlocking.Permit authorizes sections,
and Operation.Advance proposes train state. Infrastructure validation and Board's
capacity clamp are available. The current release does not advertise running trains,
metro signals, timetable execution or metro demand in Settings. Positive metro share
is rejected when the executor is unavailable. These interfaces reserve the requested
module boundary without pretending that a railway simulator already exists.

GET /api/algorithms returns registered road profiles and dispatch algorithms. Existing
GET /api/dispatch/algorithms remains compatible. Geometry planner IDs are selected
in map configuration, not from the interactive dispatch selector.

## Refactoring sequence and acceptance

1. Inventory ownership and keep existing behavior/DTO names as migration baseline.
2. Move graph/model code to infrastructure/road and vehicle execution to road;
   update imports, relative fixture paths and documentation links. Do not duplicate
   the old directories or create another EXE location.
3. Extract clock/journal and taxi business records/eligibility into their owners.
4. Add directional macro-port representation and code-based strategy registries;
   keep safety validation outside policy hooks.
5. Reserve typed metro and mode-share boundaries; keep unsupported features disabled.
6. Run original model/signal/traffic/taxi tests, new extension/output validation
   tests, busy-taxi rejection, port tests and journal ordering checks.
7. Verify real desktop mode switching, live fleet/demand changes, experimental
   Python assignment, pickup/dropoff and the existing 36 interchange movements.
8. Rebuild and smoke-test desktop/dist/win-unpacked/Meridian Traffic.exe in place.

## Remaining target capabilities

This is a behavior-preserving modular backend refactor, not a completed multimodal
simulator. Train execution, grouped pooling, geographic demand/relocation, complete
checkpoint restore, persistent macro transfer caching and a GUI experiment-job manager
remain separate implementations. Their interfaces must retain the no-reassignment
rule until a versioned grouped-task proposal/validator is explicitly introduced.
