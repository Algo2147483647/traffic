# Taxi mode: implementation and operation

## Delivered

Settings > MODE selects Traffic or Taxi. Switching starts a fresh scenario; reset
retains the selected mode and taxi configuration. Traffic restores the original
population and spawn rate. Both existing maps remain available. Taxi mode uses a
separate persistent fleet, not pedestrians repeatedly converted into vehicles.

The settings window can be dragged by its title bar, resized using its lower-right
corner, and maximized/restored. Its body scrolls independently. Keyboard arrows on
the title or resize handle provide equivalent movement/size controls.

Drivers (1-200) and orders/minute (0-60) have paired sliders and numeric inputs.
Apply live updates supply and demand without resetting time, existing orders or
trips. Extra drivers enter only when safe road space is available. When reducing
supply, excess drivers finish their current service/route before leaving; active
and retiring counts are shown. Zero order frequency stops new arrivals.

Dispatch & experiment contains seed, interval, algorithm and numeric JSON parameters.
Restart taxi scenario applies these settings as a fresh scenario.
An active order is not silently transferred when a user edits settings. Map changes
also reset the scenario and invalidate outstanding policy work.

Taxi states are Idle, ToPickup, Boarding, WithPassenger and Alighting. Actual lane
arrival drives the order lifecycle. Cars retain position and velocity constraints,
turn connectors, receiving-lane reservations, signal compliance and bridge/ramp
layers. Arrival estimates do not complete orders. Unpicked orders cancel after the
configured deadline; completed fares settle once. Idle vehicles cruise to service
stops. Pickup/dropoff dwell defaults to four seconds.

Stops are generated on trip-accessible surface roads outside the limited-access
interchange groups. Current implementation is an **in-lane stop model**, not a curb
bay: stopped vehicles occupy their lane and followers obey normal spacing. No car
teleports to a roadside bay. General curb bays, capacity scheduling and new local
street geometry remain separate map work. Stop markers indicate waiting/assigned
orders; click a marker for order details or a vehicle for its task and road route.

Taxi colors: gray idle, gold pickup/boarding, blue occupied/alighting. Lower-left
statistics distinguish fleet count, created/picked/completed/cancelled orders,
mean picked-up wait and solver latency. Export run saves city geometry, configuration,
current physical snapshot, retained order timestamps and cumulative counters.

## Dispatch extension

The native registry in `internal/dispatch` exposes a Factory/Policy.Decide interface
with detached input values. Built-ins: nearest ETA (oldest-order priority), FIFO,
greedy global ETA, Hungarian rectangular assignment, batch Hungarian (at least
five seconds), and a parameterized ETA/wait/idle/income score. No geographic balance
term is claimed by the current score policy.

Add local worker manifests under `experiments/dispatch`; packaged desktop discovery
uses `resources/experiments/dispatch`. The shipped wait-balanced Python example
uses the standard library and an available Python interpreter. Native policies need
no Python installation. See the guide in that directory for exact protocol fields,
manifest format, seed behavior and registration instructions.

The implemented external protocol is one-shot JSON per decision, rather than the
persistent initialize/ready handshake in the architecture target. State may be held
by native instances; external examples are pure functions. Restarting experimental
workers each decision gives simple process lifetime and cancellation semantics.

Candidates include all eligible idle taxis for the oldest 64 waiting orders, using
legal directed routes. Route choice uses available congestion routing; the exported
ETA itself is the free-flow travel-time estimate, not calibrated queue-delay ETA.
Distances use the map's world_units_per_km conversion. Compare actual waits, not just
predicted ETA, when assessing policies. Units and speed parameters remain synthetic.

Host validation rejects foreign IDs, duplicate vehicles/orders, unreachable pairs,
wrong request identity, malformed JSON and excessive output. Invalid proposals are
not partially committed. Worker timeouts and crashes leave orders waiting; three
consecutive failures pause the simulation and expose the diagnostic. There is no
silent fallback policy. The UI remains responsive during solver work; simulation
time waits at the decision boundary. Reset/mode/map changes cancel that work.

Workers receive no desktop authentication token. Native code is trusted and
cooperative; use external workers for enforceable wall-time limits. Process groups
or taskkill /T clean up ordinary descendants; this is not a malicious-code sandbox.

## Experiments

`go run ./cmd/taxi-lab` runs in a separate program from the desktop and accepts map,
algorithms, paired seeds, duration, fleet, demand and output prefix. It writes JSON
and CSV with configuration, map/engine hashes, cohort counts, open orders at the
horizon, waits, solver failures and fleet statistics. Independent demand and cruise
random streams keep assignments from changing the generated OD sequence.

A Windows binary can be built with:

```powershell
go build -o desktop/runtime/taxi-lab.exe ./cmd/taxi-lab
```

The CLI is the experiment entry point in this release; a GUI experiment-job manager,
confidence intervals, zonal demand/relocation, mixed background-traffic mode and
full checkpoint replay remain future design stages. These features are not exposed
as completed controls. Existing Traffic and Taxi modes are independently usable.

## Validation

- Automated complete taxi pickup/boarding/service/alighting cycles and traffic-mode
  restoration, including reset with taxi settings retained.
- All 36 central/peripheral interchange movements driven by taxi road agents.
- Sustained multi-taxi service verifies short-link lane preparation; a regression
  where vehicles waited indefinitely at green lane-count transitions was fixed.
- Deterministic demand, cancellation idempotence, no teleport on cancellation.
- Native matching uniqueness, optimal-assignment reference fixture and forbidden
  pairs; worker protocol and timeout termination tests.
- Pending algorithm work does not hold the simulation mutex; mode reset prevents
  late results from affecting the new scenario.
- Desktop smoke uses the actual mode control, completes trips, restores Traffic,
  then loads and assigns orders with the experimental Python worker.
- Separate paired CLI runs verify native and experimental service outcomes. This
  is a functional check, not an algorithm performance claim.

The Windows unpacked executable keeps its original path. macOS source compatibility
is checked by cross-compilation; a macOS GUI execution must be tested on macOS.
