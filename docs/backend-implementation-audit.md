# Backend implementation audit and remaining work

Audit date: 2026-11-14. Scope: current source, production call sites, configuration,
tests and the architecture proposed in this project's conversation. Status means
executable behavior, not the presence of a package name or interface declaration.
This audit follows the implemented hierarchical routing change.

## Subsequent corrections

[Physical units and fixed stepping](physical-units.md) are now implemented for both
scenarios. Shipped speeds are m/s, vehicle length is 6 m, and lane widths are
3.5/3.75 m. Treat the unit/step deficiencies below as historical audit findings;
compact interchange geometry is the next outstanding step.

## Overall finding

The application has a working road/taxi simulator with separate rendering and
calculation, real hierarchical interchange routing and an external taxi dispatch
protocol. The broader target architecture is only partially implemented. Several
new packages contain contracts or DTOs while the execution still lives inside the
road Simulation. It is not yet a common multimodal simulation/algorithm platform.

Keep one simulation owner and deterministic commit order. Completing these module
boundaries does not require splitting the application into network microservices.

## Capability matrix

| Target area | Current status | Source and evidence | Remaining work |
|---|---|---|---|
| Rendering / calculation separation | Running | `internal/server/web`, `internal/server/server.go`, `desktop/backend.cjs`: browser renders snapshots; Go advances vehicles | Preserve this boundary; rendering should not own model state |
| Unified clock and event order | Partial | `foundation.Clock` is used by `road.Simulation`; execution order is explicit in `advanceSubstep` | Common fixed-step kernel and documented command/event priorities. Traffic advances one caller-sized step, while Taxi divides it into <= 0.05 s substeps |
| Entity identity and state commit | Partial | Vehicle IDs and mutations are owned by Simulation; taxi proposals validate epoch/request before commit | Shared run-scoped entity IDs, command contracts and revision checks across modules; current IDs restart and the taxi epoch is not a universal entity version |
| Events / randomness / replay | Partial | Bounded 2048-entry journal records taxi lifecycle/epoch events; car trips and taxi demand/cruising use seeded RNGs | Record commands, configuration, decisions, streams and durable run identity; checkpoints including RNG/policy state; deterministic replay. Current events are not a replay log |
| Algorithm registration / versions / parameters | Partial, wired hooks | `road/policies.go`, `dispatch/dispatch.go`, `road/planners.go` | Unified capability registry and typed parameter schemas. Road profiles are aggregate native factories; infrastructure planner IDs have no implementation-version metadata. External manifest contract version is not algorithm-code version |
| Independent algorithm processes | Running for taxi matching only | `dispatch/worker.go` runs one JSON request per process with deadline and output bounds | Reusable worker contracts for route, demand, signal, geometry and future metro decisions; cancellation and stale-result validation per capability. Native hooks have no enforced timeout and can block their caller |
| Result validation / failure handling | Partial by capability | Assignment candidate/epoch checks, output decode limits; host checks finite costs/speeds, legal lanes, phase scores and valid planner output | Consistent diagnostics, fallback/rejection semantics, version matching and tests for every strategy type |
| Fixed scenarios / multiple seeds / export | Running for taxi experiments | `cmd/taxi-lab` loops policies/seeds, exports JSON/CSV with map and engine hashes | Road/signal/metro experiments, scenario/run manifests, algorithm source hashes and versions, parameter sweeps, optional warmup and comparable metrics. Current runner calls `sim.New`, so it does not select road-policy profiles |
| Road graph | Running | `infrastructure/road` reads, validates and builds directed roads and lane connectors | Explicit physical-unit contract and revisioned immutable topology; demand/network expansion is separate |
| Interchange hierarchy and internal paths | Running | `interchange/routing.go`, `road/routing.go`: macro transfer search expands actual entry/exit lanes; no road-only runtime fallback | Optional batch-level transfer reuse, route diagnostics and richer cost models. Caching is an optimization, not a missing hierarchy implementation |
| Interchange geometry / topology generation | Partial | Python cloverleaf and trumpet compilers generate runnable maps; Go planner callback is invoked by BuildGraph | Engineering scale, curvature/grade/clearance validation, complete profile plumbing and other templates. Several profile fields are still ignored by generation; see scale review below |
| Vehicle entry / exit | Running, road-owned | `simulation.go` handles ordinary trips; taxi fleet changes insert/retire physical agents | Separate vehicle/person/trip lifecycle services and shared demand admission contracts |
| Following / lane changes / obstacle handling / merge | Running baseline; limited extensibility | `lane_changes.go`, `lane_entry.go`, `transition`: following limits, adjacent changes, receiving storage and merge reservations | Full following/lane-change/merge decision contracts with useful observations. No general incident/static-obstacle entity system. DesiredSpeed and LaneChoice are narrower hooks, not complete replacement algorithms |
| Vehicle path planning | Running; cost extension only | Shared hierarchical planner used by ordinary vehicles and taxi tasks | Replaceable validated route-decision contract if algorithm experiments must change search itself. RouteCost changes weights; the Dijkstra and no-change-first policy remain built in |
| Signal movement / phase generation | Running baseline; coarse extension | `signal_plan.go` builds lane movements and conflict-compatible phases; configuration planner callback is called | Separate phase-generation contract/version/diagnostics instead of passing the entire city config to a generic hook |
| Signal timing / phase switching | Running baseline; partial extension | `signals.go`, `max_pressure.go`; PhaseScores modifies scores | Expose a decision proposal for next phase/green duration while the host enforces clearance/minimums. PhaseScores does not replace the complete scheduler |
| Unsignalized intersections | Running for supported flow/merge topology | Legal flow connectors, downstream storage and FlowPermit veto | General priority/yield/stop and conflict-occupancy rules; FlowPermit only vetoes existing permission and receives no complete conflict/vehicle scene |
| Metro track graph | Schema/helper only | `infrastructure/metro/graph.go`: station/section IDs and capacities plus Validate; no production imports | Geometry, lengths/speeds, stops, platforms, switches/blocks, input loading and actual integration |
| Metro timetable / interlocking / movement | Reserved interfaces | `metro/contracts.go`: Timetable, Interlocking, Operation; Board helper | Implement scheduler, train dynamics/position, section occupancy, route locking/release, dwell, queues and boarding/alighting. No production train loop, runtime implementation or dedicated tests |
| Mode-share demand | Unwired helper | `operations/demand/modes.go`: Shares.Validate/Choose; no production call sites | Actual demand stream and admission to car/taxi/metro services. Current Traffic/Taxi switch resets/selects a scenario; it is not mixed demand |
| Ordinary car operations | Running inside road module | Ordinary random trips are generated/recycled in Simulation | Move generation/itinerary/lifecycle ownership into operations while road owns physical agents |
| Taxi task lifecycle | Running; module extraction incomplete | `operations/taxi/domain.go` has DTOs/eligibility; actual assign/board/serve/complete/cancel logic remains in `road/taxi.go` | Extract an operational service that proposes road tasks and consumes arrival events; keep one authoritative vehicle position |
| Taxi matching | Running, native + external | Candidate creation, validation, commit, 6 native choices and Python example | Keep current no-reassignment rule; improve observations/experiments as needed |
| Taxi task routing / empty supply | Running baseline; partial algorithms | Shared road planner; idle target is random with optional TaxiCruise callback | Travel-time estimation including queues/signals; demand/supply forecasts and network-wide repositioning. TaxiCruise receives stops and a proposed target, not a demand heatmap or full fleet state |
| Standard service / pooling | Standard service running; pooling absent | Taxi holds one OrderID; busy states are rejected at candidate and commit stages | Passenger/group capacity and detour constraints only when pooling is deliberately introduced; do not lift the current acceptance lock in this refactor |
| Metro passenger trips | Not implemented | No journey demand, passenger queues or destination-based alighting execution | Complete standalone metro journeys after the train executor; road/metro transfers remain outside the agreed scope |

