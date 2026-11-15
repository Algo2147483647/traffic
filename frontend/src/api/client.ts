import type { City, Snapshot } from "./models";
import {
  scene,
  ui,
  emit,
  receiveCity,
  receiveSnapshot,
  setConnection,
} from "../state/store";
async function json<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init);
  if (!response.ok) throw new Error(await response.text());
  return response.json();
}
// All backend I/O is here; controls are serialized so a later intent cannot overtake an earlier one.
let pending: Promise<unknown> = Promise.resolve();
export function control(
  action: string,
  extras: Record<string, unknown> = {},
): Promise<Snapshot> {
  const task = pending.then(async () => {
    try {
      const snapshot = await json<Snapshot>("/api/control", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action, ...extras }),
      });
      ui.error = "";
      receiveSnapshot(snapshot);
      return snapshot;
    } catch (e) {
      ui.error = String(e instanceof Error ? e.message : e);
      emit();
      throw e;
    }
  });
  pending = task.catch(() => {});
  return task;
}
export async function loadPresets() {
  const result = await json<{
    presets: typeof ui.presets;
    current_preset_id: string;
  }>("/api/presets");
  ui.presets = result.presets;
  ui.presetID = result.current_preset_id;
  emit();
}
export async function switchPreset(id: string) {
  await control("set_preset", { preset_id: id });
  receiveCity(await json<City>("/api/city"));
  ui.presetID = id;
  emit();
}
export function connect() {
  let disposed = false;
  const events = new EventSource("/api/stream");
  const safe = (fn: () => void) => {
    try {
      if (!disposed) fn();
    } catch (e) {
      ui.error = `Invalid model message: ${String(e)}`;
      emit();
    }
  };
  events.addEventListener("open", () =>
    setConnection("live", "Live model connected"),
  );
  events.addEventListener("city", (event: MessageEvent) =>
    safe(() => {
      receiveCity(JSON.parse(event.data));
      void loadPresets().catch(report);
    }),
  );
  events.addEventListener("snapshot", (event: MessageEvent) =>
    safe(() => receiveSnapshot(JSON.parse(event.data))),
  );
  events.addEventListener("error", () => {
    if (!disposed) setConnection("error", "Reconnecting to model");
  });
  void Promise.all([
    loadPresets(),
    json<typeof ui.workers>("/api/algorithms/workers").then((v) => {
      ui.workers = v;
      emit();
    }),
    json<typeof ui.dispatch>("/api/dispatch/algorithms").then((v) => {
      ui.dispatch = v;
      emit();
    }),
  ]).catch(report);
  return () => {
    disposed = true;
    events.close();
  };
}
export function report(error: unknown) {
  ui.error = error instanceof Error ? error.message : String(error);
  emit();
}
export function exportRun() {
  const url = URL.createObjectURL(
    new Blob(
      [
        JSON.stringify(
          { schema_version: 1, city: scene.city, snapshot: scene.snapshot },
          null,
          2,
        ),
      ],
      { type: "application/json" },
    ),
  );
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = "taxi-run.json";
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export async function exportCommunications() {
  const data = await json<unknown>("/api/communications");
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(data, null, 2)], { type: "application/json" }),
  );
  const a = document.createElement("a");
  a.href = url;
  a.download = "communications-observation.json";
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
