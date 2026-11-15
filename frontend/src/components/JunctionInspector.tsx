import { useEffect, useRef, useState } from "react";
import { useAppState } from "../state/store";
import { mapController } from "../rendering/controller";
function PhaseCanvas({ intersection, phase }) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    mapController.current?.drawJunctionCopy(ref.current, intersection, phase);
  }, [intersection, phase]);
  return (
    <canvas
      ref={ref}
      width={680}
      height={520}
      style={{ width: "100%", height: "auto" }}
      aria-label={`${intersection.id}: ${phase.id}`}
    />
  );
}
export function SignalInspector() {
  const { scene } = useAppState(),
    id = scene.selected?.id,
    n = scene.city?.intersections.find((n) => n.id === id),
    s = scene.snapshot?.signals.find((s) => s.intersection_id === id);
  if (!n || !s) return null;
  return (
    <div id="signal-inspector">
      <div className="panel-heading">
        <h2 id="signal-name">
          {n.id} / {n.name}
        </h2>
        <span id="signal-countdown">{s.remaining_seconds.toFixed(1)}s</span>
      </div>
      <div className="controller-card">
        {[
          ["controller", "Controller", s.controller || "fixed-time"],
          ["decision", "Decision", s.decision || "scheduled"],
          [
            "green-target",
            "Green target",
            `${s.base_green_seconds.toFixed(1)} → ${s.target_green_seconds.toFixed(1)}s`,
          ],
          [
            "pressure",
            "Pressure",
            `${s.queue_pressure.toFixed(1)} / ${s.competing_pressure.toFixed(1)}`,
          ],
        ].map(([key, label, value]) => (
          <div key={key}>
            <span>{label}</span>
            <strong id={"signal-" + key}>{value}</strong>
          </div>
        ))}
      </div>
      <div id="signal-plan" className="signal-plan">
        {n.traffic_control.phases.map((p, i) => (
          <div
            key={p.id}
            className={`phase-row ${i === s.phase_index && s.stage === "green" ? "active" : ""} ${p.id === s.next_phase_id ? "next" : ""}`}
            title={p.id === s.next_phase_id ? "Next phase" : ""}
          >
            <i />
            <span>Phase {i + 1}</span>
            <strong>
              {p.duration_seconds}s ·{" "}
              {p.min_green_seconds || p.duration_seconds}–
              {p.max_green_seconds || p.duration_seconds}s
            </strong>
          </div>
        ))}
      </div>
      <div id="signal-movements" className="movement-list">
        {n.traffic_control.phases.map((p, i) => (
          <div
            key={p.id}
            data-phase-index={i}
            className={
              "flow-card " +
              (i === s.phase_index && s.stage === "green" ? "flow-active" : "")
            }
          >
            <div>Phase {i + 1}</div>
            <PhaseCanvas intersection={n} phase={p} />
          </div>
        ))}
      </div>
    </div>
  );
}
export function InterchangeInspector() {
  const { scene } = useAppState(),
    id = scene.selected?.id,
    g = scene.city?.interchanges?.find((g) => g.id === id);
  const [index, setIndex] = useState(0),
    ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => setIndex(0), [id]);
  useEffect(() => {
    if (g) mapController.current?.drawInterchange(ref.current, g.id, index);
  }, [g, index]);
  if (!g) return null;
  const route = g.routes[index] || g.routes[0],
    ways = scene.city.ways.filter((w) => route.way_ids.includes(w.id));
  return (
    <div id="interchange-inspector">
      <div className="panel-heading">
        <h2 id="interchange-name">{g.name}</h2>
      </div>
      <p id="interchange-summary">
        {g.type === "trumpet"
          ? "1 loop · 1 semi-direct ramp · 2 right-turn ramps"
          : "4 loops · 4 right-turn ramps · 4 C/D carriageways"}
      </p>
      <label className="select-control">
        Movement
        <select
          id="interchange-movement"
          aria-label="Interchange movement"
          value={index}
          onChange={(e) => setIndex(Number(e.target.value))}
        >
          {g.routes.map((r, i) => (
            <option key={i} value={i}>
              {r.origin} → {r.destination} · {r.turn}
            </option>
          ))}
        </select>
      </label>
      <canvas
        id="interchange-canvas"
        ref={ref}
        width={760}
        height={760}
        aria-label="Interchange route overview"
      />
      <p id="interchange-route-detail">
        {ways.some((w) => w.role === "loop_ramp")
          ? "Loop ramp"
          : ways.some((w) => w.role === "semi_direct_ramp")
            ? "Semi-direct ramp"
            : route.turn === "right"
              ? "Direct right-turn ramp"
              : "Continuous mainline"}{" "}
        · {route.way_ids.length} road segments
      </p>
    </div>
  );
}
