# Backend evolution: runnable road, taxi and metro services

This plan supersedes the ordering in the earlier implementation audit. Existing
SI units, 50 ms physics, compact interchanges, lane-path expansion and single-order
taxi service are retained. Road/taxi/metro shared demand and the executable algorithm extensions are now
implemented. The dated checkpoint below records their scope; experiment replay
and checkpoint/resume remain future work. Adding interfaces alone does not complete a stage.

## Settings baseline delivered with this plan

| Setting | Interactive range | Default | Meaning |
|---|---|---|---|
| Following clearance | 2-10 m, step 0.5 m | 2.5 m | Empty distance in the current road-following model; vehicle length is additional |
| Active people | 20-2000 | 320 | Existing ordinary-trip population, including walking portions |
| New people | 0-60 per minute | 0 | Adds people to the recycling population; this is not total trip throughput |
| People limit | 100-2000 | 1000 | Cap on active population |
| Taxi drivers | 1-200 | 32 | Persistent fleet, shared physical road engine |
| Orders | 0-60 per minute | 2 | Poisson demand intensity; higher values are deliberate load experiments |
| Dispatch interval | 1-30 s | 2 s | Matching cadence |
| Stop dwell | Configuration | 20 s | Each pickup/dropoff dwell; included in frontend scenario configuration |
| Maximum pickup wait | Configuration | 600 s | Cancellation deadline |
| Playback rate | 0.5-10x | 1x | Step count, never physical speed multiplication |

