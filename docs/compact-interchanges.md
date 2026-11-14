# Compact Map 2 interchanges

The map retains its 6 x 6 km meter coordinates and five interchange centers.
Actual road geometry changed; ruler scale and camera magnification did not.

| Centerline envelope | Before | After |
|---|---|---|
| Central cloverleaf including approaches | 3600 x 3600 m | 1740 x 1740 m |
| Central turning ramps | 2340 x 2340 m | 744 x 744 m |
| North/south trumpets including approaches | 1800 x 855 m | 1240 x 684.39 m |
| North/south trumpet turning ramps | 1300 x 695 m | 800 x 564.39 m |

East/west envelopes are rotated. These centerline boxes exclude pavement width,
shoulders, earthworks and right of way; they are not land acquisition areas.

Cloverleaf: 3.75 m lanes, 8 m mainline offset, 22 m C/D offset, 55 m loops,
350 m outside right ramps, 620 m split station, 480 m C/D station and 870 m
portals. Mainline/C-D/loop/right speeds are 80/50/30/40 km/h. The elevated
mainline section is 1240 m (previously 3000 m); this retains an elevated C/D
arrangement rather than claiming an isolated short bridge. Lane counts now use
the profile. All decks are 6.5 m above ground instead of 24 m.

Trumpets: 60 m loops, 8 m carriageway offset, 400 m split station, 620 m mainline
portals and 430 m branch portals. Profile-driven bridge endpoints and curve
controls replace the earlier fixed coordinates. The semi-direct curve stays
outside the loop envelope; its minimum sampled radius is about 33 m, and its
speed is restricted to 25 km/h. Loop/right/through speeds are 30/40/80 km/h.
Branch approaches use 250 m runs for the 6.5 m elevation change.

The executor's smoothstep elevation has peak grade 1.5 * rise/run. All graded
roads pass a 4% peak-grade check (mainline and branch approaches: 3.9%; central
loops: about 3.76%). Sampled curvature passes a project lateral-acceleration
limit of 2 m/s^2 at configured speed. Sampling spacing is 2 m.

Validation covers endpoints/tangency, outer-ramp/loop envelope separation,
all nonincident trumpet centerline crossings on distinct fixed-height levels,
grade/curvature limits, full directed reachability, hierarchical lane paths,
all 36 physically driven taxi movements, and real pickup/delivery completion.

Reference for speed-dependent geometry and preferring grades at or below 4%:
[TxDOT ramp geometrics](https://www.txdot.gov/manuals/des/rdw/chapter-15-grade-separations-and-interchanges-/15-7-ramps---direct-connectors-/15-7-3-ramp-geometrics.html).
This is a physically scaled simulation layout. Structural depth/clearance,
superelevation, sight distance, drainage and construction design are not certified
by these simulation checks.
