> Historical audit: package ownership and metro fleet implementation have changed. See [current backend architecture](backend-architecture.md) and the current release validation record.

# Current architecture review — 2026-11-15

Scope: executable production call sites, not directory names. The architecture is **partially complete**. Real multimodal operation and algorithm workers now exist, but the research platform and general railway service management are not complete. Older audits that describe all metro functions as interfaces are superseded.

| Planned component | Current implementation | Remaining work |
|---|---|---|
| Frontend rendering | Go-owned snapshots, canvas presentation and Electron host | Render scalability, richer spatial inspection |
| Unified clock and event order | One 50 ms physics clock, ordered command/arrival queues (`foundation/kernel.go`, `road/kernel.go`) | Multi-rate scheduler and high-resolution event synchronization for communications |
| Entity identity/state commits | Run/revision stamps, serialized mutation, validated external proposals | Fully unified typed identifiers across every domain and durable state schema |
| Events/randomness/replay | Seeded domain RNGs; bounded 2,048-event journal | Durable full command/decision log, RNG checkpoints, exact checkpoint restore and replay; seeded reruns alone are not replay |
| Algorithm registry/processes | Versioned manifests; runtime route, signal, demand, taxi supply, road motion, metro regulation; separate taxi matching protocol; scenario geometry/phase workers | Consolidate protocol families, richer observations, versioned parameter schemas and reproducibility manifests |
| Validation/timeouts/fallback | Bounded subprocess requests, deadline/output/stale-result checks; host safety rules | Capability-wide diagnostic/benchmark coverage; synchronous native callbacks are not process-isolated |
| Scenarios/seeds/export | JSON examples; taxi-lab multi-seed runner and export (`experiments/run.go`) | General road/metro/communications experiment matrix, warmup, statistical comparison, source hashes and archived decisions |
| Road infrastructure | Loaded directed roads and actual lane connections | General incident/obstacle geometry, topology revisions |
| Hierarchical interchanges | Macro-node/port search expands to executable internal lane paths | Additional interchange templates, richer congestion costs, formal engineering certification |
| Geometry/topology generation | Cloverleaf/trumpet compilers plus validated scenario-time external planner | Broader road design constraints and template families |
| Vehicle lifecycle and motion | Admission/retirement, acceleration/braking, headway, lane changes, merge reservations and continuous-flow reservation | A dedicated vehicle runtime boundary; static/dynamic obstacle entities beyond other vehicles |
| Vehicle routing | Hierarchical native and external route proposals used by cars/taxis | Richer travel time prediction and dynamic rerouting policies |
| Signal junctions | Connection/conflict graph and compatible phases, native/adaptive scheduling and external proposals | Independent fine-grained phase-generator API; more general controller strategies |
| Unsignalized junctions | Validated through/merge topology and reservation rules | General stop/yield/right-of-way engine and separately packaged experiment contract |
| Metro graph | JSON physical tracks, station chainages, circular/terminal circuits and explicit transfers (`metro/layout.go`) | Unify with the unused lightweight `infrastructure/metro/graph.go` schema; switches, branches, depots |
| Metro service scheduling | Exactly TWO physical trains initialized per line in `metro/runtime.go:New`; headway regulates these trains | Configurable fleet, depot insertion/removal, departure queue, service trips, timetable feasibility and recovery |
| Metro control/movement | Real acceleration/braking, section occupation, route/platform reservations, terminal loops and capacity | Segmented blocks and following multiple trains within a long station-to-station section; richer interlocking |
| Metro algorithm contracts | Worker controls speed ceiling/earliest departure; host retains safety | `metro/contracts.go` Timetable/Interlocking/Operation remain reserved interfaces; they do not replace the actual runtime |
| Shared travel demand | Real car/taxi/metro mode-share journeys; one-to-one executor/ledger audit | Common OD geography and richer demand models; no cross-mode trips, as currently intended |
| Ordinary car operations | Shared demand admission with physical road executor | Legacy random trips and lifecycle adapter remain coupled to road Simulation |
| Taxi operations | Lifecycle transitions extracted into `operations/taxi/lifecycle.go`; matching/supply/routing connected | Road adapter still orchestrates operational tasks; complete service boundary extraction |
| Taxi acceptance restriction | An accepted task locks the taxi immediately, including travel to pickup | Retain this invariant; pooling requires a distinct future task/stop-plan model |
| Pooling | Not implemented | Explicitly deferred; capacity, insertion, detour and multiple-task constraints |
| Metro passenger journeys | Real queues, boarding/alighting, capacity, transfer walking, cross-line planning and conservation | Richer pedestrian station layouts and platform crowding |

## Why there are only two trains

`metro.New` iterates over the first and last station and creates one train at each. Radial lines circulate through their end loops; the orbital line also starts with two trains. Changing headway or car count does not allocate additional train entities. With four lines, Map 3 has eight trains overall, two per line. A departure is an event performed by an existing train, not a newly created train. The current capacity-one section reservation also limits throughput, independently of headway.

## Physical heights and editable settings

The user-selected migration convention is **5 m per former level**, with no bridge structural thickness. Ground is 0 m; road bridge/tunnel planes are +5/-5 m. Map 3 metro tracks are -50/-55/-60/-65 m. JSON metro lines now store `elevation_m`; live overrides use `line_elevations_m`. Legacy integer fields remain compatibility/topological identifiers inside older contracts, not physical rendering filters. Exact elevation controls shared central crossing locks. This is a simplified travel-plane model, not a tunnel-shell or structural-clearance certification.

The map slices an inclusive `[from_m, to_m]` range. Ramp clipping inverts the actual smoothstep elevation profile; vehicle filtering uses actual interpolated elevation. Metro paths, stations, train cars and selected passenger paths follow the same filter.

Numeric settings accept typed values. Former convenience upper bounds are removed from UI and corresponding runtime settings. Finite values, positive speeds/intervals, nonnegative demand/dwell, integer entity counts, valid mode shares, capacity not below occupancy and other operational invariants remain enforced. Higher demand or fleet size does not imply unlimited machine capacity. Existing executor admission/backlog and algorithm-output bounds remain explicit runtime protections, not hidden UI clamps.

## Next implementation order

1. **Railway fleet and timetable service**: separate TrainVehicle from ServiceTrip, configure fleet/depot and departure queues, admit only into free platform/block, turnback/reuse/retire safely. Keep passenger tasks across fleet/headway changes. Add multi-block following before claiming high-frequency capacity.
2. **Durable experiment/replay foundation**: complete command/decision logs, RNG state, checkpoints and exact replay. Generalize multi-seed experiments to all capabilities and archive versions/configurations.
3. **Physical infrastructure boundaries**: unify rail graph schemas; split operations adapters from road executor; explicit obstacle/building/terrain data and stable 3D mobility IDs.
4. **Communications coupling**: implement the staged [base-station research design](communications-research-design.md), beginning with mobility traces and independent reference-model validation.
5. **Remaining strategy contracts**: general priority/yield, full demand generation and timetable proposals; retain host safety validation. Add pooling only as a deliberate separate business model.

Validation: metro/server/operations/infrastructure package tests, vet, selected road/taxi/interchange regressions, seven geometry checks and six desktop/helper tests passed. Packaged height smoke applied -52.25 m, 350 km/h, 13 cars, 4.2 m/s walking and 12x playback. Packaged exchange smoke retained the selected passenger itinerary with balanced demand. Defaults remain unchanged except the requested physical height migration. This was targeted validation, not a rerun of every Go test.
