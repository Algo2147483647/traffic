import { useEffect, useRef } from "react";
import { scene, emit, subscribe } from "../state/store";
import { createRenderer } from "../rendering/renderer";
import { mapController } from "../rendering/controller";
import { bindMapGestures } from "../interaction/mapGestures";
export function MapCanvas() {
  const canvas = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const renderer = createRenderer(canvas.current, scene, emit, () => {
      const panel = document.querySelector(".right-stack");
      return {
        stats: document.querySelector(".stats-overlay").getBoundingClientRect(),
        details: panel.getBoundingClientRect(),
        detailsOpen: !panel.classList.contains("collapsed"),
      };
    });
    mapController.current = renderer;
    const gestures = bindMapGestures(canvas.current, renderer);
    let city = scene.city,
      geometry = "",
      frame = 0;
    const refresh = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        if (city !== scene.city) {
          city = scene.city;
          renderer.updateCity();
        } else {
          const next = JSON.stringify(
            scene.snapshot?.metro?.lines.map((l) => [
              l.id,
              l.elevation_m,
              l.track.length,
            ]),
          );
          if (next !== geometry) {
            geometry = next;
            renderer.fitCanvas();
          } else renderer.draw();
        }
      });
    };
    const stop = subscribe(refresh);
    const observer = new ResizeObserver(() => renderer.fitCanvas());
    observer.observe(canvas.current);
    if (city) renderer.updateCity();
    return () => {
      stop();
      gestures();
      observer.disconnect();
      cancelAnimationFrame(frame);
      mapController.current = null;
    };
  }, []);
  return (
    <canvas
      ref={canvas}
      id="city-canvas"
      tabIndex={0}
      aria-label="Interactive lane-level map of Meridian City traffic"
    />
  );
}
