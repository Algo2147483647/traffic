# Three-mode demand and executable algorithm extensions

## Shared journeys

`set_demand` accepts `trips_per_minute`, `taxi_share`, `metro_share`, and `seed`.
Car share is `1 - taxi_share - metro_share`. Invalid/nonfinite shares and unavailable
metro networks are rejected before changing state. Mixed mode defaults to
40% car / 30% taxi / 30% metro when the map has a metro network.

One seeded stream selects a mode once. Metro OD is sampled from served directed
station pairs on a single line. Changing shares/rates affects future journeys;
accepted orders, vehicles and onboard passengers continue. Metro disabling pauses
its executor without losing riders. Existing standalone riders finish, but the
standalone metro arrival generator stops once shared demand is enabled. Settings
disable the redundant standalone metro demand input in that state.

A metro passenger carries the shared journey ID from platform queue through actual
boarding and alighting. Only alighting at the destination completes the journey.
Private journeys own one physical vehicle; taxi journeys own one order. The taxi
locks at acceptance, including travel to pickup; no pooling is enabled.

Snapshots expose `demand.modes` with generated/queued/active/completed/cancelled
counts, `conserved`, and `errors`. The audit checks both the arithmetic identity
and one-to-one links to live private vehicles, taxi orders and metro passengers.
Legacy trips created before shared demand are outside that ledger; they remain in
their executor's own counters. A journey does not transfer between modes.

## Runtime worker contracts

Use `set_algorithms` with choices named `route`, `signal`, `demand`, `supply`,
`motion`, and `metro`. Each choice is `{ "id": "python.motion", "parameters": {} }`.
An empty/native ID restores the built-in behavior. Existing taxi matching retains
its compatible dispatch protocol and Settings selector.

Workers run as bounded batches every five simulated seconds, outside the world
lock. The simulation-time decision barrier makes physical state stable while the
worker runs; controls and snapshots remain available. Full request envelopes are
validated. Crashes, deadlines, malformed JSON, stale epochs and invalid proposals
record a fallback event and retain native behavior. No process starts per vehicle
per physics tick.

| Capability | Input | Output and actual effect |
| --- | --- | --- |
| Demand | Queued journeys, road stops, served metro pairs | `journeys`: same IDs/modes, valid replacement OD and depart_at_s between the original arrival time and five seconds after input. Native seeded rates and mode shares remain authoritative; queued trips wait for the batch and are released with native OD on failure. |
| Supply | Idle vehicle IDs, stops, current orders | `targets`: vehicle/stop pairs. Validated hierarchical routes replace only idle, unassigned taxi tasks. Busy/retiring/turning/lane-changing vehicles are rejected. |
| Motion | Vehicle state, routes, roads, native speed/lane proposals | `motions`: entity, speed_mps, lane, optional merge_priority [-1,1]. Speed ceilings feed actual following; safe adjacent lane proposals start a maneuver; priorities order new merge reservations. Acceleration, braking, road limits, route preparation, gaps, conflicts and existing grants remain authoritative. |
| Metro | Full metro snapshot | `trains`: train, speed_mps, depart_after_s. Valid until five seconds after input. Actual motion uses the speed ceiling; earliest departure can delay a train by at most five seconds. Dwell, boarding, capacity, occupied blocks and route authority cannot be bypassed. |
| Route / signal | Existing lane itinerary / phase tasks | Existing fully expanded route and safe phase-scheduling contracts. |

Metro controls are short-horizon departure regulation, not a replacement for safe
interlocking. Road motion uses batched desired controls, not plugin ownership of
physical positions. Omitted motion/train proposals use native controls. Native code
extensions also remain available through `RegisterStrategies` and `RegisterPlanner`.

## Scenario-time workers

`geometry_algorithm` and `phase_algorithm` in a road JSON may select registered
`geometry` / `phases` process manifests. They receive a CityConfig and return a
candidate CityConfig. Geometry candidates undergo full graph construction and
movement/interchange validation before use. Phase workers can replace traffic
control plans only; original topology and metro layout are retained. Conflicting
movements fail graph validation. Scenario errors reject the candidate rather than
silently loading a different map. These workers never edit a live graph.

Runnable Python alternatives for all new capabilities are in
`experiments/algorithms/extensions.py`; individual JSON manifests are bundled in
the desktop. Geometry mirrors a road layout; phases changes offsets/reorders
explicit plans; demand reverses valid OD and defers admission by one second; supply targets waiting pickups; motion
uses 90% desired speed and a deterministic merge preference; metro regulates empty
train departure and speed. They are experimental examples, not optimized policies.

## Verification and scope

Tests cover seeded 40/30/30 proportions, three real transport completions, live
share changes and reset accounting, all four live worker integrations, invalid
busy-taxi/motion/train proposals, timeout/crash/malformed/stale fallbacks, and
external scenario builders. The final verification also runs existing avoidance,
interchange, hierarchical routing and taxi lifecycle regressions.

The broader Stage 6 experiment roadmap still includes full checkpoint/resume,
recorded-decision deterministic replay, and a generalized multi-seed experiment
runner. These are separate from the executable algorithm-extension work above;
this change does not claim that those experiment tools are complete.

Long-run testing also exposed and fixed a terminal-circuit deadlock: the physical
twin-track layout now reserves separate arrival/departure berths, reserves the
exit berth before entering the loop, and releases the arrival berth only after
the tail clears it. A regression first runs Map 2 empty for 1,500 seconds, then
adds every directed station pair and requires every passenger to finish.

Final validation (2026-11-15): `go test ./...` and `go vet ./...` passed;
seven interchange geometry tests and both desktop backend tests passed. After the
terminal berth correction, metro/road regression tests and the full Go suite passed
again. Packaged Map 2 smoke completed two taxi deliveries, four pickups, 220 expanded
lane routes, two metro deliveries and 15 train departures; shared-ledger accounting
was balanced. External route/signal smoke recorded two applied batches and zero
fallbacks. Separate live-worker tests exercised demand, supply, motion and metro
alternatives, plus failure handling and both scenario-builder capabilities.
The fixed unpacked Windows executable was overwritten, its bundled engine hash
matched the build, and the application was launched and responding.
