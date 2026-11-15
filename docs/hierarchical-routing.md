# Hierarchical interchange routing

## Runtime ownership

`internal/road_scheduling/routing/routing.go` owns the reusable two-level
planner. `internal/road_scheduling/routing.go` supplies the congestion snapshot, experimental
road costs and physically bounded lane-preparation rule. The same planner serves
ordinary vehicle spawning, taxi pickup, taxi delivery, idle cruising and immediate
congestion rerouting. Accepting an order still excludes the taxi from all further
assignment until its current task is complete.

## Search and expansion

1. Build immutable lane-connector adjacency from the validated road graph and
   interchange ownership. Rebuild it when the simulation changes maps.
2. Freeze a positive finite cost for each directed road for this request. The
   standard cost is travel time weighted by occupancy/reservations; RouteCost
   profiles can replace that objective.
3. Run outer Dijkstra over ordinary road states and directional macro boundaries.
   A state includes its receiving lane. A single unrestricted interchange vertex
   would lose entry/exit restrictions, so each macro node retains its ports.
4. At an interchange ingress, run local Dijkstra inside that node. Stop at every
   reachable exit or the requested internal destination. Retain each transfer's
   exact lane path and total cost; do not search through another macro node here.
5. Search those transfers as macro arcs, then expand the chosen arcs into Steps.
   Each Step names a directed road, EntryLane and ExitLane. Leg ranges retain the
   macro traversal provenance. Adjacent macro legs share a boundary road but the
   flattened path contains it once.
6. Commit the expanded road/lane arrays to the vehicle. The executor uses existing
   geometry and physical motion. Entering a ramp never teleports to its exit or
   bypasses signals, merge priority, gaps, connector legality or receiving storage.

The source road is already entered and is excluded from search cost; each following
road is charged once, including macro exit roads. Costs choose a route; they are not
a promise of real arrival time under future traffic or signal changes.

```text
ordinary road -> C(entry lane -> internal transfer -> exit lane) -> link road
              -> N(entry lane -> internal transfer -> exit lane) -> destination

execution: road/lane -> approach/lane -> ramp/lane -> merge/lane -> link/lane ...
```

## Lane preparation and edge cases

The host first searches for a continuous legal lane-connector path. If no such
path exists, a second hierarchical query permits entry/exit lane differences when
the road has enough usable length for the required number of lane changes. The
calculation includes lane-change duration, speed, safety gap and vehicle length;
the first road uses the vehicle's current position. This is a conservative policy
preference, not a global optimum over all possible lane-changing trajectories.
The executor performs changes one adjacent lane at a time after checking gaps.

The actual receiving lane is separate from the later required exit lane. This is
necessary where a two-lane branch feeds a three-lane approach. Requiring the exit
lane at the entry connector can otherwise cause a permanent wait at a flow node.

Endpoints may be inside the same or different interchanges. A destination behind
the vehicle on the same directed road requires a nonempty return cycle; the initial
state is distinguished from a later return. A forward same-road trip can be direct.
BlockedFirst excludes a downstream road only on the first transition. Unreachable
queries fail explicitly instead of returning an incomplete macro-only route.
Final curb-side lane alignment remains the taxi stop controller's responsibility.

## Code extension and inspection

`interchange.NewPlanner(graph).Route(Request)` is the reusable planning entry point.
Request supplies endpoints, initial lane, direct-trip permission, first-edge
exclusion, road-cost callback and optional lane-preparation authorization.

Route contains:

- Steps: fully expanded road IDs with zero-based entry/exit lanes.
- Legs: interchange ID, entry/exit port, inclusive Step index range, and a flag
  for endpoints inside the interchange.
- Cost: accumulated weight of the expanded path after its source road.
- MacroSettled / InternalSettled: outer and local search work for diagnostics.

For internal endpoints, a Leg's port fields identify the adjacent local junction;
EndpointInside distinguishes that partial traversal from a boundary-to-boundary one.
HTTP snapshots expose route_entry_lanes and route_exit_lanes aligned with route.
These arrays are copied out of simulation state. UI rendering remains compatible.

Native experiments use the existing versioned Strategies.RouteCost hook. It affects
ordinary roads and internal transfer costs consistently. It cannot invent missing
connectors or change physical traffic rules. Graph.RouteBetweenWays remains an
infrastructure utility and is no longer a runtime task-planning fallback.

Only topology is cached between requests. Local transfers are recomputed against
fresh weights. Outer search generally visits fewer states, but no universal speedup
is claimed: repeated local searches can cost more than a flat search on small maps.
Batch-level transfer reuse would require a shared immutable cost snapshot and
explicit invalidation for topology, policy, weights and lane-preparation rules.

## Acceptance checks

- 826 deterministic queries compare hierarchical reachability and cost with a
  detailed lane-graph reference, including custom weights and blocked first edges.
  Of 540 successful routes using macro legs, 538 settle fewer outer states.
- Internal endpoints, same-road cycles, direct trips, deterministic ties and fresh
  query weights have dedicated checks.
- A constrained synthetic interchange rejects an impossible port transfer until
  a real on-road lane change is authorized; blocking its entry makes it unreachable.
- Every one of the 36 cloverleaf/trumpet catalogue movements is planned for a taxi,
  physically driven, and checked for every advertised road and actual arrival.
- A real dispatched task travels through C and N for pickup, then E and S for
  delivery; boarding, occupied travel, alighting and completion are all observed.
- Sustained fleet service, busy-taxi rejection, original-map behavior, mode changes,
  live driver/demand controls and external dispatch remain regression checks.
- The unpacked Windows executable stays at
  `desktop/dist/win-unpacked/Meridian Traffic.exe`. Desktop smoke checks also verify
  that actual taxi tasks contain expanded entry/exit lane arrays.