Lengths remain 6 m for the design passenger car and 3.5/3.75 m for road lanes.
The stopped-queue gap caps at 2.5 m; moving clearance can be larger. These are
practical demonstration defaults, not a universal normal traffic demand or a
calibrated model of a particular city. Backend experiment limits remain broader
than interactive ranges. The 2.5 m gap is consistent with a common simulation
baseline: [SUMO vehicle parameters](https://sumo.dlr.de/docs/Definition_of_Vehicles%2C_Vehicle_Types%2C_and_Routes.html).
That reference does not certify this model. The runtime now consumes a 1.5 s
desired headway, 2 m/s2 acceleration, 3 m/s2 comfortable braking and an 8 m/s2
emergency stopping envelope. These are simulation baselines, not a calibration
against observed traffic.

## Architecture decisions

- One in-process simulation kernel owns the clock and commits state. Road,
  metro and operations are modules, not separately timed network services.
- Infrastructure owns immutable versioned topology. Road and metro own physical
  agents. Operations own travelers, journeys, orders and tasks. No module stores a
  second authoritative vehicle/train position.
- An accepted taxi task locks the taxi immediately, including travel to pickup.
  Pooling and new-order insertion into busy vehicles remain outside this plan.
- A journey selects exactly one mode. Initial metro journeys are station-to-station;
  no car-to-metro transfer or multimodal route solver is implied.
- Each stage ships a running fixture, an observable API/snapshot result, failure
  tests, and compatibility tests. A registry entry without invocation is incomplete.

## Delivery sequence

### Stage 1: road dynamics and an executable kernel boundary

First introduce typed run/entity IDs, world revision, graph revision, commands and
arrival events around existing execution. Preserve one state owner. Use explicit
tick stages: queued commands; due decisions; demand/admission; controls/reservations;
motion; arrivals/boarding; accounting; immutable snapshot. Define same-tick tie
breaking by event priority, entity ID and sequence; do not rely on map iteration.

Then add bounded acceleration/deceleration and speed-dependent desired headway to
road following. Keep minimum gap and stopping-distance enforcement outside plugins.
Expose parameters only after the model consumes them. Extract taxi task lifecycle
and ordinary-trip demand into operations using road-task/arrival contracts.

Acceptance: equivalent 1x/2x/10x results at equal simulation time; emergency queue
braking, safe insertion and merge; accepted taxis cannot be reassigned; reset/map
change rejects stale tasks; all 36 interchange movements and real taxi service pass.

### Stage 1 implementation checkpoint

The requested road-dynamics and operations slice is implemented in the runtime:

- `internal/road_scheduling/motion/dynamics.go` separates desired following speed, discrete stopping
  envelopes and host safety limits. Road admission starts at rest; connector
  traversal also accelerates instead of assigning cruise speed.
- Moving clearance includes a speed-dependent 1.5 s headway. Normal changes use
  +2/-3 m/s2. Emergency stopping uses 8 m/s2. A final geometric clamp prevents
  overlap even for an initially infeasible gap or a changed clearance. That
  exceptional correction is not a physically attainable braking trajectory.
- Lane-change admission checks relative speed and braking distance in front and
  behind; both lanes remain occupied throughout the maneuver.
- `internal/operations/taxi` owns vehicle business state, transition validation,
  pickup/dropoff dwell, occupied/empty distance and fare accounting. Road retains
  the sole physical vehicle and expands/executes its lane path. The JSON view
  remains compatible through an embedded operational record.
- Assignment locks a vehicle before pickup. Boarding, occupied travel and
  alighting remain locked. Only completion or pre-pickup cancellation releases it.
  Commit validation also rejects retiring vehicles and an existing order ID.
- `internal/operations/demand.Arrivals` owns fractional ordinary-trip generation;
  the road adapter admits generated agents. Existing saturation/reset behavior
  remains. This does not enable simultaneous car/taxi/metro demand.

The Stage 1 kernel follow-up is now connected to production execution:

- `foundation.RunID`, `EntityID`, `Revision`, `Stamp` and `EntityRef` identify a
  run, an entity within that run, and world/topology revisions. Existing integer
  IDs remain as compatibility fields; snapshots additionally publish `kernel`
  and each physical person's run-qualified `entity` reference.
- `foundation.Queue[T]` orders envelopes by due tick, priority, entity ID and
  submission sequence. Road's `Command` and `Arrival` use this same queue.
- All existing public controls execute through the command boundary, including
  pause, playback, load, spacing, mode, reset and validated graph replacement.
  Scheduled commands are accepted by `POST /api/commands`; queued application and
  rejection are visible through the existing bounded `/api/events` journal.
- Commands validate run/topology both at submission and commit. Optional strict
  world validation rejects any intervening commit. Reset invalidates pending
  commands, arrivals and reservations. Map replacement also increments topology.
- The 50 ms pipeline is commands, due decisions, demand/admission, controls and
  reservations, motion, ordered arrivals, then operational accounting. Road
  destination events validate run, vehicle, road, route index and taxi task before
  applying arrivals. Readers only obtain snapshots across a completed lock boundary.
- Existing dispatch worker epoch validation remains in its compatibility adapter;
  the general worker protocol migration belongs to Stage 2.

Unsignalized flow links now use rolling advance reservations. The front vehicle
requests the connector and receiving-lane corridor before its comfortable braking
point. Competing merges, new admissions and lane changes cannot take that entry.
The owner keeps its grant while traversing the connector; route changes, reset and
completed entry release it. Desired and hard safety speeds anticipate downstream
leaders, road storage and speed limits. Residual motion at the incoming boundary
continues into the connector instead of resetting speed. Connector completion is
not integrated a second time as road motion in the same tick.

All 36 isolated interchange routes are tested at the minimum posted speed along
that route (without an artificial 10 m/s cap), with no speed dip at intermediate
flow connectors. Another fixture checks competing merges, a stopped downstream
queue, minimum gaps, and release after the queue clears. The reservation policy
is deliberately conservative under contention: it serializes a merge node and
reserves its receiving corridor, rather than optimizing overlapping time slots.
It is rolling one-connector lookahead, not an all-route reservation or a general
2D collision solver. Short downstream storage, actual queue/conflict or a lower
speed limit may still require braking.

Kernel contract and executable examples: [command and event kernel](kernel-and-flow.md).

Verification covers acceleration/braking and emergency queues, fixed-step playback
at 1x/2x/10x, spacing and safe lane changes, road admission, all 36 interchange
movements, hierarchical taxi pickup/delivery, sustained service, and asynchronous
reset invalidation. Operational tests cover dwell and busy-vehicle rejection in
every accepted-task state. The implementation checkpoint passed `go test ./...`,
`go vet ./...`, all seven interchange geometry tests, and both desktop backend
integration tests. The rebuilt EXE smoke test picked up five orders, completed
two deliveries, and verified six expanded lane routes. The bundled engine hash
matches the newly built runtime. Delivery remains at the existing unpacked EXE
path; no installer or alternate version directory was created.

### Stage 2: general algorithm host, with two real worker integrations

Adapt dispatch's process lifecycle into a capability host. Retain its existing JSON
compatibility adapter. First implement external **route proposals** and **signal
schedule proposals**, each with a native baseline using the identical contract.
This proves generalization beyond matching before creating many unused interfaces.

Every request carries contract version, run/epoch, request ID, input tick, world and
topology revisions, algorithm ID/version, parameters and named random seed. Results
identify their input and return typed proposals, never direct mutations.

Validate route connectivity, entry/exit lanes and endpoints, including complete
interchange expansion. Validate signal phase membership, minimum green, amber and
all-red intervals. Reject invalid/stale outputs; native validated fallback remains
available. A late signal proposal cannot bypass an active clearance interval.

Acceptance: replacing worker code changes actual selected roads/phase schedules;
timeout, crash, malformed JSON and stale epoch cause bounded, recorded rejection;
UI remains responsive; existing external taxi matching still works.

### Stage 2 delivered checkpoint

`internal/algorithms` now executes both route and signal workers through one
versioned request/response host. Manifests declare capability, algorithm version,
command, working directory and wall timeout. Requests carry run/world/topology,
input tick, request ID, seed and parameters. Work runs outside the simulation lock;
physical time waits for bounded decisions while controls and snapshots remain live.
Results must echo the full envelope. Native operation resumes on timeout, process
failure, malformed/incomplete output, stale results or semantic validation failure.

The route hook is a live replanning batch (up to eight eligible physical agents per
five simulated seconds). Native hierarchical planning supplies baseline and legal
alternative candidates; the worker can return its own fully expanded lane path.
The host validates endpoints, lane indices, every connector and lane-change space,
then replaces the actual remaining physical route. Signal workers propose a phase
and bounded green duration. Native clearance/minimum-green/maximum-red safeguards
remain in charge of phase transitions. Flow nodes do not receive signal clearance.

Taxi matching retains its existing external-protocol adapter for compatibility;
the new generic host does not silently change old dispatch manifests. Working Python
route/signal examples are installed under `experiments/algorithms` and selectable in
Settings. Full contract: [external algorithms and shared demand](algorithms-and-demand.md).

### Stage 3: common demand and simultaneous car/taxi operation

Create Traveler, Journey, DemandEvent and admission-queue records. Separate ordinary
private cars from the persistent taxi fleet. One seeded OD stream chooses car or
taxi once; cars receive physical road tasks, taxi travelers receive orders. Admission
failure queues the traveler rather than dropping or double-counting them.

Retain Traffic and Taxi presets as mode-share configurations. Add Mixed only when
both populations actually coexist and interfere on the same roads. Change demand
rates/shares for future arrivals without resetting current trips or orders.

Acceptance: a fixed seeded 50/50 demand fixture produces reproducible assignments,
both road populations move together, each traveler has exactly one active journey,
and generated = queued + active + completed + cancelled at every accounting boundary.
Metro share is rejected until the metro executor reports available.

### Stage 3 delivered checkpoint

A seeded shared origin/destination stream now assigns each new journey to road or
taxi exactly once. Private cars are admitted into the same physical vehicle list as
taxis; they share lane entry, following, lane changes, reservations and signals.
Queued private trips retry when storage/capacity is available. Taxi journeys create
orders in the existing exclusive-task lifecycle. The demand summary maintains
`generated = queued + active + completed + cancelled` and separate mode totals.

Demand rate, ratio, taxi fleet and dispatch settings update live. Road/Taxi/Mixed
selection changes future demand shares without resetting time, run ID, vehicle
positions or accepted orders. Existing legacy ordinary trips finish naturally when
the shared stream is enabled; they are not retroactively counted as new journeys.
Only an explicit Reset or map replacement starts a new run. At this historical Stage 3 checkpoint, mixed metro demand was unavailable.
The Stage 5 checkpoint below supersedes that limitation.

Walking speed is configurable from 0.3 to 3 m/s (default 1.4). Driving limits are
read from each road, not the old simulation-wide speed field. Presets now use at
most 60 km/h, retaining lower ramp/local-road limits. The map generator preserves
this calibration. Vehicle bodies are drawn without the former dark outline.

### Stage 4: standalone metro, delivered in three running increments

**4A — Track and one train:** directed metric track geometry, station/platform
positions, lengths/speeds, timetable departures, train length/position/speed and
bounded acceleration/braking. Run a two-terminal line with intermediate stops.
Show train position and dwell/countdown in snapshots and the frontend.

**4B — Multiple trains and interlocking:** fixed blocks, switch/route reservations,
platform occupancy and conflicting route sets. Release a block when the train tail
clears it, not when its head exits. A train must be able to stop before its current
movement-authority limit. Failed control proposals retain the restrictive authority.
Include a shared single-track bottleneck fixture with opposing trains.

**4C — Passengers:** platform FIFO queues, destination-aware boarding/alighting,
train capacity, dwell and passenger completion. A full train leaves unboarded
passengers waiting with their original journey IDs. Start with station-to-station
trips on one connected line; walking transfers are not a prerequisite.

Acceptance: trains actually depart, move, stop and reverse/terminate; no conflicting
block occupancy; no release while a tail remains; no capacity overflow; passengers
complete journeys and remain conserved. Timetable/interlocking interfaces alone
do not satisfy any of these gates.

Stage 4 is now implemented as a four-station, two-train single-track executor.
It includes real timetable gates, bounded motion, section/block occupancy,
station-throat and platform reservations, reversal, FIFO destination-aware
passenger service and capacity enforcement. It is visible and configurable in the
running desktop. See [metro runtime](metro-runtime.md) for geometry, controls and
conservative reservation limits. The later Stage 5 checkpoint adds shared three-mode demand.

### Stage 5: three-mode demand and metro strategy execution

Extend Stage 3's stream with metro only after 4C. Resolve metro OD to served stations
and reject impossible OD explicitly. Keep physical executors on the common clock.
Display separate pending/active/completed totals and mode shares.

Connect train timetable/headway-regulation proposals and empty-taxi relocation to
the general algorithm host. Metro safety authorization remains host-owned; external
algorithms can request routes/departures but cannot grant an occupied block.

Acceptance: reproducible 40/30/30 car/taxi/metro fixture; conservation across all
modes; no resets on share changes; slow/crashed workers cannot cause unsafe motion;
each mode completes actual trips. Compare a native and external schedule in execution.

### Stage 6: all remaining algorithms and experiment/replay completion

Extend host capabilities to geometry generation, phase-plan generation, demand
generation and batched following/lane/merge proposals. Use validated baseline
adapters first, then one alternative implementation per capability.

Introduce versioned run manifests, named RNG streams, recorded inputs/decisions,
command journal and checkpoints. Include model version, map hash, source/worker
hash, policy params, seed and failure mode in exports. Generalize taxi-lab to road,
signals, metro and mixed demand with warmup, multiple seeds and scenario manifests.

Acceptance: replay reproduces decisions and physical state within a declared numeric
tolerance; checkpoint resume matches uninterrupted execution; exports retain failed,
cancelled and incomplete journeys and every proposal rejection. Add a UI experiment
manager only after this runner is usable through code/CLI.

## Algorithm capability and execution matrix

| Capability | Decision boundary | Scheduling | Host validator / fallback |
|---|---|---|---|
| Road routing | Complete road/lane itinerary | Trip start, reroute, taxi task | Connectivity and legal lane expansion; baseline hierarchical route |
| Signal schedule | Next phase and duration request | Control decision boundary | Legal phase and temporal clearance; baseline max-pressure |
| Taxi matching | Vehicle/order pairs | Dispatch interval | Idle/Waiting, uniqueness, epoch; bounded rejection or baseline |
| Empty-taxi supply | Idle vehicle/target pairs | Coarse planning interval | Eligibility, reachable stops, no accepted-task changes |
| Demand | Timestamped OD/mode arrivals | Demand batch | Finite rates, future times, served OD and entity conservation |
| Metro timetable | Departure/headway proposals | Schedule boundary | Feasible train/platform/route; hold departure if unavailable |
| Metro operation | Desired motion/stop requests | Fixed-step native or batch | Braking envelope, authority, capacity; restrictive safe state |
| Road following/lane/merge | Batched desired speed/lane/admission | Bounded control batch | Acceleration, gaps, connector and conflict checks; native safe behavior |
| Geometry/topology | Candidate immutable graph | Scenario build only | Units, geometry, ports, grades and reachability; reject graph |
| Phase generation | Candidate movement/phase sets | Scenario build only | Lane legality and conflict-free phases; reject plan |

Independent-process support is capability-specific. Use persistent workers or
batched requests for frequent decisions, not one process per vehicle per 50 ms.
Each worker has an explicit resource/deadline budget; all mutations commit centrally.
Full native algorithm replacements must also pass validators, not just tweak scores.

## Clock, failure and reproducibility policy

Interactive mode uses bounded asynchronous decisions and safe native fallback.
Experiment mode can use a simulation-time decision barrier for reproducibility;
wall-time timeout still terminates the request. Declare the selected mode in the run
manifest. Record accepted decisions and rejection/fallback outcomes, including their
effective tick, so replay does not depend on worker wall-time scheduling.

Reset, graph changes, fleet changes and command revisions invalidate affected
requests. Snapshots and policy inputs are detached. A worker never holds the world
mutex while executing; precomputation should use versioned immutable snapshots.

## Recommended next implementation

Start Stage 1 with road dynamics and operational task/arrival contracts, followed
immediately by Stage 2's external routing and signal scheduling. Then car/taxi
coexistence gives a working mixed-demand foundation before metro is added. This
delivers useful running capabilities earlier than expanding every interface first.

This document records the staged plan and implementation checkpoints. See
`metro-runtime.md` for the current executable metro scope and remaining limits;
completed checkpoints do not imply that all six stages are complete.

## Stage 5 and algorithm-extension checkpoint (2026-11-15)

The shared stream now executes car, taxi and metro journeys, with per-mode state
counts and a live executor-to-journey conservation audit. Mixed mode selects
40/30/30 on metro-equipped maps. No mode-share change resets accepted tasks.

Demand OD, idle taxi supply, batched motion/merge preference, metro departure/speed,
and scenario geometry/phase workers are connected to production invocation points.
Each uses bounded execution and host-owned validation. See
[three-mode demand and extensions](three-mode-algorithms.md) for exact contracts,
examples, failure behavior and verification. The experiment replay/checkpoint and
generalized multi-seed runner items in Stage 6 remain subsequent work.
