# Interchange implementation summary

## Delivered

- Preserved Meridian and the four-cell preset ID; changed the latter's central
  diamond to a full cloverleaf with collector-distributor roads.
- Four left-turn loops, four direct right-turn ramps, four two-lane C/D roads;
  four peripheral three-arm folded diamonds remain.
- Final graph: 88 nodes, 128 directed ways. Central movement catalogue: 12 routes,
  comprising four straight, four left, and four right movements.
- Versioned parameter catalogue, reusable arc/cubic sampling, deterministic
  template compilation, explicit lane-connection validation and unit tests.
- Mainline through routes are signal-free. Flow-node exits and merges attach to
  specified outer lanes; no all-to-all lane fan-out at the cloverleaf.
- Limited-access roads exclude walking trip entry/exit. Vehicles traverse loop
  ramps, slopes and decks under the existing following/lane-change/reservation model.
- Layered display, C/D/loop/right-ramp road roles, and a central overview supporting
  all 12 route selections. Sampled curves use the backend path without UI spline
  reconstruction. Continuous-flow nodes render connected lane strips rather than
  the generic signal-intersection curb polygon.

## Important files

| Responsibility | Owning source |
|---|---|
| Design / references / limitations | `docs/interchange-system.md` |
| Profile and common-type catalogue | `configs/interchange-design.json` |
| Curve sampling and envelope checks | `scripts/interchanges/geometry.py` |
| Cloverleaf template and explicit lane wiring | `scripts/interchanges/cloverleaf.py` |
| Map composition and folded-diamond template | `scripts/generate_interchange_map.py` |
| Generated runnable map | `configs/presets/four-cell-interchanges.json` |
| Model contract and validation | `internal/model/types.go`, `graph.go`, `signal_plan.go` |
| Elevation / access / vehicle execution | `internal/model/traffic.go`, `internal/sim/simulation.go` |
| Layered map and route overview | `internal/server/web/app.js` |

## Validation

- Geometry sampling tests: arc endpoints, maximum sample spacing, cubic endpoints,
  and rejection of overlapping profile envelopes.
- Model tests: complete directed-road reachability, all 12 central route chains,
  no signal on central movements, through-route isolation, valid lane indices,
  restricted pedestrian access, and rejection of malformed connectors.
- Simulation test: live people enter loop ramps, climb/descend ramps, and traverse
  elevated decks; elevation remains in the configured range.
- Existing model, server and simulation regression suites remain applicable.
- Desktop integration checks both presets, authenticated API, SSE and shutdown.
- Desktop smoke selects the cloverleaf, switches layers, opens central details,
  iterates all 12 route choices and checks advancing simulation time.

## Boundaries

The geometry is a reproducible, executable simulation design. It is not a
construction-standard compliance certificate. Common types marked catalogue-only
are not implemented generators. Numeric profiles remain in world units. C/D
weaving is modeled but not eliminated; throughput claims require sustained demand
experiments. Calibrated acceleration/deceleration, lateral comfort, truck swept
paths, superelevation, real structural clearance and merge priority are future
model work. The central template does not silently change the original map.

## Operation

Use Settings → MAP → Four-Cell Interchange Network. Click the central interchange
for route details. Continue to start `desktop/dist/win-unpacked/Meridian Traffic.exe`;
keep its accompanying resource files in place. No installer is required.
