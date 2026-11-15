import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { useAppState, scene, ui, emit } from "../state/store";
import { defaults } from "../state/defaults";
import {
  settingsAction,
  immediateFields,
  type Draft,
} from "../state/settingsActions";
const Context = createContext<{
  scene: typeof scene;
  ui: typeof ui;
  draft: Draft;
  feedback: Record<string, string>;
  change: (id: string, value: string | boolean, commit?: boolean) => void;
  act: (id: string) => void;
}>(null);
export const useSettings = () => useContext(Context);
export function SettingsProvider({ children }: { children: ReactNode }) {
  const state = useAppState();
  const [draft, setDraft] = useState<Draft>({
    ...defaults,
    "show-coordinates": scene.showCoordinates,
  });
  const [feedback, setFeedback] = useState<Record<string, string>>({});
  const s = scene.snapshot;
  const merge = (data: Draft) => setDraft((d) => ({ ...d, ...data }));
  useEffect(() => {
    if (!scene.city) return;
    const c = scene.city.simulation;
    merge({
      "population-control": String(c.people),
      "spawn-rate-control": String(c.spawn_rate_per_second * 60),
      "max-population-control": String(c.max_people),
      "safe-gap-control": String(c.safe_gap),
    });
  }, [scene.city]);
  useEffect(() => {
    if (!s) return;
    merge({
      "simulation-mode": s.mode,
      "walk-speed-control": String(s.walk_speed_mps),
      "time-scale": String(s.time_scale),
    });
  }, [s?.mode, s?.walk_speed_mps, s?.time_scale]);
  useEffect(() => {
    if (!s?.taxi) return;
    const c = s.taxi.config;
    merge({
      "taxi-fleet": String(c.fleet),
      "taxi-fleet-range": String(c.fleet),
      "taxi-demand": String(c.orders_per_minute),
      "taxi-demand-range": String(c.orders_per_minute),
      "taxi-seed": String(c.seed),
      "taxi-interval": String(c.dispatch_interval_s),
      "taxi-algorithm": c.algorithm,
      "taxi-parameters": JSON.stringify(c.parameters || {}),
    });
  }, [JSON.stringify(s?.taxi?.config)]);
  useEffect(() => {
    if (!s?.demand) return;
    const c = s.demand.config;
    merge({
      "shared-rate": String(c.trips_per_minute),
      "shared-share": String(c.taxi_share * 100),
      "shared-metro": String((c.metro_share || 0) * 100),
    });
  }, [JSON.stringify(s?.demand?.config)]);
  useEffect(() => {
    if (!s?.metro) return;
    const c = s.metro.config;
    merge({
      "metro-enabled": c.enabled,
      "metro-headway": String(c.headway_s),
      "metro-demand": String(c.passengers_per_minute),
      "metro-cars": String(c.cars),
      "metro-speed": String(c.max_speed_kmh),
      "metro-capacity": String(c.capacity),
      "metro-dwell": String(c.dwell_s),
      ...Object.fromEntries(
        s.metro.lines.map((l) => [
          "metro-height-" + l.id,
          String(l.elevation_m),
        ]),
      ),
    });
  }, [
    JSON.stringify(s?.metro?.config),
    JSON.stringify(s?.metro?.lines.map((l) => l.elevation_m)),
  ]);
  useEffect(() => {
    merge({ "map-preset": ui.presetID });
  }, [ui.presetID]);
  const run = (id: string, d: Draft) => {
    settingsAction(id, d)
      .then(() =>
        setFeedback((f) => ({
          ...f,
          "settings-feedback": "",
          "height-feedback": "",
          "taxi-load-feedback":
            id === "taxi-load-apply"
              ? "Applied · active trips continue"
              : f["taxi-load-feedback"],
        })),
      )
      .catch((e) =>
        setFeedback((f) => ({
          ...f,
          "settings-feedback": String(e.message),
          "height-feedback":
            id.startsWith("height") || id === "layer-filter"
              ? String(e.message)
              : "",
        })),
      );
  };
  const change = (id: string, value: string | boolean, commit = true) => {
    const next = { ...draft, [id]: value };
    if (id.startsWith("taxi-fleet") || id.startsWith("taxi-demand")) {
      const base = id.replace("-range", "");
      next[base] = value;
      next[base + "-range"] = value;
    }
    setDraft(next);
    if (commit && immediateFields.has(id)) run(id, next);
  };
  return (
    <Context.Provider
      value={{ ...state, draft, feedback, change, act: (id) => run(id, draft) }}
    >
      {children}
    </Context.Provider>
  );
}
export function Field({
  as = "input",
  id,
  children,
  ...props
}: {
  as?: string;
  id: string;
  children?: ReactNode;
  [key: string]: unknown;
}) {
  const { draft, change, ui, scene } = useSettings();
  let content = children;
  let disabled = !!props.disabled;
  if (id === "map-preset") {
    content = ui.presets.map((p, i) => (
      <option key={p.id} value={p.id}>
        Preset {i + 1} · {p.name}
      </option>
    ));
    disabled = ui.presets.length < 2;
  }
  if (id === "taxi-algorithm" && ui.dispatch.length)
    content = ui.dispatch.map((p) => (
      <option key={p.id} value={p.id}>
        {p.name}
      </option>
    ));
  if (
    ["route", "signal", "motion", "supply", "demand", "metro"].some(
      (k) => id === k + "-algorithm",
    )
  )
    content = (
      <>
        <option value="native">Native</option>
        {ui.workers
          .filter((w) => id === w.capability + "-algorithm")
          .map((w) => (
            <option key={w.id} value={w.id}>
              {w.name || w.id}
            </option>
          ))}
      </>
    );
  if (id === "height-min" || id === "height-max")
    disabled = draft["layer-filter"] === "all";
  if (id === "metro-demand") disabled = !!scene.snapshot?.demand;
  const p = {
    ...props,
    id,
    disabled,
    hidden: props.hidden !== undefined ? true : undefined,
  };
  if (as === "select")
    return (
      <select
        {...p}
        value={String(draft[id] ?? "")}
        onChange={(e) => change(id, e.currentTarget.value)}
      >
        {content}
      </select>
    );
  if (as === "textarea")
    return (
      <textarea
        {...p}
        value={String(draft[id] ?? "")}
        onInput={(e) => change(id, e.currentTarget.value)}
      />
    );
  if (props.type === "checkbox")
    return (
      <input
        {...p}
        type="checkbox"
        checked={!!draft[id]}
        onChange={(e) => change(id, e.currentTarget.checked)}
      />
    );
  return (
    <input
      {...p}
      value={String(draft[id] ?? "")}
      onInput={(e) => change(id, e.currentTarget.value, false)}
      onBlur={(e) => {
        if (immediateFields.has(id)) change(id, e.currentTarget.value);
      }}
      onKeyDown={(e) => {
        if (e.key === "Enter" && immediateFields.has(id))
          change(id, e.currentTarget.value);
      }}
    />
  );
}
export function Action({
  id,
  children,
  ...props
}: {
  id: string;
  children?: ReactNode;
  [key: string]: unknown;
}) {
  const { act, scene } = useSettings();
  return (
    <button {...props} id={id} onClick={() => act(id)}>
      {id === "pause-button"
        ? scene.snapshot?.paused
          ? "Resume"
          : "Pause"
        : id === "fullscreen-button"
          ? document.fullscreenElement
            ? "Exit full screen"
            : "Full screen"
          : children}
    </button>
  );
}
export function Connection() {
  const { ui } = useSettings();
  return (
    <div
      className={"connection " + ui.connection}
      role="status"
      aria-live="polite"
    >
      <span className="connection-dot" />
      <span id="connection-label">{ui.connectionLabel}</span>
    </div>
  );
}
export function Value({
  id,
  fallback = "",
}: {
  id: string;
  fallback?: string;
}) {
  const { draft, scene, ui, feedback } = useSettings();
  const s = scene.snapshot,
    t = s?.taxi,
    m = s?.metro,
    d = s?.demand;
  const a = s?.algorithms;
  const values: Record<string, ReactNode> = {
    "view-scale": Math.round(scene.camera.zoom * 100) + "%",
    "compute-rate": (s?.paused ? 0 : ui.computeHz || 20).toFixed(1) + " Hz",
    "walk-speed-value": Number(draft["walk-speed-control"]).toFixed(1) + " m/s",
    "safe-gap-value": draft["safe-gap-control"] + " m",
    "population-value": draft["population-control"],
    "spawn-rate-value": draft["spawn-rate-control"] + "/min",
    "max-population-value": draft["max-population-control"],
    "load-current": s?.load?.population ?? 0,
    "shared-share-value": draft["shared-share"] + "%",
    "taxi-error": t?.last_error || "",
    "settings-feedback": ui.error || feedback[id] || "",
    "height-feedback": feedback[id] || "",
    "taxi-load-feedback": feedback[id] || "",
    "taxi-fleet-status": t
      ? `${t.vehicles.length} active · ${t.vehicles.filter((v) => v.retiring).length} finishing`
      : "",
    "road-algorithm-status": a
      ? a.last_error
        ? `Native fallback: ${a.last_error}`
        : `${a.pending ? "Computing · " : ""}${a.applied} applied · ${a.fallbacks} fallbacks`
      : "",
    "shared-summary": d
      ? `${d.conserved ? "Balanced" : "Accounting mismatch"} · ${d.generated} generated · ${d.car_trips} car / ${d.taxi_trips} taxi / ${d.metro_trips || 0} metro · ${d.queued} queued · ${d.active} active · ${d.completed} completed · ${d.cancelled} cancelled\n` +
        Object.entries(d.modes || {})
          .map(
            ([mode, n]) =>
              `${mode}: ${n.queued} queued / ${n.active} active / ${n.completed} completed / ${n.cancelled} cancelled`,
          )
          .join("\n")
      : "",
    "metro-summary": m
      ? `${m.generated} generated · ${Math.max(0, m.waiting - (m.transferring || 0))} platform waiting · ${m.onboard} onboard · ${m.completed} completed · ${m.transferring || 0} transferring · ${m.transfers_completed || 0} transfers · ${m.departures} departures · ${m.blocks.filter((b) => b.occupied_by).length} occupied blocks · ${m.routes.filter((r) => r.owner).length} locked routes`
      : "",
  };
  return <>{values[id] ?? fallback}</>;
}
export function SettingsSlot({ id }: { id: string }) {
  const { scene } = useSettings();
  const m = scene.snapshot?.metro;
  if (id === "metro-levels")
    return (
      <div id={id}>
        {m?.lines.map((l) => (
          <label className="select-control" key={l.id}>
            {l.name} elevation (m)
            <Field
              id={"metro-height-" + l.id}
              data-line={l.id}
              type="number"
              step="any"
            />
          </label>
        ))}
      </div>
    );
  return (
    <div id={id}>
      {m?.trains.map((t) => (
        <p key={t.id}>
          T{t.id} · {t.state} · {(t.speed_mps * 3.6).toFixed(1)} km/h ·{" "}
          {t.passengers}/{t.capacity} · {t.cars.length} cars [
          {t.cars.join(" / ")}]
          {t.cars.length !== m.config.cars
            ? " (consist change at empty terminal)"
            : ""}{" "}
          · M{t.station + 1} → M{t.next + 1} · departure in{" "}
          {Math.max(0, t.departure_s - m.time).toFixed(0)} s · delay{" "}
          {t.delay_s.toFixed(0)} s
        </p>
      ))}
    </div>
  );
}
