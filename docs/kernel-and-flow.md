# Command, event and flow-reservation kernel

## State owner and identities

`Simulation` remains the single state owner. HTTP handlers and Go callers submit
commands; only the owner applies them under its mutex. No module keeps a second
vehicle position or clock. Run identifiers increase on reset/mode restart; topology
revision increases on graph replacement. World revision increases on committed
commands and physical ticks. It is an optimistic-concurrency token, not elapsed time.
The numeric wire ID is retained; use `(run, id)` when retaining an entity reference.
Run numbers are scoped to one backend process, so clients reconnecting after process
restart must fetch a new snapshot and discard old in-memory envelopes.

## Immediate controls

The existing `/api/control` payloads remain compatible. Their Go setters now pass
through `Execute(Command)`, which queues and drains at the current safe boundary.
Controls work while paused. `step` is immediate only; recursive scheduled steps are
rejected. A scheduled pause encountered during a long advance stops at that boundary.
Map pointers are immutable host inputs, never accepted from remote command JSON.

## Scheduled commands

Fetch `/api/snapshot` and copy its `kernel` stamp. Submit, for example:

```json
{
  "stamp": {"run": 1, "world": 20, "topology": 1},
  "due_tick": 100,
  "priority": 0,
  "entity": 0,
  "strict_world": false,
  "payload": {"kind": "set_spacing", "value": 3}
}
```

`POST /api/commands` returns HTTP 202 and the assigned sequence. The stamp and tick
above are illustrative: use current values from the running backend. A sequence is
an acceptance receipt, not proof of execution. `/api/events` records application,
rejection or reset invalidation. Ordering is due tick, priority (lower first), entity,
then sequence. The queue is bounded to 4096 entries and one million future ticks.

Run and topology must match at submission and execution. With `strict_world: true`,
world must match too; use it for immediate proposals that depend on an exact input
state. Future scheduled controls normally set it false. Reset invalidates queued
work even when numeric entity IDs are reused. Paused wall-clock ticks still drain
commands already due; simulation time does not advance to reach future commands.

Supported kinds: `pause`, `resume`, `reset`, `set_time_scale`, `set_spacing`,
`set_load`, `set_mode`, `set_taxi_load`. Values and mode configuration use the fields
of `internal/road.Command`. Unsupported kinds and nonfinite inputs are rejected.
The existing preset endpoint supplies validated immutable graph replacements.

## Tick and arrival ordering

Each fixed 50 ms step runs:

1. Due commands and stale-result validation.
2. Due taxi policy decisions (pending external work does not hold the mutex).
3. Demand, admission and ordinary-trip recycling.
4. Lane-change progress, signals, lane preparation and flow reservations.
5. Road motion followed by already-active connector motion.
6. Arrival envelopes in stable entity/sequence order.
7. Taxi dwell/accounting, tick and world-revision commit.

Arrival envelopes carry input stamp, entity, road, route index and taxi task ID.
The commit handler rejects obsolete identity/route/task combinations before calling
road transition or destination handling. Taxi business transitions still enforce
one accepted task through pickup, boarding, passenger travel and alighting.
Snapshots copy mutable routes and algorithm parameters. Journal events include
stamp and tick; its retention is 2048 events. This is diagnostics and deterministic
commit ordering, not the full durable replay facility planned for a later stage.

## Continuous flow

A lead vehicle requests an unsignalized connector before its comfortable stopping
distance. The reservation blocks competing connectors and insertion into the
receiving lane; lane changes cannot steal it. The reservation is revalidated against
route and run/topology, retained during connector traversal, and released on entry.
Downstream desired-speed and hard geometric/braking envelopes remain host-owned.
A vehicle that lacks a grant brakes; an unobstructed granted vehicle carries speed
across the boundary. Actual downstream congestion, lower limits and inadequate
storage still constrain it. The current conservative policy reserves a single
connector at a time and serializes competing merge approaches; temporal reservation
optimization and coordinated short-link chains are separate future improvements.

## Verification

- Foundation ordering and revision validation.
- Actual delayed controls, paused control processing, same-batch reset invalidation,
  stale arrivals, scheduled pause and detached snapshots.
- HTTP scheduled-command acceptance, execution and stale-run rejection.
- 1x/2x/10x physical equivalence at equal simulation time.
- All 36 interchange routes without intermediate flow-node cruise-speed loss.
- Entry protection, route-change release, competing merge/queue safety and drainage.
- Existing full road/taxi tests, including all movements, hierarchical pickup/dropoff,
  sustained service, busy-vehicle exclusion and external-worker reset invalidation.
