# Max-Pressure scheduling

The adaptive controller uses a separate, stateless `MaxPressureScheduler` for
each intersection decision. Webster timing still supplies safe minimum, design,
and maximum green durations; it does not affect phase ranking.

## Queue and pressure definition

A queue snapshot is measured once per simulation update from the 180-unit
detector zones:

- `Q_in(from, to, lane, target)` is the number of driving or stopped vehicles
  on an incoming lane whose next route way is `to` and whose lane-valid route
  requires the displayed receiving-lane connector. Final-leg vehicles that can
  accept any receiving lane contribute to every applicable target set.
- `Q_out(to, lane)` is the number of driving or stopped vehicles near the start
  of an outgoing lane. This is a receiving-occupancy/spillback estimate.

Including approaching vehicles in detector queue length lets the signal respond
before demand reaches the stop line. The simulation has each vehicle's route, so
incoming demand is movement-specific rather than inferred from approach totals.

For a configured movement `m`, `O_m(i)` is the set of downstream lanes reached
by explicit `lane_connections` from incoming lane `i`. Pressure is evaluated per
source lane before being summed:

```text
incoming(m) = sum(Q_in(m.from, m.to, lane) for lane in I_m)
outgoing(m) = sum(average(Q_out(m.to, target) for target in O_m(lane)) for lane in I_m)
weight(m)   = sum(max(0,
                  Q_in(m.from, m.to, lane)
                  - average(Q_out(m.to, target) for target in O_m(lane)))
                  for lane in I_m)
```

The downstream value for each source lane is averaged across only its reachable
targets. This models shared receiving capacity without allowing congestion in
an unrelated downstream lane to distort the source lane's score. Negative
per-lane differentials are clamped to zero because displaying green does not
require a vehicle to enter a blocked exit.

For every feasible phase `p`:

```text
pressure(p) = sum(weight(m) for every movement m permitted by p)
```

Generated plans store geometrically distinct lane connectors as separate phase
entries. Before scoring, entries with the same incoming way, outgoing way, and
source lane are recombined, with their displayed receiving lanes treated as
alternatives. This prevents a route queue from being counted once per target
lane while retaining connector-level conflict detection and signal permission.

No demand score, phase-order bonus, or timing-plan weight is added to pressure.
Every configured phase, including the phase just served, is evaluated at every
decision step. The phase with the greatest pressure is selected unless another
phase has reached the maximum-red limit of one design cycle. The oldest overdue
phase wins that decision, preventing two sustained high-pressure phases from
starving every other movement and locking a closed road network.

## Timing and data flow

```text
vehicle states and routes
  -> immutable incoming/outgoing QueueSnapshot
  -> movement pressure breakdown
  -> all phase pressure totals
  -> deterministic maximum selection
  -> green / amber / all-red runtime state
  -> snapshot API and vehicle movement permissions
```

Pressure is refreshed on every simulation update. The displayed next phase can
therefore respond immediately to changing demand, including demand changes
during amber and all-red. The runtime keeps the active phase, last-served phase,
and pending next phase as distinct state. The pending phase is committed exactly
once at the end of all-red clearance. When an intersection has alternatives,
the phase that just completed green is excluded from that immediate selection;
otherwise a dominant queue could repeatedly clear and reselect itself, defeating
maximum green and starving every other phase. An active phase observes its
minimum green, may gap out when its pressure is zero and another phase has
demand, and may extend in four-second increments only while it remains the
unique maximum and has not reached maximum green.

Stage timers consume simulation-time deltas rather than wall-clock time. A
single large step may cross green, amber, and all-red boundaries, and any unused
portion of the delta is carried into the newly selected green. Stop-line
decisions use the same end-of-step simulation instant for fixed-time and adaptive
controllers. Pausing advances neither controller, while time scaling advances
both by the same scaled delta.

## Determinism and edge cases

- Equal maxima use cyclic configuration order starting immediately after the
  phase just served. This is deterministic and rotates all phases under zero
  demand.
- A completed phase cannot immediately follow itself when another phase exists;
  single-phase intersections continue to select their sole phase.
- A phase at its maximum-red limit takes priority over higher-pressure phases;
  equal overdue ages use the same deterministic cyclic order.
- With no previous phase, the lowest configured phase index wins a tie.
- A plan with no phases returns phase index `-1`; the intersection model does
  not install an adaptive runtime for it.
- Missing, negative, NaN, or infinite queue values are treated as zero.
- Duplicate or negative lane indices are ignored defensively. Normal city
  loading validates lane indices and phase movements before simulation starts.
- Maximum-green bounds ensure a dominant phase is reconsidered periodically;
  amber and all-red clearance are never skipped.

## Validation

`internal/road/max_pressure_test.go` covers full phase evaluation, exact
lane-connection targets, target-lane demand, incoming and outgoing terms, a
demand shift, downstream congestion, deterministic ties, maximum-red fairness,
empty/invalid input, and application of the new
maximum at an actual signal decision boundary. `internal/road/simulation_test.go`
retains the seeded fixed-time/adaptive comparison, active-phase connector, and
green-bound behavior tests. `internal/infrastructure/road/graph_test.go` verifies maximal safe
movement sets for representative four-way and three-way junctions.
