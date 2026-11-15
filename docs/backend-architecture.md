# Backend architecture

Status: implemented package-boundary refactor, 2026-11-15. This document supersedes earlier architecture inventories. Communications remains a design, with no placeholder package or empty interface added.

## Ownership and dependencies

- `cmd/traffic` only enters `app.Run`. `cmd/experiment` parses experiment inputs and uses the application factory.
- `app` assembles facilities, algorithms, the world and HTTP transport. It owns startup/shutdown; it does not perform vehicle motion.
- `simulation` owns the shared clock, the metro runtime lifetime and the shared-demand ledger. Its phase coordinator runs commands/decisions, demand/signals, road motion/arrivals, metro motion and completion accounting in order. Road callbacks execute under the existing command/snapshot lock, preserving atomic boundary snapshots.
- `foundation.Clock` owns fixed-step accumulation (50 ms), including residual time and reset. Algorithms cannot advance time.
- `infrastructure/road/interchange` contains the static macro-node/port projection. Actual hierarchical route search and lane expansion are in `road_scheduling/routing`.
- `infrastructure/metro` loads and validates JSON facilities, physical dimensions, height, closed tracks, transfer connections, chainages and directed section lengths. It never instantiates a train executor or holds occupancy. Operational defaults are plain `operations/metro.Config` data; their validation is pure.
- `road_scheduling` owns vehicle state, motion, lane/connector reservations, detector reads and signal execution. It receives a live metro admission/observation boundary and a world phase coordinator; it does not import or construct `metro_scheduling`.
- `road_scheduling/motion` contains pure acceleration/following/braking functions. `road_scheduling/intersections` contains the stateless pressure policy. Runtime detector collection and safety enforcement remain with the road executor.
- `metro_scheduling` owns trains, platform/block/route occupancy, physical turnback, fleet service execution and boarding. `operations/metro` owns passenger itinerary types, shortest-path computation and timed-transfer transitions, operational settings and detached views.
- `operations/demand` generates and releases mixed-mode tasks and audits executor identity/count conservation. Road-specific admission is supplied as a callback; rail admission uses the live world-owned metro instance.
- `operations/taxi` owns order/vehicle lifecycle and acceptance eligibility. `operations/taxi/dispatch` owns matching contracts, policies and proposal validation. Taxi route/admission adapters remain next to physical road agents because they must check real lane storage and arrival state.
- `algorithms/executor` is the single bounded external process implementation used by both generic algorithms and taxi dispatch. Domain adapters retain their existing JSON contracts and host validation. Desktop secrets are filtered from subprocess environments.
- `algorithms/experiments` takes an injected session factory and observations; it has no simulation/server/executor imports. `app/experiments.go` supplies the real world adapter. Current exported experiment metrics remain taxi-oriented; this refactor does not invent a complete multi-domain statistics platform.
- `frontend` embeds HTML/CSS/JS independently; `server` serves it and translates commands. Renderer code never computes traffic physics.

Dependency direction:

```text
cmd -> app -> server + simulation
simulation -> road_scheduling + metro_scheduling + operations + foundation
scheduling -> infrastructure + operations + algorithms + foundation
operations -> algorithms + foundation
infrastructure -> declarative operation config + algorithms + foundation
algorithms/experiments -> injected session contract (no world dependency)
```

`app/architecture_test.go` enforces the important import boundaries. No infrastructure or operational package can import a scheduler, world or server. Schedulers cannot import each other or the world.

## Preserved contracts

- JSON fields and HTTP/WebSocket commands remain compatible, including metro station details, complete passenger route overlays and the floating fleet window.
- A taxi is unavailable immediately after accepting an order, throughout pickup and dropoff, until completion/cancellation safely releases it. Pooling is not enabled.
- Live demand/fleet changes preserve accepted orders and passenger identities. Explicit reset/map change retains its existing reset semantics.
- Road/metro coordinates are metres; old level conversion remains 5 m. The renderer uses metre height slices and uniform X/Y scaling.
- Terminal direction locks, track occupancy, lane reservations and host result validation remain authoritative.
- The packaged executable remains `desktop/dist/win-unpacked/Meridian Traffic.exe`.

