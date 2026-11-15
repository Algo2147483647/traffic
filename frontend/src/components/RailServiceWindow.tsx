import { useEffect, useState } from "react";
import { useAppState, ui, emit } from "../state/store";
import { control } from "../api/client";
import { FloatingWindow } from "./FloatingWindow";
function Table({
  headers,
  rows,
}: {
  headers: string[];
  rows: Array<Array<string | number>>;
}) {
  return (
    <table>
      <thead>
        <tr>
          {headers.map((h) => (
            <th key={h}>{h}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => (
          <tr key={i}>
            {row.map((v, j) => (
              <td key={j}>{v}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
export function RailServiceWindow() {
  const { scene } = useAppState(),
    m = scene.snapshot?.metro;
  const [planner, setPlanner] = useState("cycle-time"),
    [headway, setHeadway] = useState("180"),
    [line, setLine] = useState(""),
    [feedback, setFeedback] = useState("");
  const plans = m?.services || [];
  useEffect(() => {
    if (!m) return;
    setPlanner(m.config.service?.algorithm || "cycle-time");
    setHeadway(
      String(m.config.service?.target_headway_s || m.config.headway_s),
    );
    setLine((old) =>
      plans.some((s) => String(s.plan.line) === old)
        ? old
        : String(plans[0]?.plan.line ?? ""),
    );
  }, [
    JSON.stringify([
      m?.config.service,
      m?.config.headway_s,
      plans.map((s) => s.plan.line),
    ]),
  ]);
  const service = plans.find((s) => String(s.plan.line) === line),
    geometry = m?.lines.find((l) => String(l.id) === line);
  const time = (v: number) => v.toFixed(1) + " s";
  return (
    <>
      <button
        id="rail-service-toggle"
        className="quiet"
        aria-label="Rail service planning"
        title="Rail service planning"
        onClick={() => {
          ui.rail = !ui.rail;
          emit();
        }}
      >
        ▤
      </button>
      <FloatingWindow
        id="rail-service-panel"
        title="Rail service planning"
        open={ui.rail}
        close={() => {
          ui.rail = false;
          emit();
        }}
        initial={{ x: 100, y: 80, width: 780, height: 680 }}
      >
        <label>
          Planner{" "}
          <select
            id="rail-service-algorithm"
            value={planner}
            onChange={(e) => setPlanner(e.target.value)}
          >
            {(m?.service_planners || ["cycle-time"]).map((p) => (
              <option key={p}>{p}</option>
            ))}
          </select>
        </label>
        <label>
          Target interval per direction (s){" "}
          <input
            id="rail-service-headway"
            type="number"
            step="any"
            value={headway}
            onInput={(e) => setHeadway(e.currentTarget.value)}
          />
        </label>
        <button
          id="rail-service-apply"
          onClick={async () => {
            if (!Number.isFinite(Number(headway)) || Number(headway) <= 0) {
              setFeedback("Enter a positive departure interval");
              return;
            }
            try {
              await control("set_metro", {
                metro: {
                  ...m.config,
                  service: {
                    algorithm: planner,
                    target_headway_s: Number(headway),
                  },
                },
              });
              setFeedback(
                "Plan applied · existing passenger journeys retained",
              );
            } catch (e) {
              setFeedback(String(e instanceof Error ? e.message : e));
            }
          }}
        >
          Plan and apply
        </button>
        <p id="rail-service-feedback" role="status">
          {feedback}
        </p>
        <p>
          Fleet size and feasible intervals are calculated from running time,
          dwell, turnback and track capacity. Occupied platforms or blocks delay
          departure. Existing passengers keep their journeys.
        </p>
        <div id="rail-service-summary">
          <Table
            headers={[
              "Line",
              "Planned fleet",
              "In service / depot",
              "Retiring",
              "Interval (s)",
              "Trips done",
            ]}
            rows={plans.map((s) => [
              s.plan.name,
              s.plan.fleet,
              `${s.active} / ${s.depot}`,
              s.retiring,
              `${s.plan.headway_s.toFixed(1)} (target ${s.plan.requested_headway_s.toFixed(1)})`,
              s.completed,
            ])}
          />
        </div>
        <label>
          Line{" "}
          <select
            id="rail-service-line"
            value={line}
            onChange={(e) => setLine(e.target.value)}
          >
            {plans.map((s) => (
              <option key={s.plan.line} value={s.plan.line}>
                {s.plan.name}
              </option>
            ))}
          </select>
        </label>
        <div id="rail-service-trips">
          <Table
            headers={[
              "Trip",
              "Direction",
              "Scheduled",
              "Departed",
              "Delay",
              "Train",
              "State",
            ]}
            rows={(service?.trips || [])
              .slice(-30)
              .map((t) => [
                `#${t.id}`,
                geometry?.circular
                  ? t.direction > 0
                    ? "Clockwise"
                    : "Counterclockwise"
                  : t.direction > 0
                    ? "Forward"
                    : "Reverse",
                time(t.planned_s),
                t.actual_s ? time(t.actual_s) : "—",
                time(
                  t.actual_s
                    ? Math.max(0, t.actual_s - t.planned_s)
                    : Math.max(0, m.time - t.planned_s),
                ),
                t.train ? `T${t.train}` : "Awaiting depot",
                t.state,
              ])}
          />
        </div>
      </FloatingWindow>
    </>
  );
}
