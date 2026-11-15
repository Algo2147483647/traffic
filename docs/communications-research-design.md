# Base stations and research communications simulation — proposed design

Status: design only. No radio fidelity or communications capability is claimed by placing a base-station icon on the map.

## Domain boundaries

- Infrastructure: `RadioSite` (id, xyz in metres, mast/roof height, power/backhaul), `Sector` (azimuth/tilt/antenna pattern, carrier/bandwidth, transmit power), terrain/building/tunnel material geometry. Keep sites separate from road nodes and metro junctions.
- Communications runtime: UE/session lifecycle, association and handover, channel observations, interference, resource scheduling, packet queues, retransmission and backhaul.
- Mobility bridge: read immutable person/vehicle/train poses and velocity; stable UE IDs follow people through boarding/transfer. A passenger in a train is not a second independently moving body. Antenna position and vehicle penetration loss are separate from vehicle centre position.
- Research algorithms: cell selection, handover, radio scheduler, power control, placement and traffic models. Workers propose actions; runtime validates resource/power budgets and ownership.

## Data materials

Add optional `example/<map>/communications.json` plus a versioned schema and radio/scenario profiles. Use SI internally (m, s, Hz, W), explicit conversion at dBm/MHz UI boundaries, and a declared local coordinate origin. Distinguish absolute antenna elevation from mast height above local ground. Include site/sector IDs, carrier frequency, bandwidth, antenna array/pattern, azimuth/downtilt, noise figure, transmit power, backhaul and initial UE/application definitions. Model versions, seeds and material assumptions belong in the experiment manifest, not source-code constants.

## Time and external engine integration

Road physics stays at 50 ms. Radio scheduling/channel/packet events need a finer, independent event time; do not pretend one 50 ms road tick is an NR slot. Use integer timestamps, deterministic event ordering and interpolation of the road trajectory between coupling boundaries. At each synchronization boundary, publish a mobility batch and consume communication results. For closed-loop decisions, both sides stop at a barrier; late results cannot modify past world state.

Use a persistent external-engine adapter rather than starting a process for each UE or packet. A minimal contract includes run/revision/time horizon, poses, network actions, metric windows and completion acknowledgement. Separate wall-clock timeout from simulated time, and explicitly mark failed or approximated results. Do not silently mix native fallback metrics with reference-engine experiment results.

Recommended reference integration: ns-3 with CTTC 5G-LENA. Its model documentation describes 3GPP channel propagation and NR features; exact release, model support and calibration assumptions must be pinned per experiment.

## Fidelity stages and acceptance criteria

1. **Mobility export and baseline link budget**: fixed sites, antenna poses, association, received power and coverage snapshots. Validate against analytic fixtures; label this a simplified model. A coverage circle is a UI approximation, not SINR or usable capacity.
2. **Reference channel model**: a pinned 3GPP TR 38.901 scenario with LOS/NLOS, path loss, shadowing/fading and antenna effects. Validate supported frequency/distance/height ranges. Tunnels need a separately validated propagation model; ordinary outdoor urban formulas cannot be assumed correct underground.
3. **Load and protocol behaviour**: shared spectrum interference, per-UE queues, resource scheduling, HARQ/retransmission, handover and backhaul. Report throughput, delay percentiles, loss, outage, handover failure, fairness and energy with definitions and observation windows.
4. **Research experiments**: multiple seeds, warmup, confidence intervals, sensitivity analysis, calibrated reference scenarios, recorded algorithm decisions, exact replay and engine/version hashes.

## Presentation

Use the same metre-based height slices for site antenna positions and moving UEs. Optional sector cones, selected-UE serving/neighbour links, channel/interference heatmaps and event timeline belong to a communications overlay. The right inspector should show the active model/version, sampling time and metric units. Rendering consumes aggregated samples; it does not compute radio physics.

## Primary references

- [3GPP TR 38.901 specification](https://portal.3gpp.org/desktopmodules/Specifications/SpecificationDetails.aspx?specificationId=3173)
- [ns-3 propagation models](https://www2.nsnam.org/docs/models/html/propagation.html)
- [ns-3 spectrum/channel models](https://www2.nsnam.org/docs/models/html/spectrum.html)
- [CTTC 5G-LENA](https://cttc-lena.gitlab.io/5g-lena-website/)
- [5G-LENA model helpers](https://cttc-lena.gitlab.io/nr/manual/helpers.html)

## Approved package ownership (2026-11-15)

- `internal/infrastructure/communications`: sites, base station equipment, sectors, cells, antenna patterns, carrier configuration and backhaul topology.
- `internal/infrastructure/environment`: shared physical geometry and material properties.
- `internal/communications_scheduling`: UE state, association/handover, channels, interference, resource allocation, transmission and engine adapters.
- `internal/operations/communications`: subscriptions, application sessions and generated traffic.
- `internal/algorithms/experiments`: injected communications or coupled-world experiment sessions.
- `internal/simulation`: coupling barriers and stable person/vehicle/train-to-UE binding.

There is deliberately no communications runtime, empty package or placeholder interface in this refactor. The UI must not imply that a radio model exists until an actual model and its validation are delivered.
