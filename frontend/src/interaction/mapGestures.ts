import { scene as state, select, emit } from "../state/store";
import type { MapRenderer } from "../rendering/renderer";
export function bindMapGestures(
  canvas: HTMLCanvasElement,
  renderer: MapRenderer,
) {
  const { draw, screen, zoomAt, nodeVisible, levelVisible, personLevel } =
    renderer;
  const pointers = new Map<
    number,
    {
      x: number;
      y: number;
    }
  >();
  const cleanup: Array<() => void> = [];
  const on = (
    name: string,
    listener: (event: any) => void,
    options?: AddEventListenerOptions,
  ) => {
    canvas.addEventListener(name, listener, options);
    cleanup.push(() => canvas.removeEventListener(name, listener, options));
  };
  function canvasPoint(event: MouseEvent) {
    const rect = canvas.getBoundingClientRect();
    return { x: event.clientX - rect.left, y: event.clientY - rect.top };
  }
  on(
    "wheel",
    (event) => {
      event.preventDefault();
      const p = canvasPoint(event);
      zoomAt(Math.exp(-event.deltaY * 0.0015), p.x, p.y);
    },
    { passive: false },
  );
  on("pointerdown", (event) => {
    if (event.isTrusted) canvas.setPointerCapture(event.pointerId);
    pointers.set(event.pointerId, canvasPoint(event));
    canvas.classList.add("is-panning");
  });
  on("pointermove", (event) => {
    if (!pointers.has(event.pointerId)) return;
    const before = new Map(pointers),
      next = canvasPoint(event);
    pointers.set(event.pointerId, next);
    if (pointers.size === 1) {
      const old = before.get(event.pointerId);
      state.camera.panX += next.x - old.x;
      state.camera.panY += next.y - old.y;
      draw();
    } else if (pointers.size === 2) {
      const oldPoints = [...before.values()],
        newPoints = [...pointers.values()];
      const oldDistance = Math.hypot(
          oldPoints[1].x - oldPoints[0].x,
          oldPoints[1].y - oldPoints[0].y,
        ),
        newDistance = Math.hypot(
          newPoints[1].x - newPoints[0].x,
          newPoints[1].y - newPoints[0].y,
        );
      const oldCenter = {
          x: (oldPoints[0].x + oldPoints[1].x) / 2,
          y: (oldPoints[0].y + oldPoints[1].y) / 2,
        },
        center = {
          x: (newPoints[0].x + newPoints[1].x) / 2,
          y: (newPoints[0].y + newPoints[1].y) / 2,
        };
      state.camera.panX += center.x - oldCenter.x;
      state.camera.panY += center.y - oldCenter.y;
      zoomAt(oldDistance ? newDistance / oldDistance : 1, center.x, center.y);
    }
  });
  function endPointer(event) {
    pointers.delete(event.pointerId);
    if (!pointers.size) canvas.classList.remove("is-panning");
  }
  on("pointerup", endPointer);
  on("pointercancel", endPointer);
  on("click", (event) => {
    if (!state.snapshot || !state.transform) return;
    const click = canvasPoint(event);
    const metro = state.snapshot.metro;
    const exchange = (metro?.interchanges || [])
      .filter((x) =>
        x.stations.some((id) =>
          levelVisible(metro.stations.find((s) => s.id === id)?.elevation_m),
        ),
      )
      .map((x) => ({
        site: x,
        d: Math.hypot(
          screen(x.position).x - click.x,
          screen(x.position).y - click.y,
        ),
      }))
      .filter((x) => x.d < 14)
      .sort((a, b) => a.d - b.d)[0]?.site;
    if (exchange) {
      select({ type: "metro-station", id: exchange.id });
      draw();
      return;
    }
    const t = state.snapshot.taxi;
    if (t && levelVisible(0)) {
      const stop = t.stops.find((st) => {
        const p = screen(st.position);
        return Math.hypot(p.x - click.x, p.y - click.y) < 10;
      });
      if (stop) {
        const o = t.orders.find(
          (o) =>
            o.pickup === stop.id &&
            ["Waiting", "Assigned", "Boarding"].includes(o.state),
        );
        if (o) {
          select({ type: "order", id: o.id });
          draw();
          return;
        }
      }
    }
    let intersectionMatch = null,
      intersectionDistance = 18;
    state.city.intersections
      .filter(nodeVisible)
      .sort((a, b) => (a.elevation || 0) - (b.elevation || 0))
      .forEach((item) => {
        const p = screen(item.position),
          d = Math.hypot(p.x - click.x, p.y - click.y);
        if (d < intersectionDistance) {
          intersectionMatch = item;
          intersectionDistance = d;
        }
      });
    const group = (state.city.interchanges || [])
      .filter((g) =>
        state.city.intersections.some(
          (n) => n.interchange === g.id && nodeVisible(n),
        ),
      )
      .find(
        (g) =>
          intersectionMatch?.interchange === g.id ||
          Math.hypot(
            screen(g.center).x - click.x,
            screen(g.center).y - click.y,
          ) < 45,
      );
    let selection = state.selected;
    if (group) selection = { type: "interchange", id: group.id };
    else if (intersectionMatch)
      selection = { type: "intersection", id: intersectionMatch.id };
    else {
      let personMatch = null,
        distance = 13;
      state.snapshot.people
        .filter((p) => levelVisible(personLevel(p)))
        .forEach((person) => {
          const p = screen(person.position),
            d = Math.hypot(p.x - click.x, p.y - click.y);
          if (d < distance) {
            personMatch = person;
            distance = d;
          }
        });
      if (personMatch) selection = { type: "person", id: personMatch.id };
    }
    select(selection);
    draw();
  });

  return () => {
    cleanup.forEach((f) => f());
    pointers.clear();
  };
}
