> Current geometry: see [Compact interchange revision](compact-interchanges.md).
> Earlier dimensions below describe the previous layout.

# Interchange system — design and implementation specification

## Scope and design decision

The four-cell map uses four rotated three-arm trumpet interchanges at its outer junctions.
The central interchange becomes a full cloverleaf: four directional through
carriageways, four 270-degree left-turn loops, four direct right-turn ramps,
and four two-lane collector-distributor (C/D) carriageways. Mainline through
traffic bypasses the loop merge/diverge sequence. C/D roads still contain weaving;
this is represented by lane changes and storage checks, not assumed away.

The legacy Meridian map remains independent. Existing executable location remains
`desktop/dist/win-unpacked/Meridian Traffic.exe`.

## Standards and source register (checked 2026-11-14)

- China MOT: **JTG/T D21-2014, Guidelines for Design of Highway Grade-separated
  Intersections**. The official standards catalogue marks it current. It is the
  baseline reference for overall layout, ramps and terminal intersections.
  https://jtst.mot.gov.cn/hb/search/stdHBDetailed?id=90a88625565b079dc2dcc3fabce96279
- Official publication / full text:
  https://xxgk.mot.gov.cn/jigou/glj/202006/t20200623_3312204.html
- TxDOT Roadway Design Manual, section 15.3, interchange types: use as a typology
  and tradeoff reference, not as a replacement for Chinese design requirements.
  https://www.txdot.gov/manuals/des/rdw/chapter-15-grade-separations-and-interchanges-/15-3-types-of-interchanges.html
- TxDOT sections 15.7.3 and 15.7.7: horizontal alignment, gore areas, ramp terminals,
  speed-change facilities and transition continuity.
  https://www.txdot.gov/manuals/des/rdw/chapter-15-grade-separations-and-interchanges-/15-7-ramps---direct-connectors-/15-7-3-ramp-geometrics.html
  https://www.txdot.gov/manuals/des/rdw/chapter-15-grade-separations-and-interchanges-/15-7-ramps---direct-connectors-/15-7-7-ramp-terminal-design.html

No numeric engineering minimum is inferred from a search excerpt. Project lengths,
heights and speeds below are simulation-world parameters, not approved road-design
values. Construction design additionally requires jurisdiction, design vehicle,
traffic forecasts, terrain, sight distance, superelevation, drainage, structural
clearance, roadside protection, signing and engineering review.

## Common interchange catalogue

| Type | Suitable role | Essential geometry / operation | Main limitation | Software status |
|---|---|---|---|---|
| Diamond | Freeway to surface arterial | Four ramps, two surface terminals; turn storage and coordinated signals | Terminal queues can spill back | Design catalogue only |
| Folded diamond | Constrained three-arm access | Folded ramp arrangement, surface connector and terminals | Extra turns and terminal conflicts | Replaced by trumpets |
| Trumpet | Three-arm freeway connection | One loop, directional ramps; independent merges/diverges | Loop speed/capacity; difficult fourth-arm expansion | Four peripheral instances |
| Full cloverleaf | Four-arm connection | Four loops, four right ramps; two crossing levels; C/D preferred | Weaving and loop capacity | Central instance supported |
| Partial cloverleaf | Constrained site / uneven turns | Selected loops, other turns at surface terminals | Residual crossing conflicts | Design catalogue only |
| Directional T / stack | High turning volumes | Direct/semi-direct ramps, multiple structures/levels | Footprint, structures and cost | Design catalogue only |
| Diverging diamond | High arterial left-turn demand | Two crossover terminals with coordinated control | Specialized lane/control rules | Design catalogue only |

The catalogue is not a claim that every type has a runnable template.

## Internal architecture

1. **Design profile and catalogue**: versioned JSON with explicit world units,
   dimensions, lane counts, role speeds and supported templates.
2. **Geometry primitives**: sampled circular arcs and tangent cubic transition
   curves; maximum sample spacing bounds the piecewise-linear approximation.
3. **Template compiler**: rotation/translation instances produce normal graph
   nodes and ways. Explicit lane-to-lane connections are compiled at flow nodes.
4. **Validation**: endpoint coincidence, elevation continuity, allowed structures,
   per-lane bounds, complete approach coverage and simple merge/split topology.
   Tests verify all arm-to-arm movements and actual loop/bridge traversal.
