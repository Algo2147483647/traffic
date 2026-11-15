import { useAppState, scene, emit } from "../state/store";
import { fitPassenger } from "../rendering/controller";
export function MetroInspector() {
  useAppState();
  const m = scene.snapshot?.metro,
    id = scene.selected?.id,
    site = m?.interchanges?.find((x) => x.id === id),
    detail = m?.station_details?.find((x) => x.id === id);
  if (!site || !detail) return null;
  const name = (id: number) =>
      m.stations.find((s) => s.id === id)?.name || `Station ${id + 1}`,
    lineName = (id: number) =>
      m.lines.find((l) => l.id === id)?.name || `Line ${id + 1}`;
  const passengers = (m.journeys || []).filter((j) =>
      detail.passenger_ids.includes(j.id),
    ),
    j = scene.metroPassenger;
  if (j && !passengers.some((p) => p.id === j.id)) passengers.push(j);
  return (
    <div id="metro-station-inspector">
      <h2 id="metro-station-name">{site.name}</h2>
      <p id="metro-station-count">
        {detail.platforms.reduce(
          (n, p) => n + p.directions.reduce((a, d) => a + d.waiting, 0),
          0,
        )}{" "}
        waiting · {detail.transferring} transferring
      </p>
      <div id="metro-platforms">
        {detail.platforms.map((p) => (
          <section key={p.station}>
            <h3>
              {lineName(p.line)} · {p.elevation_m} m
            </h3>
            {p.directions.map((d) => (
              <p key={d.direction}>
                <strong>
                  {m.lines.find((l) => l.id === p.line)?.circular
                    ? d.direction === 1
                      ? "Clockwise"
                      : "Counterclockwise"
                    : `Toward ${name(d.toward)}`}
                </strong>
                <span>
                  {d.waiting} waiting ·{" "}
                  {d.eta_seconds === null
                    ? d.status
                    : d.eta_seconds === 0
                      ? `T${d.train} at platform`
                      : `T${d.train} in ~${Math.ceil(d.eta_seconds)} s (${d.status})`}
                </span>
              </p>
            ))}
          </section>
        ))}
      </div>
      <h3>Passengers at this station</h3>
      <label>
        Passenger{" "}
        <select
          id="metro-passenger-select"
          disabled={!passengers.length}
          value={j?.id ?? ""}
          onChange={(e) => {
            scene.metroPassenger = passengers.find(
              (p) => String(p.id) === e.target.value,
            );
            emit();
            fitPassenger();
          }}
        >
          {passengers.map((p) => (
            <option key={p.id} value={p.id}>
              P{p.id}
              {p.journey_id ? ` · Trip ${p.journey_id}` : ""} · {p.state}
            </option>
          ))}
        </select>
      </label>
      <p id="metro-passenger-state">
        {j
          ? `${j.state}${j.state === "transferring" ? ` · ${Math.ceil(Math.max(0, j.ready_at - m.time))} s walk remaining` : ""}`
          : "No passengers waiting or transferring here"}
      </p>
      <div id="metro-route-legend" hidden={!j}>
        <span className="route-done">Completed</span>
        <span className="route-current">Current</span>
        <span className="route-next">Upcoming · dashed</span>
        <button id="metro-route-fit" className="quiet" onClick={fitPassenger}>
          Fit route
        </button>
      </div>
      <ol id="metro-passenger-path">
        {j?.plan.map((step, i) => (
          <li
            key={i}
            className={
              i === j.step ? "current-step" : i < j.step ? "completed-step" : ""
            }
          >
            {i === j.step ? "Current · " : i < j.step ? "Done · " : ""}
            {step.line < 0
              ? `Transfer at ${name(step.from)}: ${lineName(m.lines.find((l) => l.stations.includes(step.from))?.id)} → ${lineName(m.lines.find((l) => l.stations.includes(step.to))?.id)} (${step.estimated_seconds} s walk)`
              : `${lineName(step.line)}: ${name(step.from)} → ${name(step.to)}`}
          </li>
        ))}
      </ol>
      <p className="metro-estimate-note">
        Arrival estimates use simulation time and scheduled stops. Traffic
        control and boarding may cause delays.
      </p>
    </div>
  );
}
