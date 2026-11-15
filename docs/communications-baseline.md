# Map 1 communications baseline

## Delivered scope

Map 1 replaces the former Map 3; Map 2 remains unchanged. The old Map 1 is no longer a preset or packaged example. Historical signal/circuit regressions use isolated test fixtures.

The first communications stage is operational: JSON sites and sectors, moving terminal poses, deterministic strongest-cell association, received power, thermal noise/SNR, per-cell counts, height-aware map overlays and mobility/link export. The model is explicitly a simplified outdoor baseline. No NR scheduler, packets, interference/SINR, protocol handover, calibrated 3GPP channel or underground propagation is claimed.

## 256 m honeycomb

`example/Map1/communications-generation.json` defines a pointy-top hexagonal deployment with circumradius 256 m (centre to vertex). Horizontal nearest-neighbour distance is `sqrt(3) * 256 = 443.405 m`; row pitch is 384 m with alternating half-spacing. The local origin is (128,128) m, independent of the map origin at the central cloverleaf.

The current 10.4 km by 10.4 km map bounds contain 700 site centres including a boundary guard band, with three directional sectors each (2,100 cells). Border sites ensure the map edge is not left uncovered. `scripts/generate_communications.py` writes the actual facilities to `communications.json`; no site coordinates are compiled into Go or JavaScript.

The 256 m horizontal radius is an enforced service-range limit. Hexagons depict the nominal tessellation, while overlapping circular range limits and received-power ranking determine actual association. The sensitivity threshold can still cause outage within the nominal range. This is a planning assumption, not a claim about real-world guaranteed reception.

## Model and units

Internally use metres, seconds, Hz and W; UI converts W to dBm and Hz to GHz/MHz. JSON differentiates ground/roof elevation from mast height. Antenna elevation is their sum. Terminal antenna height is a separate profile parameter.

The baseline uses Friis loss at the configured reference distance, followed by `10 n log10(d/d0)`, with three-dimensional distance. The configured parabolic azimuth/elevation pattern and vehicle loss modify the link budget. Thermal noise is `k T B F`. This follows the basic [ns-3 propagation model definitions](https://www2.nsnam.org/docs/models/html/propagation.html), but does not embed ns-3 or implement a standards-compliant NR channel. Profile values are scenario assumptions recorded in JSON.

Distances below the reference distance are unsupported. Underground terminals retain valid mobility records but have `unsupported-underground` status and no invented outdoor radio link. An outdoor terminal outside all service radii is in outage. Neighbours are ranked by received power, with cell ID as a deterministic tie break. No random fading is included.

A spatial bucket index restricts each terminal query to nearby sites. The observer is pure at a committed simulation snapshot and does not advance the world clock, mutate traffic or count a read as a handover. Observation timestamps are integer microseconds at the existing 50 ms mobility boundaries; no sub-tick radio fidelity is implied.

## Responsibilities

- `internal/infrastructure/communications`: versioned configuration, validation, site and sector geometry.
- `internal/communications_scheduling`: link budget, antenna pattern, spatial search, association and observation accounting.
- `internal/simulation/communications.go`: world composition and stable terminal binding.
- `internal/metro_scheduling/transfers.go`: passenger observations follow the actual assigned train car; transfers retain identity.
- `frontend/src/rendering/radio.ts`: radio overlays only.
- `frontend/src/components/RadioSettings.tsx` and `RadioInspector.tsx`: visibility, site/cell/terminal metrics and export.

Terminal IDs include run and topology identity. Road people, taxi passengers, train modems and metro passengers have distinct namespaces. Taxi passengers follow their assigned vehicle after boarding. Metro passengers follow their actual car while riding and interpolate station elevation during transfer. Completing a passenger task removes that terminal from active observations; resetting the world changes the ID namespace.

`GET /api/communications` exports one coherent `{layout, observation}` snapshot with model/version, stamp, time, terminal mobility and links. Map 2 returns 404 because it has no radio layout. Settings offer export and optional nominal hexagons; site/UE visibility follows physical metre height slices. A selected terminal shows its serving and neighbour links. Terminal counts are not substituted for transport passenger counts.

## Checks

Analytic Friis/distance/antenna/penetration/noise fixtures; deterministic selection and terminal accounting; nominal-radius enforcement; honeycomb spacing and map-edge coverage; actual metro car binding; reset/map isolation; shared passenger accounting; packaged React controls and inspector.

## Next stages

Keep the remaining plan explicit: a pinned, validated reference channel model and terrain/tunnel materials; persistent reference-engine coupling at time barriers; sessions/queues/resource scheduling/handover; then multi-seed calibrated experiments. Do not introduce empty environment, operations or adapter packages before their execution is implemented.

## Validation record — 2026-11-15

Go regression packages passed, including communications analytic fixtures, all-map road/taxi/metro regressions, real transfer-to-car binding and world accounting. The infrastructure/road package was rerun successfully after replacing its obsolete three-preset/old-ring expectations. `go vet ./...` passed. All eight desktop tests passed, including the coherent communications export and Map 2 radio absence.

The packaged React scenario reported two presets, 700 radio sites, 2,100 cells, working settings/interchange/signal inspectors, three transferring passengers, a four-step itinerary, conserved demand and no captured page errors. Radio/honeycomb and passenger-route screenshots were inspected. The packaged resources contain only Map1 and Map2; bundled backend SHA-256 matches the current runtime build:

`46E5D86782F9B9F6FA4317429D9159620D76D4CB072069054283502D89194699`

The unchanged Windows launch path is `desktop/dist/win-unpacked/Meridian Traffic.exe`. macOS runtime verification was not performed on this Windows host.

## Traffic / communications presentation

The right-edge icon group selects All, Traffic or Communications without changing simulation state. Communications shows every visible connected UE's serving-site link and a smooth user-count heatmap. Hexagonal border/sector lines and their old checkbox are removed; the 256 m physical deployment and association radius remain unchanged.

Heat is associated users per site, scaled to the current maximum, not radio-resource utilization. The inspector plots serving-link received power and thermal-noise SNR empirical CDFs, a site-load histogram including idle sites, and connected/outage counts over simulation time. Empty populations have an explicit empty state. Network statistics include all heights; map objects and links honor height slices. History uses at most 180 simulation-time samples, clears on run/topology changes and does not accumulate duplicate paused samples. Unsupported metrics (RSRP, SINR, throughput, delay, resource utilization) are not fabricated.