## Actual production source tree

```text
cmd/experiment/main.go
cmd/traffic/main.go
frontend/assets.go
internal/algorithms/executor/process.go
internal/algorithms/executor/process_unix.go
internal/algorithms/executor/process_windows.go
internal/algorithms/experiments/run.go
internal/algorithms/host.go
internal/app/bootstrap.go
internal/app/experiments.go
internal/foundation/clock.go
internal/foundation/geometry/pose.go
internal/foundation/kernel.go
internal/foundation/runtime.go
internal/infrastructure/metro/circular.go
internal/infrastructure/metro/layout.go
internal/infrastructure/road/crossroads.go
internal/infrastructure/road/geometry.go
internal/infrastructure/road/graph.go
internal/infrastructure/road/interchange/graph.go
internal/infrastructure/road/planners.go
internal/infrastructure/road/presets.go
internal/infrastructure/road/signal_plan.go
internal/infrastructure/road/traffic.go
internal/infrastructure/road/types.go
internal/metro_scheduling/controls.go
internal/metro_scheduling/network.go
internal/metro_scheduling/network_assembly.go
internal/metro_scheduling/runtime.go
internal/metro_scheduling/sections.go
internal/metro_scheduling/service.go
internal/metro_scheduling/station_details.go
internal/metro_scheduling/transfers.go
internal/metro_scheduling/types.go
internal/operations/demand/arrivals.go
internal/operations/demand/dispatch.go
internal/operations/demand/modes.go
internal/operations/demand/stream.go
internal/operations/metro/config.go
internal/operations/metro/routing.go
internal/operations/metro/transfers.go
internal/operations/metro/views.go
internal/operations/taxi/config.go
internal/operations/taxi/dispatch/dispatch.go
internal/operations/taxi/dispatch/worker.go
internal/operations/taxi/domain.go
internal/operations/taxi/lifecycle.go
internal/operations/taxi/vehicle.go
internal/road_scheduling/algorithm_extensions.go
internal/road_scheduling/algorithm_runtime.go
internal/road_scheduling/coupling.go
internal/road_scheduling/flow_reservations.go
internal/road_scheduling/intersections/max_pressure.go
internal/road_scheduling/kernel.go
internal/road_scheduling/lane_changes.go
internal/road_scheduling/lane_entry.go
internal/road_scheduling/mixed.go
internal/road_scheduling/motion/dynamics.go
internal/road_scheduling/parallel.go
internal/road_scheduling/phases.go
internal/road_scheduling/policies.go
internal/road_scheduling/routing.go
internal/road_scheduling/routing/routing.go
internal/road_scheduling/signal_types.go
internal/road_scheduling/signals.go
internal/road_scheduling/simulation.go
internal/road_scheduling/taxi.go
internal/server/server.go
internal/simulation/phases.go
internal/simulation/world.go
```

## Intended extensions, design only

Communications infrastructure belongs in `infrastructure/communications`, with runtime in `communications_scheduling` and application traffic in `operations/communications`. Shared terrain/materials belong in `infrastructure/environment`. See `communications-research-design.md`. These folders are not created until they contain real implementations.

The earlier recommended filenames are responsibility boundaries, not a requirement to create empty files. Fully physical depot tracks/shunting, durable checkpoint/replay, general multi-domain experiment metrics and communications physics are not claimed by this refactor. Current metro depots are logical storage with safe platform admission and real terminal-circuit turnback.

## Validation

The final release runs Go tests/vet, desktop backend/frontend-helper tests, packaged fleet and transfer smoke scenarios, and executable runtime identity checks. Results are recorded in `refactor-validation-2026-11-15.md` after execution.