5. **Simulation**: existing routing, following, lane changes and receiving-lane
   reservations apply to all components. Flow nodes have no periodic red phase.
6. **Presentation**: road roles, layer filters, bridge occlusion and interchange
   overview show the same sampled geometry used by the backend. Curves are not
   independently re-smoothed by the UI.

## Central cloverleaf geometry

Use the local eastbound direction as the base; rotate it by 90 degrees to obtain
all four directions. Right-hand traffic only for this preset.

- Mainline carriageway centers: 24 world units from the axes; 3 lanes × 8 units.
- C/D centers: 70 units from the axes; 2 lanes × 8 units.
- Loop radius: 140 units; each loop sweeps 270 degrees and has one lane.
- Right-turn radius: 1100 units, one lane, outside the loop envelope.
- Mainline split / merge stations: -1500 / +1500; external portals: ±1800.
- C/D sequence: right exit → loop merge → loop exit → right merge.
- Loop merge-to-exit station separation: 420 units. This is an explicit weaving
  segment, not a certified minimum. Mainline and C/D traffic have separate paths.
- East/west mainline and C/D roads are elevated; north/south are on the ground.
  All eight turn ramps change elevation continuously between their endpoints.
- Elevation profile is smooth at ramp endpoints. Height changes require a ramp
  structure, including same-level ramp roads used for C/D entry/exit transitions.

Lane continuity: through lane i maps to i; exits depart from the outer lane;
ramp merges enter the outer receiving lane. A turn never fans across every lane.
The C/D outer lane serves weaving while its inner lane provides a through option.
Mainline entry/exit transitions are separate roads; the current engine does not
simulate a continuously growing lane width or a calibrated acceleration lane.

## Peripheral trumpet geometry

The north instance uses an east/west mainline and a south branch. Rotate it for
south, west and east; cardinal route labels rotate with the geometry. Each has
six non-U-turn movements: two mainline through, two direct right turns, one
270-degree loop and one semi-direct left turn. Every merge/diverge has explicit
lane connections and continuous flow, without folded-diamond signal terminals.

Mainline portals are at +/-900, splits/merges at +/-650; the loop radius is 100.
The branch splits at 360 and ends at 520. Opposing carriageways are offset 24
from their axis. Mainline roads stay at ground level; two branch decks cross
above them at elevation 24. The loop and semi-direct ramp connect to those decks;
the two right ramps stay at ground level. Curves preserve endpoint positions and
tangents. These numbers are simulation parameters, not certified ramp minima.

## Kilometer coordinates

`world_units_per_km` is carried from configuration through the graph and city API.
It defaults to 1000 for existing maps; the four-cell profile explicitly sets 1000.
The four-cell domain is **6 x 6 km**. X grows right and Y grows down, measured
from the map's upper-left bound. Edge rulers follow zoom/pan; a bottom scale bar
shows a physical distance and the full domain size. There is no background grid.
This defines horizontal display distances; speed, vehicle dimensions and vertical
clearance still require separate calibration before interpreting engineering performance.

## Validation and release gates

- Deterministic preset regeneration; profile validation rejects invalid geometry.
- Four loops, four direct right ramps, four C/D groups and all 12 non-U-turn arm
  movements reachable without using the peripheral network.
- No shared at-grade node at the mainline crossing; mainline through routes do
  not require loop or signal-terminal roads.
- Lane connection indices valid; merge reservations prevent two vehicles claiming
  the same receiving lane concurrently.
- Loop samples and road rendering use one common path; bridge order and filters
  do not change simulation state.
- Both presets load; existing model/server/simulation tests pass.
- Live desktop smoke check covers map selection, layers, time progression and
  interchange details; update the same unpacked EXE directory, no installer.

## Known simulation boundaries

This release is a complete executable simulation feature, not a certified civil
engineering design. It does not calculate bridge clearance against deck thickness,
road crossfall/superelevation, sight distance, driver gap acceptance priority,
vehicle power on slopes, or calibrated acceleration. Existing receiving-lane
reservations serialize merges conservatively. Constant lane widths approximate
speed-change lanes. C/D weaving remains a capacity risk requiring long-duration
load tests before drawing throughput conclusions.
