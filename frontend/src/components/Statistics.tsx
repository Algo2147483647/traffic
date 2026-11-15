import { useAppState } from "../state/store";
export function Statistics() {
  const { scene } = useAppState(),
    s = scene.snapshot,
    c = scene.city,
    t = s?.taxi,
    p = s?.control_performance;
  const time = s?.time || 0;
  const fmt = (v = 0) => v.toFixed(1);
  if (scene.mapView === "communications") {
    const r = s?.communications;
    return (
      <section
        className="glass stats-overlay"
        aria-label="Communications statistics"
      >
        <div className="stat-primary">
          <div>
            <strong>{r?.connected || 0}</strong>
            <span>connected users</span>
          </div>
          <strong>{time.toFixed(1)} s</strong>
        </div>
        <p>
          {r?.terminals.length || 0} terminals · {r?.outage || 0} outage ·{" "}
          {r?.unsupported || 0} outside model
        </p>
        <p className="radio-note">
          Received power · thermal-noise SNR · user-count heat
        </p>
      </section>
    );
  }
  return (
    <section
      className="glass stats-overlay"
      aria-label="Live traffic statistics"
    >
      <div className="stat-primary">
        <div>
          <strong id="people-count">{s?.people.length ?? "—"}</strong>
          <span id="population-label">population</span>
        </div>
        <strong id="sim-time" aria-label="Simulation time">
          {Math.floor(time / 60)
            .toString()
            .padStart(2, "0")}
          :{(time % 60).toFixed(1).padStart(4, "0")}
        </strong>
      </div>
      <div className="stat-row">
        {[
          ["driving", "Driving"],
          ["waiting", "Waiting"],
          ["walking", "Walking"],
        ].map(([key, label]) => (
          <span key={key}>
            <i className={"dot " + key} />
            {label} <b id={key + "-count"}>{s?.stats[key] ?? "—"}</b>
          </span>
        ))}
      </div>
      <div className="stat-meta">
        <span>
          <b id="average-speed">{fmt((s?.stats.average_speed || 0) * 3.6)}</b>{" "}
          km/h avg
        </span>
        <span>
          <b id="way-count">{c?.ways.length ?? "—"}</b> ways /{" "}
          <b id="intersection-count">
            {c?.intersections.filter((n) => n.traffic_control.mode !== "flow")
              .length ?? "—"}
          </b>{" "}
          signals /{" "}
          <b id="lane-count">
            {c?.edges.reduce((n, e) => n + e.weight, 0) ?? "—"}
          </b>{" "}
          lanes
        </span>
      </div>
      <div className="efficiency-row">
        <span>
          <b id="signal-queue">{p?.current_signal_queue ?? "—"}</b>signal queue
        </span>
        <span>
          <b id="throughput">{fmt(p?.throughput_per_minute)}</b>trips/min
        </span>
        <span>
          <b id="mean-delay">{fmt(p?.mean_signal_delay_seconds)}s</b>mean delay
        </span>
      </div>
      <div id="taxi-stats" hidden={!t}>
        {t &&
          [
            ["created", "Orders", t.created],
            ["picked", "Picked up", t.picked],
            ["completed", "Completed", t.completed],
            ["cancelled", "Cancelled", t.cancelled],
            ["wait", "Mean wait", fmt(t.mean_wait_s) + " s"],
            [
              "solver",
              "Solver",
              t.solving ? "Computing…" : fmt(t.solver_ms) + " ms",
            ],
          ].map(([id, label, v]) => (
            <div key={id}>
              <span>{label}</span>
              <b id={"taxi-" + id}>{v}</b>
            </div>
          ))}
      </div>
    </section>
  );
}