## Confirmed integration gaps

These are observations from production references, not guesses from folder names:

- No production imports/calls connect `internal/metro`,
  `internal/infrastructure/metro` or `internal/operations/demand` to the runtime.
  Their helpers/interfaces cannot currently produce passengers or train motion.
- The broad road Strategies profile is real code integration, but most hooks are
  modifiers to built-in decisions. It does not satisfy unrestricted algorithm
  replacement for every item marked `[algorithm]` in the target architecture.
- Only assignment has the external JSON proposal/deadline/commit lifecycle.
  Infrastructure planners and road hooks execute in-process and require rebuilding
  the executable when new code is registered. The desktop launcher does not pass
  a custom road-profile choice; the backend CLI can do so.
- The new taxi domain package does not yet own the running lifecycle. That logic
  remains coupled to physical agents, demand and matching in `internal/road/taxi.go`.
- Existing deterministic seeds support comparable reruns; they do not provide
  exact checkpoint restore, arbitrary replay or recorded external-policy outputs.
- Current tests prove supported routes and business rules. They do not certify
  physical highway dimensions or the absent metro/mixed-demand services.

## Map 2 scale review

Update: lane widths are now 3.5 m on Map 1 and 3.75 m on Map 2;
passenger vehicles use a 6 m design length and 1.8 m rendering width. The
measurements and old parameter findings below record the pre-correction audit.
Speed calibration and interchange footprint redesign remain outstanding.


