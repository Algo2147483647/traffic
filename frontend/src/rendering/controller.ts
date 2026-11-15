import type { MapRenderer } from "./renderer";
export const mapController: {
  current: MapRenderer;
} = { current: null };
export function fitPassenger() {
  const panel = document.querySelector(".right-stack");
  const width =
    panel && !panel.classList.contains("collapsed")
      ? panel.getBoundingClientRect().left
      : innerWidth;
  mapController.current?.fitMetroPassengerRoute(Math.max(200, width));
}
