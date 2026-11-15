import { control, switchPreset, exportRun } from "../api/client";
import { scene, ui, emit } from "../state/store";
import { mapController } from "../rendering/controller";
export type Draft = Record<string, string | boolean>;
export function number(d: Draft, id: string) {
  const n = Number(d[id]);
  if (!Number.isFinite(n)) throw new Error("Enter a finite number");
  return n;
}
function parameters(text: string | boolean) {
  const p = JSON.parse(String(text || "{}"));
  if (
    !p ||
    Array.isArray(p) ||
    typeof p !== "object" ||
    Object.values(p).some((v) => typeof v !== "number" || !Number.isFinite(v))
  )
    throw new Error("Parameters must be a JSON object of finite numbers");
  return p;
}
function taxi(d: Draft) {
  return {
    fleet: number(d, "taxi-fleet"),
    orders_per_minute: number(d, "taxi-demand"),
    algorithm: d["taxi-algorithm"],
    seed: number(d, "taxi-seed"),
    dispatch_interval_s: number(d, "taxi-interval"),
    max_wait_s: 600,
    dwell_s: 20,
    parameters: parameters(d["taxi-parameters"]),
  };
}
export async function settingsAction(id: string, d: Draft) {
  const map = mapController.current;
  switch (id) {
    case "pause-button":
      return control(scene.snapshot?.paused ? "resume" : "pause");
    case "step-button":
      return control("step", { delta: 0.05 });
    case "reset-button":
      scene.selected = null;
      scene.metroPassenger = null;
      return control("reset");
    case "time-scale":
      return control("set_time_scale", { time_scale: number(d, id) });
    case "walk-speed-control":
      return control("set_walk_speed", { walk_speed_mps: number(d, id) });
    case "safe-gap-control":
      return control("set_spacing", { safe_gap: number(d, id) });
    case "map-preset":
      return switchPreset(String(d[id]));
    case "simulation-mode":
      return control("set_mode", {
        mode: d[id],
        taxi: d[id] !== "traffic" ? taxi(d) : {},
      });
    case "zoom-in-button":
    case "zoom-out-button":
      if (scene.transform)
        map?.zoomAt(
          id === "zoom-in-button" ? 1.5 : 1 / 1.5,
          scene.transform.width / 2,
          scene.transform.height / 2,
        );
      return;
    case "fit-button":
      map?.resetView();
      map?.fitCanvas();
      return;
    case "fullscreen-button":
      if (document.fullscreenElement) await document.exitFullscreen();
      else await document.documentElement.requestFullscreen();
      return;
    case "show-coordinates":
      scene.showCoordinates = !!d[id];
      try {
        localStorage.setItem(
          "traffic-show-coordinates",
          String(scene.showCoordinates),
        );
      } catch {}
      emit();
      return;
    case "layer-filter":
    case "height-min":
    case "height-max": {
      const lo = number(d, "height-min"),
        hi = number(d, "height-max");
      if (lo > hi) throw new Error("Enter a finite range with From ≤ To");
      scene.heightMin = lo;
      scene.heightMax = hi;
      scene.layerFilter = String(d["layer-filter"]);
      map?.resetView();
      map?.fitCanvas();
      emit();
      return;
    }
    case "apply-load-button":
      return control("set_load", {
        population: number(d, "population-control"),
        spawn_rate_per_second: number(d, "spawn-rate-control") / 60,
        max_population: Math.max(
          number(d, "population-control"),
          number(d, "max-population-control"),
        ),
      });
    case "taxi-apply":
      return control("set_taxi_config", { taxi: taxi(d) });
    case "taxi-load-apply":
      return control("set_taxi_load", {
        taxi: {
          fleet: number(d, "taxi-fleet"),
          orders_per_minute: number(d, "taxi-demand"),
        },
      });
    case "taxi-export":
      exportRun();
      return;
    case "shared-apply":
      return control("set_demand", {
        demand: {
          trips_per_minute: number(d, "shared-rate"),
          taxi_share: number(d, "shared-share") / 100,
          metro_share: number(d, "shared-metro") / 100,
        },
      });
    case "metro-apply":
      return control("set_metro", {
        metro: {
          ...scene.snapshot.metro.config,
          service: scene.snapshot.metro.config.service,
          line_elevations_m: Object.fromEntries(
            scene.snapshot.metro.lines.map((l) => [
              String(l.id),
              number(d, `metro-height-${l.id}`),
            ]),
          ),
          cars: number(d, "metro-cars"),
          max_speed_kmh: number(d, "metro-speed"),
          enabled: !!d["metro-enabled"],
          headway_s: number(d, "metro-headway"),
          passengers_per_minute: number(d, "metro-demand"),
          capacity: number(d, "metro-capacity"),
          dwell_s: number(d, "metro-dwell"),
        },
      });
    case "road-algorithm-apply": {
      const p = parameters(d["road-algorithm-parameters"]);
      return control("set_algorithms", {
        algorithms: Object.fromEntries(
          ["route", "signal", "demand", "supply", "motion", "metro"].map(
            (k) => [k, { id: d[k + "-algorithm"], parameters: p }],
          ),
        ),
      });
    }
  }
}
export const immediateFields = new Set([
  "time-scale",
  "walk-speed-control",
  "safe-gap-control",
  "map-preset",
  "simulation-mode",
  "show-coordinates",
  "layer-filter",
  "height-min",
  "height-max",
]);