The user's reference to the second map is interpreted as Map 2, the four-cell road
map. There is no runnable metro map in the current application.

The configured conversion is 1000 world units per km. Measurements below are
centerline bounding boxes calculated from the generated JSON's sampled points,
grouping roads whose two endpoints belong to the same interchange. They exclude
pavement-width envelopes, shoulders, earthworks and property boundaries.

| Measurement | Current extent |
|---|---|
| Whole map | 6 x 6 km |
| Central cloverleaf, including approach/exit roads | 3.600 x 3.600 km |
| Central turn ramps alone | 2.340 x 2.340 km |
| North/south trumpet, including approaches | 1.800 x 0.855 km |
| West/east trumpet, including approaches | 0.855 x 1.800 km |
| Each central elevated mainline tagged as one bridge road | 3.000 km long |

The 3 km bridge length is not only a visual impression. The two directed roads
`C_D0_split__C_D0_merge` and `C_D2_split__C_D2_merge` are each assigned `bridge`
for the whole 3000-unit interval. Bridge structural extents have not been separated
from the wider interchange's through-road alignment.

The footprint is chiefly a template choice:

- `portal_station=1800` places opposite macro portals 3.6 km apart.
- `split_station=1500` makes each central bypass 3 km long.
- `right_radius=1100` makes right-turn curves much larger than the 140-unit loops.
- The current circular construction's separation check requires
  `sqrt(2)*(R-r)-R-r >= 2*lane_width`. With r=140 and lane_width=8, R must be at
  least about 854.6 units. This is a limitation of this particular circle layout,
  not a general minimum right-ramp radius for cloverleaf interchanges.
- `lane_width=8`, `vehicle_size=18`, and deck elevation=24 also use world units.
  At the displayed conversion these mean 8 m lanes, an 18 m vehicle length
  parameter, and a 24 m deck elevation. `drive_speed=95` is used as units/second,
  equivalent to 342 km/h at this scale; it is not converted from km/h. A mainline
  speed_limit of 100 corresponds to 360 km/h, although the vehicle cap is 95.
- The wrapper generator hardcodes lane_width=8 and elevation=24*layer; the
  cloverleaf compiler hardcodes its 3/2 lane counts. Declared lane-width, deck-height
  and lane-count profile fields are therefore not all authoritative inputs yet.

Conclusion: this simulation does not establish a requirement for a 3 km-wide
interchange or 3 km-long continuous bridge. The topology is operational, but its
physical geometry/scale needs another implementation pass. A single smaller
radius will fail the present curve-separation rule; changing only the ruler
conversion would misrepresent lanes, vehicles, distances and travel times.

