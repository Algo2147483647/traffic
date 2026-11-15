import type { SceneState } from "../state/store";
import type { Snapshot } from "../api/models";
import * as MetroRoute from "./metroRoute";
import * as HeightSlice from "./heightSlice";
export function createRenderer(
  canvas: HTMLCanvasElement,
  source: SceneState,
  changed: () => void,
  overlays: () => {
    stats: DOMRect;
    details: DOMRect;
    detailsOpen: boolean;
  },
) {
  let state = source;
  let ctx = canvas.getContext("2d")!;
  let renderStructure: string = null;
  let renderLevel: number = null;
  function fitCanvas() {
    const rect = canvas.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) return;
    const ratio = Math.min(window.devicePixelRatio || 1, 2);
    canvas.width = Math.max(1, Math.round(rect.width * ratio));
    canvas.height = Math.max(1, Math.round(rect.height * ratio));
    ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    if (!state.city) return;
    const b = displayBounds(),
      padding = 50;
    const baseScale = Math.min(
      (rect.width - padding * 2) / (b.max_x - b.min_x),
      (rect.height - padding * 2) / (b.max_y - b.min_y),
    );
    const baseOx =
      (rect.width - (b.max_x - b.min_x) * baseScale) / 2 - b.min_x * baseScale;
    const baseOy =
      (rect.height - (b.max_y - b.min_y) * baseScale) / 2 - b.min_y * baseScale;
    state.transform = {
      baseScale,
      baseOx,
      baseOy,
      width: rect.width,
      height: rect.height,
    };
    draw();
  }
  function currentTransform() {
    const t = state.transform,
      c = state.camera;
    return {
      scale: t.baseScale * c.zoom,
      ox: t.baseOx + c.panX,
      oy: t.baseOy + c.panY,
      width: t.width,
      height: t.height,
    };
  }
  function screen(point) {
    const t = currentTransform();
    return { x: point.x * t.scale + t.ox, y: point.y * t.scale + t.oy };
  }
  function zoomAt(factor, x, y) {
    if (!state.transform) return;
    const old = currentTransform();
    const world = { x: (x - old.ox) / old.scale, y: (y - old.oy) / old.scale };
    state.camera.zoom = Math.max(
      0.03,
      Math.min(64, state.camera.zoom * factor),
    );
    const scale = state.transform.baseScale * state.camera.zoom;
    state.camera.panX = x - state.transform.baseOx - world.x * scale;
    state.camera.panY = y - state.transform.baseOy - world.y * scale;
    changed();
    draw();
  }
  function resetView() {
    state.camera = { zoom: 1, panX: 0, panY: 0 };
    changed();
    draw();
  }
  function traceBezier(points, tension = 0.55) {
    if (!points.length) return;
    ctx.moveTo(points[0].x, points[0].y);
    if (points.length === 1) return;
    for (let index = 0; index < points.length - 1; index++) {
      const before = points[Math.max(0, index - 1)],
        start = points[index],
        end = points[index + 1],
        after = points[Math.min(points.length - 1, index + 2)];
      const control1 = {
        x: start.x + ((end.x - before.x) * tension) / 6,
        y: start.y + ((end.y - before.y) * tension) / 6,
      };
      const control2 = {
        x: end.x - ((after.x - start.x) * tension) / 6,
        y: end.y - ((after.y - start.y) * tension) / 6,
      };
      ctx.bezierCurveTo(
        control1.x,
        control1.y,
        control2.x,
        control2.y,
        end.x,
        end.y,
      );
    }
  }
  function drawPolyline(points) {
    traceBezier(points.map(screen));
  }
  function wayLength(way) {
    if (way.length) return way.length;
    return way.points
      .slice(1)
      .reduce(
        (sum, point, index) =>
          sum +
          Math.hypot(
            point.x - way.points[index].x,
            point.y - way.points[index].y,
          ),
        0,
      );
  }
  function sampleWay(way, along, lane = 0, extraOffset = 0) {
    const total = wayLength(way),
      target = Math.max(0, Math.min(total, along));
    let covered = 0;
    for (let index = 0; index < way.points.length - 1; index++) {
      const a = way.points[index],
        b = way.points[index + 1],
        dx = b.x - a.x,
        dy = b.y - a.y,
        length = Math.hypot(dx, dy);
      if (covered + length >= target || index === way.points.length - 2) {
        const portion = length ? (target - covered) / length : 0,
          tx = dx / length,
          ty = dy / length;
        const offset =
          (way.carriageway_offset || 0) +
          (lane - (way.lanes - 1) / 2) * way.lane_width +
          extraOffset;
        return {
          x: a.x + dx * portion - ty * offset,
          y: a.y + dy * portion + tx * offset,
          tx,
          ty,
        };
      }
      covered += length;
    }
    return { ...way.points[0], tx: 1, ty: 0 };
  }
  function drawWayRange(
    way,
    from,
    to,
    lane,
    color,
    width,
    extraOffset = 0,
    lineCap: CanvasLineCap = "round",
  ) {
    const interval = wayInterval(way);
    if (!interval) return;
    const length = wayLength(way),
      lo = Math.max(Math.min(from, to), interval[0] * length),
      hi = Math.min(Math.max(from, to), interval[1] * length);
    if (hi <= lo) return;
    from = lo;
    to = hi;
    ctx.strokeStyle = color;
    ctx.lineWidth = width;
    ctx.lineCap = lineCap;
    ctx.lineJoin = "round";
    ctx.beginPath();
    const distance = Math.abs(to - from),
      steps =
        way.geometry === "sampled"
          ? Math.max(4, Math.ceil(distance / 6))
          : Math.max(4, Math.min(36, Math.ceil(distance / 160)));
    const points = [];
    for (let index = 0; index <= steps; index++) {
      points.push(
        screen(
          sampleWay(
            way,
            from + ((to - from) * index) / steps,
            lane,
            extraOffset,
          ),
        ),
      );
    }
    if (way.geometry === "sampled") {
      points.forEach((p, i) =>
        i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y),
      );
    } else traceBezier(points);
    ctx.stroke();
  }
  function draw() {
    if (!state.city || !state.transform) return;
    const { width, height } = currentTransform();
    ctx.clearRect(0, 0, width, height);
    drawBackdrop(width, height);
    drawMetro();
    const levels = [null];
    for (const level of levels) {
      renderLevel = level;
      for (const layer of ["tunnel", "ground", "ramp", "bridge"]) {
        renderStructure = layer;
        drawWays();
        drawIntersectionPads();
        drawSelectedTrip();
        drawSignals();
        drawLaneDirectionMarkings();
        drawIntersections();
        drawTaxiStops();
        drawPeople();
      }
    }
    renderStructure = null;
    renderLevel = null;
    const groups = new Map();
    state.city.intersections.forEach((n) => {
      if (n.interchange && nodeVisible(n) && !groups.has(n.interchange))
        groups.set(n.interchange, n);
    });
    ctx.font = "12px Times New Roman";
    ctx.fillStyle = "#dddddd";
    groups.forEach((n, id) => {
      const members = state.city.intersections.filter(
          (i) => i.interchange === id,
        ),
        p = screen({
          x: members.reduce((a, n) => a + n.position.x, 0) / members.length,
          y: members.reduce((a, n) => a + n.position.y, 0) / members.length,
        });
      ctx.fillText(
        (state.city.interchanges || []).find((g) => g.id === id)?.name ||
          `${id} interchange`,
        p.x + 12,
        p.y - 28,
      );
    });
    drawMetroPassengerRoute();
    drawCoordinateRulers();
  }
  function drawCoordinateRulers() {
    if (!state.showCoordinates) return;
    const t = currentTransform(),
      b = displayBounds(),
      units = state.city.world_units_per_km || 1000;
    const nice = (value) => {
      const power = 10 ** Math.floor(Math.log10(value)),
        n = value / power;
      return (n >= 5 ? 5 : n >= 2 ? 2 : 1) * power;
    };
    const step = nice(110 / t.scale / units),
      stepWorld = step * units;
    const format = (value) =>
      Number(
        value.toFixed(Math.max(0, -Math.floor(Math.log10(step)) + 1)),
      ).toString();
    const { stats, details, detailsOpen } = overlays();
    const left = 80,
      top = 26,
      right = detailsOpen
        ? Math.min(t.width - 68, details.left - 14)
        : t.width - 68;
    const bottom = Math.min(t.height - 50, stats.top - 12);
    ctx.save();
    ctx.font = "12px Times New Roman";
    const transferLabels = new Set();
    ctx.fillStyle = "#bcbcbc";
    ctx.strokeStyle = "#777777";
    ctx.lineWidth = 1;
    ctx.shadowColor = "#080808";
    ctx.shadowBlur = 5;
    // Edge rulers only: no lines cross the road surface.
    ctx.beginPath();
    ctx.moveTo(left, top);
    ctx.lineTo(right, top);
    ctx.moveTo(left, top);
    ctx.lineTo(left, bottom);
    ctx.stroke();
    const ticks = (min, max, origin, screenMin, screenMax, drawTick) => {
      const start = Math.max(min, (screenMin - origin) / t.scale),
        end = Math.min(max, (screenMax - origin) / t.scale);
      for (
        let i = Math.ceil(start / stepWorld - 1e-9);
        i * stepWorld <= end + 1e-8;
        i++
      )
        drawTick(origin + i * stepWorld * t.scale, format(i * step));
    };
    ctx.textAlign = "center";
    ctx.textBaseline = "bottom";
    ticks(b.min_x, b.max_x, t.ox, left, right, (x, label) => {
      ctx.beginPath();
      ctx.moveTo(x, top);
      ctx.lineTo(x, top + 5);
      ctx.stroke();
      ctx.fillText(label, x, top - 4);
    });
    ctx.textAlign = "right";
    ctx.textBaseline = "middle";
    ticks(b.min_y, b.max_y, t.oy, top, bottom, (y, label) => {
      ctx.beginPath();
      ctx.moveTo(left, y);
      ctx.lineTo(left + 5, y);
      ctx.stroke();
      ctx.fillText(label, left - 7, y);
    });
    ctx.textAlign = "left";
    ctx.fillText("X · km", left + 10, top - 12);
    ctx.fillText("Y · km", left + 10, top + 20);
    const km = nice(130 / t.scale / units),
      pixels = km * units * t.scale,
      x = (t.width - pixels) / 2,
      y = t.height - 24;
    ctx.beginPath();
    ctx.moveTo(x, y - 5);
    ctx.lineTo(x, y);
    ctx.lineTo(x + pixels, y);
    ctx.lineTo(x + pixels, y - 5);
    ctx.stroke();
    ctx.textAlign = "center";
    ctx.textBaseline = "bottom";
    ctx.fillText(`${Number(km.toPrecision(4))} km`, t.width / 2, y - 6);
    ctx.textBaseline = "top";
    ctx.font = "11px Times New Roman";
    ctx.fillText(
      `${Number(((b.max_x - b.min_x) / units).toFixed(3))} × ${Number(((b.max_y - b.min_y) / units).toFixed(3))} km`,
      t.width / 2,
      y + 4,
    );
    ctx.restore();
  }
  function levelVisible(height) {
    return (
      state.layerFilter === "all" ||
      (height >= state.heightMin - 1e-6 && height <= state.heightMax + 1e-6)
    );
  }
  function wayInterval(way) {
    return HeightSlice.interval(
      way.start_elevation || 0,
      way.end_elevation || 0,
      state.layerFilter === "all" ? null : [state.heightMin, state.heightMax],
    );
  }
  function wayVisible(way) {
    return (
      (!renderStructure || (way.structure || "ground") === renderStructure) &&
      !!wayInterval(way)
    );
  }
  function nodeVisible(node) {
    const z = node.elevation || 0;
    return (
      levelVisible(z) &&
      (!renderStructure ||
        renderStructure === (z < 0 ? "tunnel" : z > 0 ? "bridge" : "ground"))
    );
  }
  function personLevel(person) {
    return person.elevation || 0;
  }
  function displayBounds() {
    const points = [];
    for (const line of state.snapshot?.metro?.lines || [])
      if (levelVisible(line.elevation_m)) points.push(...(line.track || []));
    if (state.layerFilter === "all") {
      const b = state.city.bounds;
      points.push({ x: b.min_x, y: b.min_y }, { x: b.max_x, y: b.max_y });
    } else
      for (const w of state.city.ways)
        if (wayInterval(w)) points.push(...w.points);
    if (!points.length) return state.city.bounds;
    const xs = points.map((p) => p.x),
      ys = points.map((p) => p.y),
      min_x = Math.min(...xs),
      max_x = Math.max(...xs),
      min_y = Math.min(...ys),
      max_y = Math.max(...ys);
    return {
      min_x,
      max_x: max_x === min_x ? max_x + 200 : max_x,
      min_y: min_y === max_y ? min_y - 100 : min_y,
      max_y: min_y === max_y ? max_y + 100 : max_y,
    };
  }
  function drawMetro() {
    const m = state.snapshot?.metro;
    if (!m) return;
    const scale = currentTransform().scale,
      colors = ["#72bde8", "#c7a1ef", "#7ed4b1"];
    ctx.save();
    ctx.font = "12px Times New Roman";
    const transferLabels = new Set();
    for (const line of [...m.lines].sort(
      (a, b) => a.elevation_m - b.elevation_m,
    )) {
      if (!line.track?.length || !levelVisible(line.elevation_m)) continue;
      const color = colors[line.id % colors.length];
      const trace = () => {
        ctx.beginPath();
        for (const track of [line.track, line.reverse_track || []])
          track.forEach((v, i) => {
            const p = screen(v);
            i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y);
          });
      };
      ctx.lineJoin = "round";
      ctx.lineCap = "round";
      trace();
      ctx.strokeStyle = color;
      ctx.lineWidth = line.tunnel_outer_diameter_m * scale;
      ctx.stroke();
      trace();
      ctx.strokeStyle = "#080808";
      ctx.lineWidth = line.tunnel_inner_diameter_m * scale;
      ctx.stroke();
      for (const id of line.stations) {
        const st = m.stations[id],
          p = screen(st.position);
        ctx.fillStyle = color;
        ctx.beginPath();
        ctx.arc(p.x, p.y, 2.5 * scale, 0, Math.PI * 2);
        ctx.fill();
        if (st.transfer) {
          ctx.beginPath();
          ctx.arc(p.x, p.y, 6 * scale, 0, Math.PI * 2);
          ctx.strokeStyle = color;
          ctx.lineWidth = Math.max(0.7, scale);
          ctx.stroke();
        }
        if (scale > 0.08 && (!st.transfer || !transferLabels.has(st.transfer)))
          ctx.fillText(st.name + " / " + st.waiting, p.x + 8, p.y - 10);
        if (st.transfer) transferLabels.add(st.transfer);
      }
      for (const t of m.trains.filter(
        (t) =>
          t.line === line.id && t.state !== "depot" && t.state !== "withdrawn",
      )) {
        const w = t.car_length_m * scale,
          h = t.car_width_m * scale,
          gap = 0.3 * scale;
        for (let i = 0; i < t.cars.length; i++) {
          const pose = t.car_poses?.[i];
          if (!pose) continue;
          const p = screen(pose.position);
          ctx.save();
          ctx.translate(p.x, p.y);
          ctx.rotate(pose.heading);
          ctx.fillStyle = color;
          ctx.fillRect(-w / 2 + gap / 2, -h / 2, w - gap, h);
          const n = t.cars[i],
            cols = Math.max(1, Math.ceil(Math.sqrt((n * (w - gap)) / h))),
            rows = Math.max(1, Math.ceil(n / cols));
          const dx = (w - gap) / cols,
            dy = h / rows,
            r = Math.min(0.15 * scale, dx * 0.32, dy * 0.32);
          ctx.fillStyle = "#111";
          for (let k = 0; k < n; k++) {
            ctx.beginPath();
            ctx.arc(
              -w / 2 + gap / 2 + ((k % cols) + 0.5) * dx,
              -h / 2 + (Math.floor(k / cols) + 0.5) * dy,
              r,
              0,
              Math.PI * 2,
            );
            ctx.fill();
          }
          ctx.restore();
        }
        const p = screen(t.position);
        ctx.fillStyle = color;
        ctx.fillText(
          "T" + t.id + " / " + t.passengers,
          p.x + 8,
          p.y + (t.direction > 0 ? 20 : -14),
        );
      }
    }
    ctx.restore();
  }
  function fitMetroPassengerRoute(
    availableWidth = canvas.getBoundingClientRect().width,
  ) {
    const j = state.metroPassenger,
      m = state.snapshot?.metro;
    if (!j || !state.transform) return;
    const points = [];
    for (const step of j.plan) {
      if (step.line >= 0) {
        const line = m.lines.find((l) => l.id === step.line);
        if (line && levelVisible(line.elevation_m))
          points.push(...MetroRoute.points(line, step.from, step.to));
      } else
        for (const id of [step.from, step.to]) {
          const st = m.stations.find((s) => s.id === id);
          if (st && levelVisible(st.elevation_m)) points.push(st.position);
        }
    }
    if (!points.length) return;
    const rect = canvas.getBoundingClientRect();
    const width = availableWidth,
      height = rect.height,
      padding = 80;
    const minX = Math.min(...points.map((p) => p.x)),
      maxX = Math.max(...points.map((p) => p.x)),
      minY = Math.min(...points.map((p) => p.y)),
      maxY = Math.max(...points.map((p) => p.y)),
      scale = Math.min(
        (width - 2 * padding) / Math.max(200, maxX - minX),
        (height - 2 * padding) / Math.max(200, maxY - minY),
      );
    state.camera.zoom = scale / state.transform.baseScale;
    state.camera.panX =
      width / 2 - ((minX + maxX) / 2) * scale - state.transform.baseOx;
    state.camera.panY =
      height / 2 - ((minY + maxY) / 2) * scale - state.transform.baseOy;
    draw();
  }
  function drawMetroPassengerRoute() {
    if (state.selected?.type !== "metro-station" || !state.metroPassenger)
      return;
    const m = state.snapshot.metro,
      j = state.metroPassenger,
      styles = {
        done: { color: "#8eabbc", width: 3, dash: [] },
        current: { color: "#f5ce58", width: 5, dash: [] },
        next: { color: "#8dbdff", width: 3, dash: [9, 6] },
      },
      markers = new Map();
    const mark = (id, status, transfer) => {
      const st = m.stations.find((s) => s.id === id);
      if (!st || !levelVisible(st.elevation_m)) return;
      const key = `${st.position.x.toFixed(1)},${st.position.y.toFixed(1)}`,
        old = markers.get(key),
        rank = { current: 3, next: 2, done: 1 };
      if (!old || rank[status] > rank[old.status])
        markers.set(key, {
          st,
          status,
          transfer: transfer || old?.transfer,
          transferStatus: transfer ? status : old?.transferStatus,
        });
      else if (transfer) {
        old.transfer = true;
        old.transferStatus = status;
      }
    };
    ctx.save();
    ctx.lineJoin = "round";
    ctx.lineCap = "round";
    for (const status of ["next", "done", "current"])
      j.plan.forEach((step, i) => {
        if (MetroRoute.status(i, j.step) !== status) return;
        const style = styles[status];
        if (step.line >= 0) {
          const line = m.lines.find((l) => l.id === step.line);
          if (!line || !levelVisible(line.elevation_m)) return;
          const path = MetroRoute.points(line, step.from, step.to);
          if (path.length < 2) return;
          ctx.setLineDash(style.dash);
          ctx.strokeStyle = style.color;
          ctx.lineWidth = style.width;
          ctx.globalAlpha = status === "done" ? 0.7 : 0.95;
          ctx.beginPath();
          path.forEach((v, k) => {
            const p = screen(v);
            k ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y);
          });
          ctx.stroke();
        }
        mark(step.from, status, step.line < 0);
        mark(step.to, status, step.line < 0);
      });
    ctx.globalAlpha = 1;
    ctx.setLineDash([]);
    ctx.font = "13px Times New Roman";
    for (const { st, status, transfer, transferStatus } of markers.values()) {
      const p = screen(st.position),
        style = styles[status];
      ctx.fillStyle = "#080808";
      ctx.strokeStyle = style.color;
      ctx.lineWidth = status === "current" ? 3 : 2;
      ctx.beginPath();
      ctx.arc(p.x, p.y, transfer ? 9 : 5, 0, Math.PI * 2);
      ctx.fill();
      ctx.stroke();
      if (transfer) {
        ctx.strokeStyle = styles[transferStatus || status].color;
        ctx.beginPath();
        ctx.arc(p.x, p.y, 5, 0, Math.PI * 2);
        ctx.stroke();
        ctx.fillStyle = style.color;
        ctx.fillText(st.name, p.x + 13, p.y + 18);
      }
    }
    ctx.restore();
  }
  function drawBackdrop(width, height) {
    ctx.fillStyle = "#080808";
    ctx.fillRect(0, 0, width, height);
  }
  function drawWays() {
    const scale = currentTransform().scale;
    const minimumWidths = {
      arterial: 4.8,
      ramp: 3.2,
      collector: 2.6,
      local: 1.35,
    };
    state.city.ways.forEach((way) => {
      if (!wayVisible(way)) return;
      const length = wayLength(way),
        centre = (way.lanes - 1) / 2;
      const radius = state.city.simulation.intersection_radius || 22;
      const start = Math.min(radius, length / 2),
        end = Math.max(start, length - radius);
      const physicalWidth = way.lanes * way.lane_width * scale;
      const width = Math.max(
        physicalWidth,
        minimumWidths[way.road_class] || 1.35,
      );
      if (way.structure === "bridge") {
        ctx.save();
        ctx.shadowColor = "#000000";
        ctx.shadowBlur = 10 * scale;
        ctx.shadowOffsetY = 4 * scale;
        drawWayRange(
          way,
          start,
          end,
          centre,
          "#090909",
          width + 8 * scale,
          0,
          "butt",
        );
        ctx.restore();
      }
      drawWayRange(
        way,
        start,
        end,
        centre,
        "rgba(218,218,218,.3)",
        width + 0.8 * scale,
        0,
        "butt",
      );
      drawWayRange(
        way,
        start,
        end,
        centre,
        "#121212",
        Math.max(0.1, width - 0.8 * scale),
        0,
        "butt",
      );
      if (way.structure === "bridge")
        for (const side of [-1, 1])
          drawWayRange(
            way,
            start,
            end,
            centre,
            "#9a9a9a",
            1.2 * scale,
            side * ((way.lanes * way.lane_width) / 2 + 2),
            "butt",
          );
      if (way.lane_width * scale >= 1.2) {
        for (let boundary = 1; boundary < way.lanes; boundary++) {
          ctx.setLineDash([Math.max(2, 5 * scale), Math.max(2, 5 * scale)]);
          drawWayRange(
            way,
            start,
            end,
            centre,
            "rgba(218,218,218,.35)",
            Math.max(0.45, 0.65 * scale),
            (boundary - way.lanes / 2) * way.lane_width,
          );
          ctx.setLineDash([]);
        }
      }
    });
  }
  function drawIntersectionPads() {
    const scale = currentTransform().scale,
      radius = state.city.simulation.intersection_radius || 22;
    state.city.intersections.forEach((intersection) => {
      if (!nodeVisible(intersection)) return;
      if (intersection.traffic_control.mode === "flow") {
        const ways = Object.fromEntries(state.city.ways.map((w) => [w.id, w]));
        for (const m of intersection.traffic_control.phases[0]
          .allowed_movements || []) {
          const a = ways[m.from_way_id],
            b = ways[m.to_way_id];
          if (!a || !b) continue;
          for (const c of m.lane_connections || []) {
            const start = sampleWay(
                a,
                Math.max(0, wayLength(a) - radius),
                c.from_lane,
              ),
              end = sampleWay(b, Math.min(radius, wayLength(b)), c.to_lane),
              p = screen(start),
              q = screen(end),
              h =
                Math.min(
                  radius * 0.9,
                  Math.hypot(start.x - end.x, start.y - end.y) * 0.45,
                ) * scale;
            ctx.beginPath();
            ctx.moveTo(p.x, p.y);
            ctx.bezierCurveTo(
              p.x + start.tx * h,
              p.y + start.ty * h,
              q.x - end.tx * h,
              q.y - end.ty * h,
              q.x,
              q.y,
            );
            ctx.strokeStyle = "#121212";
            ctx.lineWidth = Math.min(a.lane_width, b.lane_width) * scale + 0.5;
            ctx.lineCap = "butt";
            ctx.stroke();
          }
        }
        return;
      }
      const incoming = state.city.ways.filter(
          (way) => way.to === intersection.id,
        ),
        outgoing = state.city.ways.filter(
          (way) => way.from === intersection.id,
        ),
        corners = [];
      [
        ...incoming.map((way) => ({
          way,
          along: Math.max(0, wayLength(way) - radius),
        })),
        ...outgoing.map((way) => ({
          way,
          along: Math.min(radius, wayLength(way)),
        })),
      ].forEach(({ way, along }) => {
        const sample = sampleWay(way, along, (way.lanes - 1) / 2),
          half = (way.lanes * way.lane_width) / 2 + 2;
        corners.push(
          { x: sample.x - sample.ty * half, y: sample.y + sample.tx * half },
          { x: sample.x + sample.ty * half, y: sample.y - sample.tx * half },
        );
      });
      // Use the mouth edges as anchors, then blend the curb between approaches.
      corners.sort(
        (a, b) =>
          Math.atan2(
            a.y - intersection.position.y,
            a.x - intersection.position.x,
          ) -
          Math.atan2(
            b.y - intersection.position.y,
            b.x - intersection.position.x,
          ),
      );
      const edges = [];
      [...incoming, ...outgoing].forEach((way) => {
        const inbound = way.to === intersection.id;
        const sample = sampleWay(
          way,
          inbound
            ? Math.max(0, wayLength(way) - radius)
            : Math.min(radius, wayLength(way)),
          (way.lanes - 1) / 2,
        );
        const half = (way.lanes * way.lane_width) / 2;
        for (const side of [-1, 1])
          edges.push({
            x: sample.x - side * sample.ty * half,
            y: sample.y + side * sample.tx * half,
            dx: sample.tx * (inbound ? 1 : -1),
            dy: sample.ty * (inbound ? 1 : -1),
            way: way.id,
          });
      });
      edges.sort(
        (a, b) =>
          Math.atan2(
            a.y - intersection.position.y,
            a.x - intersection.position.x,
          ) -
          Math.atan2(
            b.y - intersection.position.y,
            b.x - intersection.position.x,
          ),
      );
      if (!edges.length) return;
      const traceEdge = (a, b) => {
        const p = screen(a),
          q = screen(b);
        if (a.way === b.way) {
          ctx.lineTo(q.x, q.y);
          return;
        }
        const handle =
          Math.min(radius * 0.65, Math.hypot(a.x - b.x, a.y - b.y) * 0.35) *
          scale;
        ctx.bezierCurveTo(
          p.x + a.dx * handle,
          p.y + a.dy * handle,
          q.x + b.dx * handle,
          q.y + b.dy * handle,
          q.x,
          q.y,
        );
      };
      ctx.fillStyle = "#121212";
      ctx.strokeStyle = "rgba(218,218,218,.3)";
      ctx.lineWidth = 0.8 * scale;
      ctx.lineCap = "butt";
      ctx.lineJoin = "round";
      ctx.beginPath();
      const first = screen(edges[0]);
      ctx.moveTo(first.x, first.y);
      edges.forEach((edge, i) =>
        traceEdge(edge, edges[(i + 1) % edges.length]),
      );
      ctx.closePath();
      ctx.fill();
      // Mouths stay open: only draw curbs between different approaches.
      edges.forEach((edge, i) => {
        const next = edges[(i + 1) % edges.length];
        if (edge.way === next.way) return;
        ctx.beginPath();
        const p = screen(edge);
        ctx.moveTo(p.x, p.y);
        traceEdge(edge, next);
        ctx.stroke();
      });
    });
  }
  function buildLaneDirections(city) {
    const ways = Object.fromEntries(city.ways.map((way) => [way.id, way]));
    const assignments = new Map();
    city.intersections.forEach((intersection) => {
      (intersection.traffic_control?.phases || []).forEach((phase) => {
        (phase.allowed_movements || []).forEach((movement) => {
          const incoming = ways[movement.from_way_id],
            outgoing = ways[movement.to_way_id];
          if (!incoming || !outgoing) return;
          const fromLanes = movement.from_lanes?.length
            ? movement.from_lanes
            : [
                ...new Set(
                  (movement.lane_connections || []).map(
                    (connection) => connection.from_lane,
                  ),
                ),
              ];
          const incomingTangent = sampleWay(incoming, wayLength(incoming), 0);
          const outgoingTangent = sampleWay(outgoing, 0, 0);
          const dot =
            incomingTangent.tx * outgoingTangent.tx +
            incomingTangent.ty * outgoingTangent.ty;
          const cross =
            incomingTangent.tx * outgoingTangent.ty -
            incomingTangent.ty * outgoingTangent.tx;
          const direction = {
            toWayID: outgoing.id,
            turn: movement.turn,
            angle: Math.atan2(cross, dot),
          };
          if (!assignments.has(incoming.id))
            assignments.set(incoming.id, new Map());
          const byLane = assignments.get(incoming.id);
          fromLanes.forEach((lane) => {
            if (!byLane.has(lane)) byLane.set(lane, new Map());
            byLane.get(lane).set(outgoing.id, direction);
          });
        });
      });
    });
    assignments.forEach((byLane) =>
      byLane.forEach((targets, lane) => {
        byLane.set(
          lane,
          [...targets.values()].sort(
            (a, b) => a.angle - b.angle || a.toWayID.localeCompare(b.toWayID),
          ),
        );
      }),
    );
    return assignments;
  }
  function drawLaneDirectionMarkings() {
    if (!state.laneDirections.size) return;
    const radius = state.city.simulation.intersection_radius || 22;
    state.city.ways.forEach((way) => {
      if (!wayVisible(way)) return;
      const byLane = state.laneDirections.get(way.id);
      if (!byLane) return;
      const stop = Math.max(0, wayLength(way) - radius);
      for (let lane = 0; lane < way.lanes; lane++) {
        const directions = byLane.get(lane);
        if (!directions?.length) continue;
        const order = { left: 0, through: 1, right: 2, u_turn: 3 };
        const turns = [
          ...new Set(
            directions.map((direction) => direction.turn || "through"),
          ),
        ].sort((a, b) => (order[a] ?? 4) - (order[b] ?? 4));
        // GB 5768.3 limits a combined arrow to two directions. Extremely
        // direction-scarce simulated lanes therefore use adjacent standard
        // groups rather than an invented many-branched symbol.
        const groups = [];
        for (let index = 0; index < turns.length; index += 2)
          groups.push(turns.slice(index, index + 2));
        const metreScale = way.lane_width / 3.5;
        const nominalMetres =
          way.speed_limit * 3.6 >= 100
            ? 9
            : way.speed_limit * 3.6 > 60
              ? 6
              : way.speed_limit * 3.6 > 40
                ? 4.5
                : 3;
        const arrowLength = Math.max(6, nominalMetres * metreScale);
        const stopGap = 3 * metreScale;
        const groupSpan =
          groups.length * arrowLength +
          Math.max(0, groups.length - 1) * stopGap;
        const repeatSpacing = Math.max(
          30 * metreScale,
          groupSpan + arrowLength,
        );
        for (let repetition = 0; repetition < 2; repetition++) {
          for (let group = 0; group < groups.length; group++) {
            const tipAlong =
              stop -
              stopGap -
              repetition * repeatSpacing -
              group * (arrowLength + stopGap);
            const along = tipAlong - arrowLength / 2;
            if (along - arrowLength / 2 < 0) continue;
            drawLaneDirectionArrow(
              way,
              lane,
              along,
              groups[group],
              arrowLength,
            );
          }
        }
      }
    });
  }
  function drawLaneDirectionArrow(way, lane, along, turns, arrowLength) {
    const interval = wayInterval(way);
    if (
      !interval ||
      along < interval[0] * wayLength(way) ||
      along > interval[1] * wayLength(way)
    )
      return;
    const sample = sampleWay(way, along, lane),
      position = screen(sample),
      scale = currentTransform().scale;
    const length = arrowLength * scale,
      laneWidth = way.lane_width * scale;
    if (laneWidth < 3) return;
    const shaft = Math.min(laneWidth * 0.1, length * 0.045);
    const head = Math.min(laneWidth * 0.16, length * 0.1);
    ctx.save();
    ctx.translate(position.x, position.y);
    ctx.rotate(Math.atan2(sample.ty, sample.tx));
    ctx.fillStyle = "#dedede";
    ctx.strokeStyle = "#dedede";
    ctx.lineWidth = shaft;
    ctx.lineCap = "butt";
    ctx.lineJoin = "round";
    const arrowhead = (x, y, angle) => {
      ctx.save();
      ctx.translate(x, y);
      ctx.rotate(angle);
      ctx.beginPath();
      ctx.moveTo(0, 0);
      ctx.lineTo(-head * 1.8, -head);
      ctx.lineTo(-head * 1.8, head);
      ctx.closePath();
      ctx.fill();
      ctx.restore();
    };
    ctx.beginPath();
    ctx.moveTo(-length * 0.48, 0);
    ctx.lineTo(turns.includes("through") ? length * 0.46 - head : 0, 0);
    ctx.stroke();
    if (turns.includes("through")) arrowhead(length * 0.46, 0, 0);
    for (const [turn, sign] of [
      ["left", -1],
      ["right", 1],
    ] as const)
      if (turns.includes(turn)) {
        const tipY = sign * laneWidth * 0.36,
          x = length * 0.12;
        ctx.beginPath();
        ctx.moveTo(-length * 0.12, 0);
        ctx.quadraticCurveTo(x, 0, x, sign * laneWidth * 0.14);
        ctx.lineTo(x, tipY - sign * head);
        ctx.stroke();
        arrowhead(x, tipY, (sign * Math.PI) / 2);
      }
    if (turns.includes("u_turn")) {
      const sign = state.city.driving_side === "left" ? 1 : -1,
        y = sign * laneWidth * 0.3;
      ctx.beginPath();
      ctx.moveTo(-length * 0.12, 0);
      ctx.bezierCurveTo(length * 0.3, 0, length * 0.3, y, -length * 0.12, y);
      ctx.stroke();
      arrowhead(-length * 0.12 - head, y, Math.PI);
    }
    ctx.restore();
  }
  function drawSelectedTrip() {
    if (state.selected?.type !== "person" || !state.snapshot) return;
    const person = state.snapshot.people.find(
      (item) => item.id === state.selected.id,
    );
    if (!person) return;
    const ways = Object.fromEntries(
      state.city.ways.map((way) => [way.id, way]),
    );
    person.route.forEach((wayID, index) => {
      const way = ways[wayID];
      if (!way || !wayVisible(way)) return;
      ctx.strokeStyle =
        index < person.route_index
          ? "rgba(239,200,91,.20)"
          : "rgba(255,255,255,.32)";
      drawWayRange(
        way,
        0,
        wayLength(way),
        (way.lanes - 1) / 2,
        ctx.strokeStyle,
        2.5,
      );
    });
    if (!levelVisible(personLevel(person))) return;
    const origin = screen(person.origin),
      current = screen(person.position),
      destination = screen(person.destination);
    ctx.setLineDash([3, 4]);
    ctx.lineWidth = 1.2 * currentTransform().scale;
    ctx.strokeStyle = "rgba(239,200,91,.72)";
    ctx.beginPath();
    ctx.moveTo(origin.x, origin.y);
    ctx.lineTo(current.x, current.y);
    ctx.stroke();
    ctx.strokeStyle = "rgba(255,120,95,.78)";
    ctx.beginPath();
    ctx.moveTo(current.x, current.y);
    ctx.lineTo(destination.x, destination.y);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = "#efc85b";
    ctx.beginPath();
    ctx.arc(origin.x, origin.y, 5, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = "#ff785f";
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.arc(destination.x, destination.y, 6, 0, Math.PI * 2);
    ctx.stroke();
  }
  function drawSignals() {
    if (!state.snapshot?.signals) return;
    const ways = Object.fromEntries(
      state.city.ways.map((way) => [way.id, way]),
    );
    state.snapshot.signals.forEach((signal) => {
      const node = state.city.intersections.find(
        (n) => n.id === signal.intersection_id,
      );
      if (!node || !nodeVisible(node) || node.traffic_control.mode === "flow")
        return;
      const greenLanes = new Map();
      (signal.allowed_movements || []).forEach((movement) => {
        const incoming = ways[movement.from_way_id],
          outgoing = ways[movement.to_way_id];
        if (!incoming || !outgoing) return;
        const inLength = wayLength(incoming),
          outLength = wayLength(outgoing),
          radius = state.city.simulation.intersection_radius || 22,
          scale = currentTransform().scale;
        const inStop = Math.max(0, inLength - radius),
          outStart = Math.min(radius, outLength);
        const fromLanes = movement.from_lanes?.length
            ? movement.from_lanes
            : [0],
          toLanes = movement.to_lanes?.length ? movement.to_lanes : [0];
        const active = greenLanes.get(incoming.id) || new Set();
        fromLanes.forEach((lane) => active.add(lane));
        greenLanes.set(incoming.id, active);
        const count = Math.max(fromLanes.length, toLanes.length),
          fallback = Array.from({ length: count }, (_, lane) => ({
            from_lane: fromLanes[Math.min(lane, fromLanes.length - 1)],
            to_lane: toLanes[Math.min(lane, toLanes.length - 1)],
          })),
          connections = movement.lane_connections?.length
            ? movement.lane_connections
            : fallback;
        for (const connection of connections) {
          const start = sampleWay(incoming, inStop, connection.from_lane),
            end = sampleWay(outgoing, outStart, connection.to_lane),
            a = screen(start),
            b = screen(end),
            handle =
              Math.min(
                radius * 0.9,
                Math.hypot(end.x - start.x, end.y - start.y) * 0.45,
              ) * scale;
          ctx.strokeStyle = "rgba(200,242,107,.82)";
          ctx.lineWidth = 1.45 * scale;
          ctx.beginPath();
          ctx.moveTo(a.x, a.y);
          ctx.bezierCurveTo(
            a.x + start.tx * handle,
            a.y + start.ty * handle,
            b.x - end.tx * handle,
            b.y - end.ty * handle,
            b.x,
            b.y,
          );
          ctx.stroke();
        }
      });
      greenLanes.forEach((lanes, wayID) => {
        const way = ways[wayID],
          scale = currentTransform().scale,
          radius = state.city.simulation.intersection_radius || 22,
          stop = Math.max(0, wayLength(way) - radius);
        lanes.forEach((lane) =>
          drawWayRange(
            way,
            Math.max(0, stop - 58),
            stop,
            lane,
            "rgba(200,242,107,.86)",
            2.25 * scale,
          ),
        );
      });
      Object.values(ways)
        .filter((way) => way.to === signal.intersection_id)
        .forEach((way) => {
          const scale = currentTransform().scale,
            radius = state.city.simulation.intersection_radius || 22,
            stop = Math.max(0, wayLength(way) - radius);
          const sample = sampleWay(way, stop, (way.lanes - 1) / 2),
            p = screen(sample),
            green = (signal.allowed_incoming_way_ids || []).includes(way.id);
          ctx.save();
          ctx.translate(p.x, p.y);
          ctx.rotate(Math.atan2(sample.ty, sample.tx));
          ctx.strokeStyle = "rgba(238,244,240,.82)";
          ctx.lineWidth = 1.4 * scale;
          ctx.beginPath();
          ctx.moveTo(0, -way.lanes * way.lane_width * 0.5 * scale);
          ctx.lineTo(0, way.lanes * way.lane_width * 0.5 * scale);
          ctx.stroke();
          ctx.fillStyle =
            signal.stage === "amber"
              ? "#efc85b"
              : green
                ? "#c8f26b"
                : "#ff785f";
          ctx.beginPath();
          ctx.arc(-2.2 * scale, 0, 1.8 * scale, 0, Math.PI * 2);
          ctx.fill();
          ctx.restore();
        });
    });
  }
  function drawIntersections() {
    const selectedID =
      state.selected?.type === "intersection" ? state.selected.id : null;
    const denseOverview =
      state.city.intersections.length > 40 && state.camera.zoom < 4.5;
    state.city.intersections.forEach((intersection) => {
      if (!nodeVisible(intersection)) return;
      const p = screen(intersection.position);
      if (
        selectedID === intersection.id ||
        (!denseOverview && state.camera.zoom > 0.36)
      ) {
        ctx.fillStyle = "rgba(235,238,228,.66)";
        ctx.font = "9px ui-monospace,monospace";
        ctx.fillText(
          `${intersection.id}  ${intersection.name.toUpperCase()}`,
          p.x + 13,
          p.y - 8,
        );
      }
    });
  }
  function drawTransforms() {
    state.city.intersections.forEach((intersection) =>
      intersection.approaches.forEach((approach) =>
        approach.sub_segments.forEach((segment) => {
          const matrix = segment.transform,
            origin = screen({ x: matrix[0][2], y: matrix[1][2] }),
            length = 12;
          ctx.lineWidth = 1;
          ctx.strokeStyle = "rgba(255,120,95,.72)";
          ctx.beginPath();
          ctx.moveTo(origin.x, origin.y);
          ctx.lineTo(
            origin.x + matrix[0][0] * length,
            origin.y + matrix[1][0] * length,
          );
          ctx.stroke();
          ctx.strokeStyle = "rgba(200,242,107,.72)";
          ctx.beginPath();
          ctx.moveTo(origin.x, origin.y);
          ctx.lineTo(
            origin.x + matrix[0][1] * length,
            origin.y + matrix[1][1] * length,
          );
          ctx.stroke();
        }),
      ),
    );
  }
  function drawTaxiStops() {
    const taxi = state.snapshot?.taxi;
    if (!taxi || renderStructure !== "ground" || !levelVisible(0)) return;
    for (const stop of taxi.stops) {
      const p = screen(stop.position),
        order = taxi.orders.find(
          (o) =>
            o.pickup === stop.id &&
            ["Waiting", "Assigned", "Boarding"].includes(o.state),
        );
      ctx.save();
      ctx.strokeStyle = order ? "#efc85b" : "#7196a1";
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.arc(p.x, p.y, 1.2 * currentTransform().scale, 0, Math.PI * 2);
      ctx.stroke();
      if (order) {
        ctx.fillStyle = "#efc85b";
        ctx.font = "12px Times New Roman";
        ctx.fillText(`#${order.id}`, p.x + 8, p.y - 5);
      }
      ctx.restore();
    }
  }
  function drawPeople() {
    if (!state.snapshot) return;
    const scale = currentTransform().scale,
      ways = Object.fromEntries(state.city.ways.map((way) => [way.id, way]));
    state.snapshot.people.forEach((person) => {
      const structure = person.structure || "ground";
      if (
        (renderStructure && structure !== renderStructure) ||
        !levelVisible(personLevel(person))
      )
        return;
      const p = screen(person.position),
        selected =
          state.selected?.type === "person" && state.selected.id === person.id;
      ctx.save();
      ctx.translate(p.x, p.y);
      ctx.rotate((person.heading_degrees * Math.PI) / 180);
      if (person.state.startsWith("walking") || person.state === "arrived") {
        ctx.fillStyle = person.state === "arrived" ? "#ffffff" : "#efc85b";
        ctx.shadowColor = ctx.fillStyle;
        ctx.shadowBlur = 4 * scale;
        ctx.beginPath();
        ctx.arc(0, 0, 2 * scale, 0, Math.PI * 2);
        ctx.fill();
        ctx.shadowBlur = 0;
      } else {
        const way = ways[person.way_id],
          length = (state.city.simulation.vehicle_size || 6) * scale,
          width = 1.8 * scale;
        const taxi = state.snapshot.taxi?.vehicles.find(
          (v) => v.id === person.id,
        );
        ctx.fillStyle = taxi
          ? {
              Idle: "#a8b0b5",
              ToPickup: "#efc85b",
              Boarding: "#efc85b",
              WithPassenger: "#69d6ef",
              Alighting: "#69d6ef",
            }[taxi.state] || "#eee"
          : person.state === "waiting"
            ? "#ff785f"
            : "#c8f26b";
        ctx.shadowColor = ctx.fillStyle;
        ctx.shadowBlur = 4 * scale;
        ctx.fillRect(-length / 2, -width / 2, length, width);
        ctx.shadowBlur = 0;
      }
      if (selected) {
        ctx.strokeStyle = "#fff";
        ctx.lineWidth = 1.2 * scale;
        ctx.beginPath();
        ctx.arc(0, 0, 8 * scale, 0, Math.PI * 2);
        ctx.stroke();
      }
      ctx.restore();
    });
  }
  function drawJunctionCopy(copy: HTMLCanvasElement, intersection, phase) {
    const original = state;
    state = { ...state };
    const saved = {
      ctx,
      renderStructure,
      layerFilter: state.layerFilter,
      city: state.city,
      snapshot: state.snapshot,
      transform: state.transform,
      camera: state.camera,
    };
    try {
      ctx = copy.getContext("2d");
      renderStructure = null;
      state.layerFilter = "all";
      const ways = saved.city.ways.filter(
        (w) => w.from === intersection.id || w.to === intersection.id,
      );
      const radius = saved.city.simulation.intersection_radius || 22;
      // Fit the junction mouth geometry, rather than long approach roads.
      const corners = [];
      ways.forEach((way) => {
        const along =
          way.to === intersection.id
            ? Math.max(0, wayLength(way) - radius)
            : Math.min(radius, wayLength(way));
        const point = sampleWay(way, along, (way.lanes - 1) / 2);
        const half = (way.lanes * way.lane_width) / 2 + 2;
        for (const side of [-1, 1])
          corners.push({
            x: point.x - side * point.ty * half,
            y: point.y + side * point.tx * half,
          });
      });
      const minX = Math.min(...corners.map((p) => p.x)),
        maxX = Math.max(...corners.map((p) => p.x));
      const minY = Math.min(...corners.map((p) => p.y)),
        maxY = Math.max(...corners.map((p) => p.y));
      const margin = Math.max(4, radius * 0.18);
      const scale = Math.min(
        copy.width / (maxX - minX + margin * 2),
        copy.height / (maxY - minY + margin * 2),
      );
      state.transform = {
        baseScale: scale,
        baseOx: copy.width / 2 - ((minX + maxX) / 2) * scale,
        baseOy: copy.height / 2 - ((minY + maxY) / 2) * scale,
        width: copy.width,
        height: copy.height,
      };
      state.camera = { zoom: 1, panX: 0, panY: 0 };
      state.city = { ...saved.city, ways, intersections: [intersection] };
      state.snapshot = {
        ...state.snapshot,
        signals: [
          {
            intersection_id: intersection.id,
            stage: "green",
            allowed_movements: phase.allowed_movements || [],
            allowed_incoming_way_ids: [
              ...new Set(
                (phase.allowed_movements || []).map((m) => m.from_way_id),
              ),
            ],
          },
        ],
      } as Snapshot;
      ctx.fillStyle = "#080808";
      ctx.fillRect(0, 0, copy.width, copy.height);
      drawWays();
      drawIntersectionPads();
      drawLaneDirectionMarkings();
      drawSignals();
      ctx.fillStyle = "#e6ecdf";
      ctx.font = "16px sans-serif";
      ctx.fillText(intersection.id, 12, 22);
    } finally {
      ctx = saved.ctx;
      renderStructure = saved.renderStructure;
      state.layerFilter = saved.layerFilter;
      state.city = saved.city;
      state.snapshot = saved.snapshot;
      state.transform = saved.transform;
      state.camera = saved.camera;
      state = original;
    }
  }
  function drawInterchange(
    copy: HTMLCanvasElement,
    id: string,
    routeIndex: number,
  ) {
    const original = state;
    state = { ...state };
    const group = state.city.interchanges.find((g) => g.id === id);
    if (!group) {
      state = original;
      return;
    }
    const route = group.routes[routeIndex || 0],
      ids = new Set(route.way_ids);
    const nodes = state.city.intersections.filter((n) => n.interchange === id),
      nodeIDs = new Set(nodes.map((n) => n.id));
    const ways = state.city.ways.filter(
      (w) => nodeIDs.has(w.from) && nodeIDs.has(w.to),
    );
    const saved = {
      ctx,
      city: state.city,
      snapshot: state.snapshot,
      transform: state.transform,
      camera: state.camera,
      renderStructure,
      layerFilter: state.layerFilter,
    };
    try {
      ctx = copy.getContext("2d");
      state.city = { ...state.city, ways, intersections: nodes };
      state.snapshot = null;
      state.layerFilter = "all";
      const points = ways.flatMap((w) => w.points),
        minX = Math.min(...points.map((p) => p.x)),
        maxX = Math.max(...points.map((p) => p.x)),
        minY = Math.min(...points.map((p) => p.y)),
        maxY = Math.max(...points.map((p) => p.y));
      const scale = Math.min(
        copy.width / (maxX - minX + 100),
        copy.height / (maxY - minY + 100),
      );
      state.transform = {
        baseScale: scale,
        baseOx: copy.width / 2 - ((minX + maxX) * scale) / 2,
        baseOy: copy.height / 2 - ((minY + maxY) * scale) / 2,
        width: copy.width,
        height: copy.height,
      };
      state.camera = { zoom: 1, panX: 0, panY: 0 };
      ctx.clearRect(0, 0, copy.width, copy.height);
      for (const layer of ["tunnel", "ground", "ramp", "bridge"]) {
        renderStructure = layer;
        drawWays();
        drawIntersectionPads();
        for (const w of ways) {
          if (ids.has(w.id) && wayVisible(w)) {
            drawWayRange(w, 0, wayLength(w), (w.lanes - 1) / 2, "#c8f26b", 3);
            for (let d = 80; d < wayLength(w); d += 180) {
              const v = sampleWay(w, d, (w.lanes - 1) / 2),
                p = screen(v);
              ctx.save();
              ctx.translate(p.x, p.y);
              ctx.rotate(Math.atan2(v.ty, v.tx));
              ctx.fillStyle = "#e7ffb1";
              ctx.beginPath();
              ctx.moveTo(5, 0);
              ctx.lineTo(-4, -3);
              ctx.lineTo(-4, 3);
              ctx.closePath();
              ctx.fill();
              ctx.restore();
            }
          }
        }
      }
    } finally {
      ctx = saved.ctx;
      state.city = saved.city;
      state.snapshot = saved.snapshot;
      state.transform = saved.transform;
      state.camera = saved.camera;
      renderStructure = saved.renderStructure;
      state.layerFilter = saved.layerFilter;
      state = original;
    }
  }
  return {
    draw,
    fitCanvas,
    screen,
    zoomAt,
    resetView,
    fitMetroPassengerRoute,
    drawJunctionCopy,
    drawInterchange,
    nodeVisible,
    levelVisible,
    personLevel,
    updateCity() {
      state.laneDirections = buildLaneDirections(state.city);
      fitCanvas();
    },
  };
}
export type MapRenderer = ReturnType<typeof createRenderer>;
