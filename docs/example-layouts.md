# JSON examples and physical metro circuits

## Source of truth

The lowercase `example/` folder is the runtime source of preset materials.
`Map1/preset.json` and `Map2/preset.json` identify each example and its road and
metro JSON. The road file names its local metro file; inconsistent references,
duplicate preset IDs, nonlocal files and malformed physical layouts are rejected.
The CLI discovers example subdirectories; Electron ships the same hierarchy under
`resources/example`. Edit JSON and restart to load changes, without recompiling.
There is no production-code branch for an AC junction or a radial metro layout.
A road-only configuration without a metro file has no synthetic metro overlay.

Map 2's geometry profiles and generator site positions/port links also live in its
example folder. The optional generator is generic geometry code, not the loader.
Generated road points and metro track points are already persisted in JSON.

## Map 1 roads

The centre is (3000, 3000) metres. Sixteen directed eighth-circle roads form a
2,000 m centreline-radius ring, with the existing four cardinal axes meeting at AC.
The four outer districts are mirrored around both axes. All sixteen ring roads have
two 3.5 m lanes and a 60 km/h limit. Bounds are 6 km by 6 km. Road graph reachability,
lane connections, turn conflicts and signal generation use the actual sampled
arcs. The circle is not a decorative canvas overlay.

## Metro physical units

The selected A-type train uses 22.8 m by 3 m cars, six by default: 136.8 m per train.
Cars have per-car occupancy and per-car poses sampled along the physical route.
There is no minimum pixel size for cars, tunnel walls or tunnel spacing. Zooming
changes all three by the same scale. At overview scale trains may be very small;
zooming in reveals the individual passenger dots. Walking dots have half their
previous radius and no black outline.

Examples use 5.8 m tunnel inner diameter, 6.4 m outer diameter and 15 m straight
track centre spacing. These are explicit design assumptions for this simulation,
not a claim that one national standard prescribes a universal tunnel diameter or
spacing. The 5.8/6.4 m section has an A-type metro precedent in Beijing Line 16:
https://jtw.beijing.gov.cn/xxgk/xwfbh/201912/t20191209_1007465.html
A-type rolling-stock dimensions are covered by the official operating specification:
https://xxgk.mot.gov.cn/jigou/ysfws/202302/t20230227_3765192.html
The 300 km/h setting remains the requested simulation ceiling, not a certification
of A-type vehicles or these civil works for that speed.

The renderer strokes each real tunnel's outside and hollow inside, then places
cars on that track. The simulation and renderer share the JSON route; there is no
independent cosmetic offset. Car centres and headings follow curves individually.

## Terminal circuits and operations

Each line has one closed sampled route: outbound track, far-end balloon loop,
inbound track, near-end balloon loop. Stations retain their specified positions;
loop infrastructure extends beyond terminal stations. Map 1 retains three 8 km
station corridors at 60-degree bearings, with 2 km station spacing. Map 2 retains
four stations inside its six-kilometre road bounds. Route chainages are explicit
JSON values, validated for order, range, continuity and closure.

At terminals, passengers alight/board using their existing identities and normal
capacity/dwell constraints. Trains traverse the actual connecting loop at at most
30 km/h with acceleration and braking, then continue on the opposite track. They
never teleport or reverse their bodies in place. The terminal berth remains held
through the turn; other trains cannot enter the same terminal. Consist changes
wait until the train is empty and stopped at a terminal.

Interstation reservations remain conservative: a whole section is reserved against
opposing traffic, including source/destination throats and receiving platform.
This implementation does not claim independent double-track throughput merely
because two tunnels are drawn. Cross-line same-level central movements share an
exclusive lock through the station and until the departing tail clears it.

## Underground levels

Map 1 defaults to -1, -2 and -3 for its three lines. Each line's level can be set
from -1 to -16 in the always-visible metro settings. All-level rendering draws deeper
lines first; a selected level includes only its tunnels, stations and trains.
The level is an operational property: different levels use independent crossing
locks. Combining occupied crossings onto the same floor is rejected atomically;
wait until trains clear before applying that combination. Changes preserve clock,
orders, passengers, train positions and active reservations, and survive reset.
There is no passenger transfer between metro lines or mixed-mode demand integration
in this change. Road/taxi sharing and the single-order taxi constraint are retained.

## Verification

Focused tests cover closed-route continuity, all six trains traversing real terminal
loops and delivering passengers, physical car poses, conservation, same-level
interlocking and concurrent different-level crossings. Final checks additionally
cover the complete backend suite, geometry, desktop startup and packaged map/UI
workflows. Validation results are recorded after the final run.

Final verification passed on 2026-11-15: complete Go tests and vet; all seven
interchange geometry checks; both desktop backend integration tests; packaged
Map 1 and Map 2 smoke runs. Map 1 completed two taxi orders and 98 metro trips;
Map 2 completed two taxi orders and 43 metro trips. Both applied external route
and signal algorithms without fallback. The enlarged road map's taxi observation
window was extended to allow physically longer journeys; its 600-second pickup
cancellation constraint remains unchanged. The final rendering regression verified
that 1.5x zoom increases both car and tunnel widths by exactly 1.5x. A detailed
screenshot confirmed six cars inside hollow tunnels with real passenger dots.

Map 1 topology correction: the former collector-to-cardinal outer cycle has been
removed. CNW, CNE, CSW and CSE now lie exactly on the 2 km ring at diagonal bearings.
Each has one radial connection to its neighborhood hub; the hubs retain two leaf
streets each. The only peripheral cycle is the requested circle. Intersection
square/halo markers are removed; selection, labels and actual signal indications
remain functional. World coordinates continue to use one shared X/Y metre scale.
