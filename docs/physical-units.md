# Physical units and fixed simulation steps

The road engine uses meters for positions, road/lane widths, vehicle length,
spacing and elevation; seconds for time; and meters/second for speed_limit,
drive_speed and walk_speed. world_units_per_km must be 1000 (omission defaults to
1000). Other coordinate scales fail validation instead of silently mixing units.
Existing custom maps must convert their lengths and speeds before loading.

Both shipped maps were migrated: ordinary road lanes are 3.5 m; interchange lanes
are 3.75 m; the passenger-car design length is 6 m and rendering width is 1.8 m.
These are a design vehicle envelope, not the dimensions of every real car.
The lane-width selection follows the urban road width tables; the design passenger
vehicle follows the 6 x 1.8 m design-vehicle reference:
https://jst.zj.gov.cn/attach/-1/1904041304003289034.pdf
https://ghzrzyw.beijing.gov.cn/biaozhunguanli/bz/szjgdjt/202406/P020251126538149168541.pdf

Former road speed numbers were interpreted as intended km/h and divided by 3.6.
The vehicle cruise cap is now 95 / 3.6 m/s; walking is 1.4 m/s. UI average speed
converts m/s to km/h. The geometry compilers output m/s as well. Headway and stop
storage calculations use the same 6 m vehicle length as rendering.

Traffic and Taxi share a 0.05 s physical step. A time accumulator retains smaller
fractions between calls; reset clears it. Playback rate changes the number of steps,
not their size. Waiting for an external decision freezes simulation time and does
not accumulate a wall-time catch-up backlog. Snapshots may update less frequently
than physics. Durations not divisible by 0.05 s resolve on the physical step grid.

Tests compare a single 1 s request, 100 requests of 0.01 s, and 0.5 s at 2x in both
modes, asserting identical vehicle state and 20 ticks. Existing physical ramp and
taxi tests remain active. The sustained-service benchmark now runs 2160 s with
8/3.6 orders/min and a 2160 s wait budget to retain the previous offered demand per
journey after the speed conversion; this is not the default scenario demand.

This change does not shrink interchange centerlines or redesign bridge elevations.
The oversized interchange footprint, engineered curvature/grade constraints and
full strategy/replay/metro work remain in the implementation audit.