For engineering context, ramp horizontal geometry depends on the design speeds of
the ramp and connected facilities, and vertical alignment has separate controls;
there is no single universal footprint prescribed for every interchange.
[TxDOT ramp geometrics](https://www.txdot.gov/manuals/des/rdw/chapter-15-grade-separations-and-interchanges-/15-7-ramps---direct-connectors-/15-7-3-ramp-geometrics.html).
Interchange selection also considers traffic patterns, volumes and right of way.
[TxDOT interchange types](https://www.txdot.gov/manuals/des/rdw/chapter-15-grade-separations-and-interchanges-/15-3-types-of-interchanges.html).

Recommended redesign: establish meters/seconds internally and km/km/h presentation;
keep lane/vehicle screen visibility separate from physical size; redesign outer
right-ramp alignments and approach stations; partition bridge spans from approach
roads. Derive compact geometry from selected design speeds, curve/grade/clearance
and weaving constraints, then check all 36 movements and real taxi arrivals again.
Do not change the map's unit label to conceal the oversized template.

## Implementation order and acceptance gates

1. **Physical units and common stepping (P0).** Specify length, speed, acceleration
   and time units; migrate map/vehicle values with explicit conversion. Introduce
   the same bounded physical substep for Traffic and Taxi while retaining one
   clock and command owner. Accept: distance/time calibration and equivalent
   fixed-step scenarios across playback rates in both modes; existing safety tests.
2. **Compact, fully parameterized Map 2 geometry (P1).** After unit calibration,
   make design profiles authoritative, redesign the oversized right ramps and
   split actual bridge structures from approach roads. Preserve macro IDs/ports
   where practical. Accept: reported dimensions, curve/envelope/elevation/grade
   checks, all 36 physical movements and end-to-end taxi service. This addresses
   the visible 3 km concern before adding more transport modes.
3. **Finish foundation and operation boundaries (P1).** Add run-scoped identity,
   revisioned command/arrival contracts and explicit stage ordering. Move car
   demand and taxi operational lifecycle out of road motion into services owned
   by operations. Accept: one copy of physical state; accepted taxis unavailable;
   no stale commit after reset, map change or live fleet modification.
4. **Complete algorithm contracts (P1).** Define typed inputs/proposals/validators
   for routing, following/lane changes, merge/priority, phase generation/timing,
   geometry, demand and supply repositioning. Adapt current baselines first;
   generalize the existing process runner for bounded decision tasks. Real-time
   motion algorithms need an explicit execution budget, not an unbounded external
   process per vehicle per tick. Accept: alternate implementations demonstrably
   affect execution; invalid/late decisions cannot mutate the world.
5. **Replay and general experiments (P2).** Record run manifest, commands, named
   random streams, decisions and checkpoints; extend the taxi runner to road/signal
   strategies and later metro. Include implementation hashes, versions and params.
   Accept: deterministic paired runs and checkpoint replay comparisons; export
   captures failed/cancelled runs and pending trips without hiding them.
6. **Taxi routing/supply and capacity improvements (P2).** Add better ETA inputs,
   forecast-aware idle relocation and per-decision reuse of route weight snapshots.
   Profile the current candidate loop: up to 64 waiting orders x idle fleet each
   builds routes and congestion snapshots while the simulation lock is held.
   Accept: documented routing/candidate/solver timings, continued pickup/dropoff
   and congestion response, and unchanged assignment restrictions.
7. **Standalone metro execution (P2).** Load a real track graph; implement timetable,
   occupied blocks and route interlocking, train motion/stops, platform queues and
   capacity-limited boarding/alighting. Wire the clock, snapshots and experimental
   contracts. Accept: two-train conflict avoidance, lock release, timetable/dwell
   behavior, capacity accounting and completed metro passenger journeys.
8. **Mixed demand without transfers (P3).** Wire Shares into one demand source and
   admit each trip to exactly one available mode. Coexist ordinary cars and taxis;
   add metro only after step 7 passes. Accept: reproducible mode proportions,
   population conservation and shared-road interaction without scenario resets.

Pooling is a separate later extension, not an implicit requirement to remove the
current single-task rule. It needs grouped-task capacity/detour contracts and an
explicit business decision. A GUI experiment manager and additional interchange
types can follow their underlying runtime contracts.

## Changes made with this audit

Settings now provides **Show coordinates and scale**. It controls the X/Y kilometer
rulers, numeric coordinate labels and scale/map-size display together, without
altering geometry or routing. The preference uses the existing frontend storage
approach (localStorage, scoped to the current origin). Desktop backends use dynamic
localhost ports, so persistence across different launches is not guaranteed.

This delivery audits and schedules the remaining backend work; it does not claim
to implement metro, mixed demand, full strategy replacement, replay, unit migration
or compact interchange geometry. Those remain the steps above.
