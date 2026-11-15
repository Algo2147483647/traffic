import { useSyncExternalStore } from "react";
import type { City, Snapshot, MetroNetworkJourney } from "../api/models";
export type Selection = {
  type: "person" | "intersection" | "interchange" | "order" | "metro-station";
  id: string | number;
} | null;
export interface Camera {
  zoom: number;
  panX: number;
  panY: number;
}
export interface Transform {
  baseScale: number;
  baseOx: number;
  baseOy: number;
  width: number;
  height: number;
}
export interface SceneState {
  city: City;
  snapshot: Snapshot;
  selected: Selection;
  metroPassenger: MetroNetworkJourney;
  layerFilter: string;
  heightMin: number;
  heightMax: number;
  showTransforms: boolean;
  showCoordinates: boolean;
  camera: Camera;
  transform: Transform;
  laneDirections: Map<
    string,
    Map<
      number,
      Array<{
        toWayID: string;
        turn: string;
        angle: number;
      }>
    >
  >;
}
const listeners = new Set<() => void>();
let revision = 0;
const savedCoordinates = () => {
  try {
    return localStorage.getItem("traffic-show-coordinates") !== "false";
  } catch {
    return true;
  }
};
export const scene: SceneState = {
  city: null,
  snapshot: null,
  selected: null,
  metroPassenger: null,
  layerFilter: "all",
  heightMin: -5,
  heightMax: 5,
  showTransforms: false,
  showCoordinates: savedCoordinates(),
  camera: { zoom: 1, panX: 0, panY: 0 },
  transform: null,
  laneDirections: new Map(),
};
export const ui = {
  settings: false,
  rail: false,
  collapsed: false,
  connection: "connecting",
  connectionLabel: "Connecting",
  error: "",
  presets: [] as Array<{
    id: string;
    name: string;
  }>,
  presetID: "",
  workers: [] as Array<{
    id: string;
    name: string;
    capability: string;
  }>,
  dispatch: [] as Array<{
    id: string;
    name: string;
  }>,
  computeHz: 0,
};
export function emit() {
  revision++;
  listeners.forEach((f) => f());
}
export function subscribe(f: () => void) {
  listeners.add(f);
  return () => {
    listeners.delete(f);
  };
}
export function useAppState() {
  useSyncExternalStore(subscribe, () => revision);
  return { scene, ui };
}
export function select(value: Selection) {
  const changed =
    scene.selected?.id !== value?.id || scene.selected?.type !== value?.type;
  scene.selected = value;
  if (changed) scene.metroPassenger = null;
  ui.collapsed = false;
  reconcilePassenger();
  emit();
}
export function reconcilePassenger() {
  const m = scene.snapshot?.metro;
  if (scene.selected?.type !== "metro-station" || !m) return;
  const d = m.station_details?.find((x) => x.id === scene.selected.id);
  if (scene.metroPassenger) {
    scene.metroPassenger = m.journeys?.find(
      (j) => j.id === scene.metroPassenger.id,
    ) || {
      ...scene.metroPassenger,
      state: "completed",
      step: scene.metroPassenger.plan.length,
    };
  } else
    scene.metroPassenger =
      m.journeys?.find((j) => d?.passenger_ids.includes(j.id)) || null;
}
let lastTick = 0,
  lastAt = 0;
export function receiveSnapshot(snapshot: Snapshot) {
  const prev = scene.snapshot;
  if (
    prev &&
    snapshot.kernel.run === prev.kernel.run &&
    snapshot.kernel.topology === prev.kernel.topology &&
    snapshot.kernel.world < prev.kernel.world
  )
    return;
  if (prev && snapshot.kernel.run !== prev.kernel.run) {
    scene.metroPassenger = null;
    scene.selected = null;
  }
  if (prev?.mode !== snapshot.mode) scene.selected = null;
  scene.snapshot = snapshot;
  const now = performance.now();
  if (lastAt && snapshot.tick >= lastTick) {
    const hz = ((snapshot.tick - lastTick) * 1000) / (now - lastAt);
    ui.computeHz = ui.computeHz ? ui.computeHz * 0.72 + hz * 0.28 : hz;
  }
  lastAt = now;
  lastTick = snapshot.tick;
  reconcilePassenger();
  emit();
}
export function receiveCity(city: City) {
  scene.city = city;
  scene.selected = null;
  scene.metroPassenger = null;
  scene.camera = { zoom: 1, panX: 0, panY: 0 };
  emit();
}
export function setConnection(status: string, label: string) {
  ui.connection = status;
  ui.connectionLabel = label;
  emit();
}
