# External route/signal algorithms and shared demand

For the current three-mode source and additional runtime/scenario capabilities, see
[three-mode execution and extensions](three-mode-algorithms.md). The route/signal
protocol below remains compatible.

## Use in Settings

- **Walking speed:** 0.3-3 m/s, initially 1.4. Vehicle speed is constrained by the
  road's `speed_limit` in m/s. Preset ordinary-road limits are no more than 60 km/h;
  tighter ramps retain lower limits. There is no runtime global driving-speed cap.
- **Shared travel demand:** set trips/minute and taxi share. Apply changes to future
  journeys without resetting current trips. The Road/Taxi/Mixed selector is a
  convenient demand-share control, not a scene restart.
- **Taxi service:** adjust drivers or dispatch configuration live. Existing accepted
  tasks finish before retiring drivers leave. A taxi becomes unavailable at
  assignment, not only when the passenger boards.
- **Route and signal algorithms:** choose Native or a Python example. Parameters
  are JSON numeric fields, applied to both selected examples. Errors show native
  fallback status. Native remains the default.

Enabling the Taxi/Mixed shortcut for the first time preserves existing private
vehicles and adds an initial sample of up to eight journeys, matching the previous
startup demo behavior. Subsequent changes do not inject another sample. Applying
Shared travel demand directly uses only the configured Poisson stream. Explicit
Reset and map replacement still intentionally create a new run.

## Worker installation

Put a manifest and code in `experiments/algorithms` before starting the backend:

```json
{
  "id": "research.route",
  "name": "Research route planner",
  "capability": "route",
  "algorithm_version": "1",
  "contract_version": 1,
  "command": ["python", "route.py"],
  "working_directory": ".",
  "wall_timeout_ms": 1500
}
```

`capability` is `route` or `signal`. The directory is relative to the manifest.
Timeout bounds are 50-10000 ms. Each invocation receives one JSON request on stdin
and must write exactly one JSON response to stdout; diagnostic logs use stderr.
Input is capped at 16 MiB, output at 4 MiB and stderr at 16 KiB. Process cancellation
terminates its process group/tree; Windows workers do not open console windows.
Python must be installed for the supplied Python examples. Native requires none.

The host discovers manifests with `-algorithm-dir` (default
`experiments/algorithms`); the desktop passes the packaged resource directory.
`GET /api/algorithms/workers` lists them. Existing taxi dispatch manifests remain in
`experiments/dispatch` with their existing protocol.

## Shared envelope

Requests include `contract_version`, `algorithm`, `algorithm_version`, `capability`,
`stamp` (run/world/topology), `request`, `input_tick`, `seed`, `parameters` and `payload`.
Responses echo every identity field except seed/parameters, and provide `payload`.
A wrong version, request ID, stamp or input tick is rejected. A change committed
while a worker runs makes the old proposal stale. Reset cancels pending work.
The simulation mutex is released while workers run; physical time waits, so results
are applied at a reproducible simulation boundary rather than racing moving agents.

## Route payload

Input contains `tasks`, `ways` and `intersections`. Each task provides entity ID,
start/end road, current lane/distance, destination distance, and baseline/alternative
hierarchical routes. Candidate routes already include actual internal interchange
lanes. A worker may select a candidate or compute a new expanded path from topology.

Return `{"routes":[{"entity":123,"steps":[{"way_id":"...",
"entry_lane":0,"exit_lane":0}, ...]}]}` with one result for every input task.
The host verifies road endpoints, continuity, every lane connector, bounds and
available lane-change preparation distance. No direct vehicle positions may be
returned. A valid result replaces the real remaining route; it is not merely a
visual overlay or an unused cost proposal. A failed batch falls back to validated
native baseline routes. The initial demand/dispatch route and matching ETAs remain
native; the worker hook is live replanning, not an external matching/ETA service.

Up to eight eligible vehicles are processed per five simulated seconds in rotating
batches. Vehicles changing lanes or traversing a connector are not replanned.
`route.py` selects the alternative when available; set `prefer_alternative` to 0
for the native candidate. This is a demonstrator, not an optimal dispatch strategy.

## Signal payload

Input tasks contain intersection, current phase/stage and phase options with minimum
and maximum green seconds and pressure. Return one
`{"intersection":"...","phase":1,"green_s":20}` per task under `signals`.
Indices and green bounds must be valid. No arbitrary conflicting movement list,
instant phase switch, yellow duration or all-red override is accepted.

The chosen phase is consumed by the real scheduler after its current green and
clearance. Minimum green, maximum green, maximum-red anti-starvation and existing
amber/all-red transitions remain host-owned. Proposals expire if not refreshed;
errors or selecting Native immediately restore native selection. Unsignalized flow
nodes retain reservations and have no artificial all-red wait.

## Shared journeys

`internal/operations/demand.Stream` owns seeded Poisson arrivals, origin/destination,
mode choice and journey accounting. The road adapter admits private cars or creates
taxi orders. Private admissions retry while blocked; accepted mode never changes
when the ratio slider changes. Completed/cancelled records are pruned after totals
are accumulated. Snapshots expose demand config and counts; cars and orders expose
`journey_id` links. Initial legacy private trips are preserved but are outside the
new source's generated count. This prevents retroactive counting or task deletion.

`set_demand` accepts `demand: {trips_per_minute, taxi_share, seed}`. Omitted seed
preserves the stream; an explicit changed seed reseeds future draws only. Rate
changes reschedule the next arrival, leaving queued/active journeys intact.
`set_taxi_config` updates dispatch settings without changing the demand ratio.
There is still no metro executor or car/metro transfer; metro allocation is not
exposed as a working option.

## Acceptance checks

Tests invoke real child processes to prove that external paths and phases alter
runtime behavior. Timeout, crash, malformed JSON, stale envelope and invalid phase
outputs must leave a running native fallback. Mixed-service tests require both car
completion and taxi delivery, shared population, invariant accounting and task
preservation during ratio changes. Desktop smoke additionally runs the packaged
Python workers, mixed demand, walking controls and the existing taxi pickup/dropoff
and interchange UI checks.

## Packaged verification record

The final Windows unpacked executable passed the extended desktop smoke run:
real taxi service completed two deliveries and picked up four orders; shared demand
reported 12 generated = 0 queued + 8 active + 2 completed + 2 cancelled, with both
road and taxi journeys present. Both packaged Python capabilities applied once,
with zero fallback/error results. Walking speed changed to 1.8 m/s through the
control API, and live mix changes preserved run/time. The runtime binary SHA-256
matches the copy bundled beside the fixed-path EXE.

The final check batch passed road/interchange/operations tests, geometry tests and
static analysis. An HTTP catalog path collision found in that batch was fixed and
server/desktop integration retested. A packaged near-entry destination validation
failure was fixed with a regression test and the worker tests plus packaged smoke
were rerun; unrelated road suites were not repeated after those targeted repairs.
