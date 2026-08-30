"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { sampleWay, wayLength, type City, type Point, type Selection, type Snapshot, type Way } from "./simulation";

type ViewCommand = { id: number; type: "in" | "out" | "fit" } | null;
type Props = { city: City; snapshot: Snapshot; selected: Selection; onSelect: (selection: Selection) => void; transforms: boolean; viewCommand: ViewCommand; onZoom: (zoom: number) => void };
type ScreenTransform = { scale: number; ox: number; oy: number; width: number; height: number; baseScale: number; baseOx: number; baseOy: number };

function traceWay(context: CanvasRenderingContext2D, way: Way, t: ScreenTransform, from = 0, to = wayLength(way), lane = (way.lanes - 1) / 2, extraOffset = 0) {
  const distance = Math.max(0, to - from), steps = Math.max(2, Math.ceil(distance / 24));
  context.beginPath();
  for (let index = 0; index <= steps; index++) {
    const world = sampleWay(way, from + distance * index / steps, lane, extraOffset).position;
    const x = world.x * t.scale + t.ox, y = world.y * t.scale + t.oy;
    if (!index) context.moveTo(x, y); else context.lineTo(x, y);
  }
}

function screenPoint(point: Point, t: ScreenTransform) { return { x: point.x * t.scale + t.ox, y: point.y * t.scale + t.oy }; }

export default function TrafficCanvas({ city, snapshot, selected, onSelect, transforms, viewCommand, onZoom }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const camera = useRef({ zoom: 1, panX: 0, panY: 0 });
  const pointers = useRef(new Map<number, Point>());
  const moved = useRef(false);
  const [panning, setPanning] = useState(false);

  const baseTransform = useCallback(() => {
    const canvas = canvasRef.current, bounds = city.bounds;
    if (!canvas) return { baseScale: 1, baseOx: 0, baseOy: 0, width: 1, height: 1 };
    const rect = canvas.getBoundingClientRect(), padding = 38;
    const baseScale = Math.max(.001, Math.min((rect.width - padding * 2) / (bounds.max_x - bounds.min_x), (rect.height - padding * 2) / (bounds.max_y - bounds.min_y)));
    return { baseScale, baseOx: (rect.width - (bounds.max_x - bounds.min_x) * baseScale) / 2 - bounds.min_x * baseScale, baseOy: (rect.height - (bounds.max_y - bounds.min_y) * baseScale) / 2 - bounds.min_y * baseScale, width: rect.width, height: rect.height };
  }, [city]);

  const transform = useCallback((): ScreenTransform => {
    const base = baseTransform(), view = camera.current;
    return { ...base, scale: base.baseScale * view.zoom, ox: base.baseOx + view.panX, oy: base.baseOy + view.panY };
  }, [baseTransform]);

  const zoomAt = useCallback((factor: number, x?: number, y?: number) => {
    const old = transform(), anchorX = x ?? old.width / 2, anchorY = y ?? old.height / 2;
    const world = { x: (anchorX - old.ox) / old.scale, y: (anchorY - old.oy) / old.scale };
    camera.current.zoom = Math.max(.03, Math.min(64, camera.current.zoom * factor));
    const base = baseTransform(), scale = base.baseScale * camera.current.zoom;
    camera.current.panX = anchorX - base.baseOx - world.x * scale;
    camera.current.panY = anchorY - base.baseOy - world.y * scale;
    onZoom(camera.current.zoom);
  }, [baseTransform, onZoom, transform]);

  const draw = useCallback(() => {
    const canvas = canvasRef.current; if (!canvas) return;
    const context = canvas.getContext("2d"); if (!context) return;
    const rect = canvas.getBoundingClientRect(), ratio = Math.min(devicePixelRatio || 1, 2);
    const pixelWidth = Math.max(1, Math.round(rect.width * ratio)), pixelHeight = Math.max(1, Math.round(rect.height * ratio));
    if (canvas.width !== pixelWidth || canvas.height !== pixelHeight) { canvas.width = pixelWidth; canvas.height = pixelHeight; }
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
    context.clearRect(0, 0, rect.width, rect.height);
    context.fillStyle = "#07100e"; context.fillRect(0, 0, rect.width, rect.height);
    const t = transform(), ways = Object.fromEntries(city.ways.map((way) => [way.id, way])), radius = city.simulation?.intersection_radius || 22;
    const signalByIntersection = Object.fromEntries((snapshot.signals || []).map((signal) => [signal.intersection_id, signal]));

    const grid = Math.max(16, 36 * Math.min(3, camera.current.zoom));
    context.strokeStyle = "rgba(218,236,227,.045)"; context.lineWidth = 1;
    for (let x = ((t.ox % grid) + grid) % grid; x < rect.width; x += grid) { context.beginPath(); context.moveTo(x, 0); context.lineTo(x, rect.height); context.stroke(); }
    for (let y = ((t.oy % grid) + grid) % grid; y < rect.height; y += grid) { context.beginPath(); context.moveTo(0, y); context.lineTo(rect.width, y); context.stroke(); }

    context.lineCap = "round"; context.lineJoin = "round";
    city.ways.forEach((way) => {
      const width = way.lanes * way.lane_width * t.scale;
      const casing = { arterial: "rgba(224,234,228,.42)", collector: "rgba(218,229,222,.31)", ramp: "rgba(200,242,107,.25)", local: "rgba(218,229,222,.2)" }[way.road_class || "local"];
      const surface = { arterial: "#1c2925", collector: "#182520", ramp: "#19271f", local: "#14201d" }[way.road_class || "local"];
      context.strokeStyle = casing; context.lineWidth = width + (way.road_class === "arterial" ? 2.8 : 1.8) * t.scale; traceWay(context, way, t); context.stroke();
      context.strokeStyle = surface; context.lineWidth = width; traceWay(context, way, t); context.stroke();
    });

    city.ways.forEach((way) => {
      for (let boundary = 1; boundary < way.lanes; boundary++) {
        const extraOffset = (boundary - way.lanes / 2) * way.lane_width;
        context.setLineDash([5 * t.scale, 5 * t.scale]); context.strokeStyle = "rgba(218,229,222,.35)"; context.lineWidth = .65 * t.scale;
        traceWay(context, way, t, 0, wayLength(way), (way.lanes - 1) / 2, extraOffset); context.stroke();
      }
      context.setLineDash([]);
      const length = wayLength(way);
      for (let along = 55; along < length - 38; along += 86) {
        for (let lane = 0; lane < way.lanes; lane++) {
          const sample = sampleWay(way, along, lane), p = screenPoint(sample.position, t), angle = Math.atan2(sample.tangent.y, sample.tangent.x), size = Math.min(4.2, way.lane_width * .58) * t.scale;
          context.save(); context.translate(p.x, p.y); context.rotate(angle); context.strokeStyle = "rgba(224,235,229,.46)"; context.lineWidth = .8 * t.scale;
          context.beginPath(); context.moveTo(-size, -size * .55); context.lineTo(0, 0); context.lineTo(-size, size * .55); context.stroke(); context.restore();
        }
      }

      if (way.road_class === "ramp") {
        const merge = Math.max(0, length - radius * 2.3);
        for (let stripe = 0; stripe < 5; stripe++) {
          const along = merge + stripe * radius * .22, sample = sampleWay(way, along, way.lanes - 1), p = screenPoint(sample.position, t), angle = Math.atan2(sample.tangent.y, sample.tangent.x);
          context.save(); context.translate(p.x, p.y); context.rotate(angle - .55); context.strokeStyle = "rgba(200,242,107,.36)"; context.lineWidth = .7 * t.scale; context.beginPath(); context.moveTo(0, 0); context.lineTo(8 * t.scale, 0); context.stroke(); context.restore();
        }
      }
    });

    if (selected?.type === "person") {
      const person = snapshot.people.find((item) => item.id === selected.id);
      person?.route.forEach((wayID, index) => {
        const way = ways[wayID]; if (!way) return;
        context.strokeStyle = index < person.route_index ? "rgba(239,200,91,.25)" : "rgba(255,255,255,.52)"; context.lineWidth = 2.2 * t.scale;
        traceWay(context, way, t); context.stroke();
      });
      if (person) {
        const origin = screenPoint(person.origin, t), current = screenPoint(person.position, t), destination = screenPoint(person.destination, t);
        context.setLineDash([3 * t.scale, 4 * t.scale]); context.lineWidth = 1.2 * t.scale;
        context.strokeStyle = "rgba(239,200,91,.72)"; context.beginPath(); context.moveTo(origin.x, origin.y); context.lineTo(current.x, current.y); context.stroke();
        context.strokeStyle = "rgba(255,120,95,.78)"; context.beginPath(); context.moveTo(current.x, current.y); context.lineTo(destination.x, destination.y); context.stroke(); context.setLineDash([]);
        context.fillStyle = "#efc85b"; context.beginPath(); context.arc(origin.x, origin.y, 4 * t.scale, 0, Math.PI * 2); context.fill();
        context.strokeStyle = "#ff785f"; context.lineWidth = 1.8 * t.scale; context.beginPath(); context.arc(destination.x, destination.y, 5 * t.scale, 0, Math.PI * 2); context.stroke();
      }
    }

    city.intersections.forEach((intersection) => {
      const incoming = city.ways.filter((way) => way.to === intersection.id), outgoing = city.ways.filter((way) => way.from === intersection.id), corners: Point[] = [];
      [...incoming.map((way) => ({ way, along: Math.max(0, wayLength(way) - radius) })), ...outgoing.map((way) => ({ way, along: Math.min(radius, wayLength(way)) }))].forEach(({ way, along }) => {
        const sample = sampleWay(way, along, (way.lanes - 1) / 2), half = way.lanes * way.lane_width / 2 + 2;
        corners.push({ x: sample.position.x - sample.tangent.y * half, y: sample.position.y + sample.tangent.x * half }, { x: sample.position.x + sample.tangent.y * half, y: sample.position.y - sample.tangent.x * half });
      });
      corners.sort((a,b) => Math.atan2(a.y-intersection.position.y,a.x-intersection.position.x)-Math.atan2(b.y-intersection.position.y,b.x-intersection.position.x));
      context.fillStyle = "#111c19"; context.strokeStyle = "rgba(220,233,225,.3)"; context.lineWidth = .8 * t.scale;
      context.beginPath(); corners.forEach((corner,index) => { const p=screenPoint(corner,t); if(!index)context.moveTo(p.x,p.y);else context.lineTo(p.x,p.y); }); context.closePath(); context.fill(); context.stroke();
    });

    const drawMovement = (incoming: Way, outgoing: Way, color: string, width: number, dashed = false, fromLanes = Array.from({length:incoming.lanes},(_,i)=>i), toLanes = Array.from({length:outgoing.lanes},(_,i)=>i), laneConnections?: {from_lane:number;to_lane:number}[]) => {
      const count = Math.max(fromLanes.length, toLanes.length), fallback = Array.from({length:count},(_,index)=>({
        from_lane:fromLanes[Math.min(fromLanes.length-1,Math.floor(index*fromLanes.length/count))] ?? 0,
        to_lane:toLanes[Math.min(toLanes.length-1,Math.floor(index*toLanes.length/count))] ?? 0,
      })),connections=laneConnections?.length?laneConnections:fallback;
      for (const connection of connections) {
        const fromLane = connection.from_lane, toLane = connection.to_lane;
        const startSample=sampleWay(incoming,Math.max(0,wayLength(incoming)-radius),fromLane),endSample=sampleWay(outgoing,Math.min(radius,wayLength(outgoing)),toLane),start=screenPoint(startSample.position,t),end=screenPoint(endSample.position,t),handle=radius*.82*t.scale;
        context.setLineDash(dashed ? [3 * t.scale, 4 * t.scale] : []); context.strokeStyle = color; context.lineWidth = width * t.scale;
        context.beginPath(); context.moveTo(start.x,start.y); context.bezierCurveTo(start.x+startSample.tangent.x*handle,start.y+startSample.tangent.y*handle,end.x-endSample.tangent.x*handle,end.y-endSample.tangent.y*handle,end.x,end.y); context.stroke();
      }
      context.setLineDash([]);
    };

    const selectedIntersection = selected?.type === "intersection" ? String(selected.id) : "";
    if (selectedIntersection || camera.current.zoom > .85) {
      city.intersections.forEach((intersection) => {
        if (selectedIntersection && intersection.id !== selectedIntersection) return;
        city.ways.filter((way) => way.to === intersection.id).forEach((incoming) => city.ways.filter((way) => way.from === intersection.id).forEach((outgoing) => drawMovement(incoming, outgoing, "rgba(205,222,214,.16)", .7, true)));
      });
    }
    (snapshot.signals || []).forEach((signal) => {
      const intersection = city.intersections.find((item) => item.id === signal.intersection_id); if (!intersection) return;
      (signal.allowed_movements || []).forEach((movement) => {
        const incoming = ways[movement.from_way_id], outgoing = ways[movement.to_way_id];
        if (incoming && outgoing) drawMovement(incoming, outgoing, "rgba(200,242,107,.88)", 1.65, false, movement.from_lanes || [], movement.to_lanes || [], movement.lane_connections || []);
      });
    });

    const greenLanesByWay = new Map<string,Set<number>>();
    (snapshot.signals || []).forEach((signal)=>(signal.allowed_movements || []).forEach((movement)=>{
      const lanes=greenLanesByWay.get(movement.from_way_id) || new Set<number>();
      (movement.from_lanes || []).forEach((lane)=>lanes.add(lane));greenLanesByWay.set(movement.from_way_id,lanes);
    }));

    city.ways.forEach((way) => {
      const signal = signalByIntersection[way.to], greenLanes = greenLanesByWay.get(way.id) || new Set<number>(), green = greenLanes.size > 0, length = wayLength(way), stop = Math.max(0, length - radius);
      if (green) {
        greenLanes.forEach((lane) => { context.strokeStyle = "rgba(200,242,107,.9)"; context.lineWidth = 2.25 * t.scale; traceWay(context, way, t, Math.max(0, stop - 58), stop, lane); context.stroke(); });
      }
      const centre = sampleWay(way, stop, (way.lanes - 1) / 2), p = screenPoint(centre.position, t), angle = Math.atan2(centre.tangent.y, centre.tangent.x);
      context.save(); context.translate(p.x, p.y); context.rotate(angle); context.strokeStyle = "rgba(238,244,240,.82)"; context.lineWidth = 1.4 * t.scale;
      context.beginPath(); context.moveTo(0, -way.lanes * way.lane_width * .5 * t.scale); context.lineTo(0, way.lanes * way.lane_width * .5 * t.scale); context.stroke();
      const lamp = signal?.stage === "amber" ? "#efc85b" : green ? "#c8f26b" : "#ff785f"; context.fillStyle = lamp; context.beginPath(); context.arc(-2.2 * t.scale, 0, 1.8 * t.scale, 0, Math.PI * 2); context.fill(); context.restore();

      const turns = city.intersections.find((item)=>item.id===way.to)?.traffic_control.phases.flatMap((phase)=>phase.allowed_movements || []).filter((movement)=>movement.from_way_id===way.id) || [];
      for(let lane=0;lane<way.lanes;lane++){
        const available=[...new Set(turns.filter((movement)=>movement.from_lanes?.includes(lane)).map((movement)=>movement.turn))], glyph=available.map((turn)=>turn==="left"?"↰":turn==="right"?"↱":turn==="u_turn"?"↶":"↑").join("");
        if(!glyph)continue;const marker=sampleWay(way,Math.max(0,stop-34),lane),mp=screenPoint(marker.position,t);context.save();context.translate(mp.x,mp.y);context.rotate(Math.atan2(marker.tangent.y,marker.tangent.x)+Math.PI/2);context.fillStyle="rgba(235,242,237,.62)";context.font=`${Math.max(7,8*t.scale)}px sans-serif`;context.textAlign="center";context.fillText(glyph,0,0);context.restore();
      }
    });

    city.intersections.forEach((intersection) => {
      const p = screenPoint(intersection.position, t), active = selectedIntersection === intersection.id, signal = signalByIntersection[intersection.id];
      const marker=(active?5:3)*t.scale;context.fillStyle=active?"#f6fbf7":"#dfe8e2";context.fillRect(p.x-marker,p.y-marker,marker*2,marker*2);
      context.strokeStyle=signal?.stage==="amber"?"rgba(239,200,91,.8)":signal?.stage==="green"?"rgba(200,242,107,.72)":"rgba(255,120,95,.55)";context.lineWidth=(active?1.5:.7)*t.scale;context.strokeRect(p.x-marker*2,p.y-marker*2,marker*4,marker*4);
      if (camera.current.zoom > .42) { context.fillStyle = "rgba(235,242,237,.72)"; context.font = `${Math.max(8, 8.5 * t.scale)}px ui-monospace,monospace`; context.fillText(`${intersection.id} · ${intersection.name.toUpperCase()}`, p.x + 10 * t.scale, p.y - 8 * t.scale); }
      if (transforms) { context.strokeStyle = "#ff785f"; context.lineWidth = t.scale; context.beginPath(); context.moveTo(p.x, p.y); context.lineTo(p.x + 15 * t.scale, p.y); context.stroke(); context.strokeStyle = "#c8f26b"; context.beginPath(); context.moveTo(p.x, p.y); context.lineTo(p.x, p.y - 15 * t.scale); context.stroke(); }
    });

    snapshot.people.forEach((person) => {
      const p = screenPoint(person.position, t), active = selected?.type === "person" && selected.id === person.id, way = person.way_id ? ways[person.way_id] : undefined;
      context.save(); context.translate(p.x, p.y); context.rotate(person.heading_degrees * Math.PI / 180);
      context.fillStyle = person.state.startsWith("walking") ? "#efc85b" : person.state === "arrived" ? "#f6fbf7" : person.state === "waiting" ? "#ff785f" : "#c8f26b";
      context.shadowColor = context.fillStyle; context.shadowBlur = 4 * t.scale;
      if (person.state.startsWith("walking") || person.state === "arrived") {
        context.beginPath(); context.arc(0, 0, 4 * t.scale, 0, Math.PI * 2); context.fill(); context.shadowBlur = 0; context.strokeStyle = "rgba(7,16,14,.9)"; context.lineWidth = .8 * t.scale; context.stroke();
      } else {
        const vehicleLength = (city.simulation?.vehicle_size || 12) * t.scale, vehicleWidth = Math.min(6.5, (way?.lane_width || 7) * .72) * t.scale;
        context.fillRect(-vehicleLength / 2, -vehicleWidth / 2, vehicleLength, vehicleWidth); context.shadowBlur = 0; context.strokeStyle = "rgba(6,14,12,.92)"; context.lineWidth = .7 * t.scale; context.strokeRect(-vehicleLength / 2, -vehicleWidth / 2, vehicleLength, vehicleWidth);
        context.fillStyle = "rgba(7,16,14,.58)"; context.fillRect(-vehicleLength * .08, -vehicleWidth * .38, vehicleLength * .32, vehicleWidth * .76);
      }
      if (active) { context.strokeStyle = "#ffffff"; context.lineWidth = 1.2 * t.scale; context.beginPath(); context.arc(0, 0, 8 * t.scale, 0, Math.PI * 2); context.stroke(); }
      context.restore();
    });
  }, [city, selected, snapshot, transform, transforms]);

  useEffect(() => {
    draw(); let frame = 0;
    const observer = new ResizeObserver(() => { cancelAnimationFrame(frame); frame = requestAnimationFrame(draw); });
    if (canvasRef.current?.parentElement) observer.observe(canvasRef.current.parentElement);
    return () => { observer.disconnect(); cancelAnimationFrame(frame); };
  }, [draw]);
  useEffect(() => {
    if (!viewCommand) return;
    if (viewCommand.type === "fit") { camera.current = { zoom: 1, panX: 0, panY: 0 }; onZoom(1); }
    else zoomAt(viewCommand.type === "in" ? 1.5 : 1 / 1.5);
    draw();
  }, [draw, onZoom, viewCommand, zoomAt]);

  const canvasPoint = (event: React.PointerEvent<HTMLCanvasElement>) => { const rect = event.currentTarget.getBoundingClientRect(); return { x: event.clientX - rect.left, y: event.clientY - rect.top }; };
  return <canvas ref={canvasRef} className={panning ? "is-panning" : ""} aria-label="Interactive lane-level map; drag to pan, wheel or pinch to zoom" tabIndex={0}
    onWheel={(event) => { event.preventDefault(); const rect = event.currentTarget.getBoundingClientRect(); zoomAt(Math.exp(-event.deltaY * .0015), event.clientX - rect.left, event.clientY - rect.top); draw(); }}
    onPointerDown={(event) => { event.currentTarget.setPointerCapture(event.pointerId); pointers.current.set(event.pointerId, canvasPoint(event)); moved.current = false; setPanning(true); }}
    onPointerMove={(event) => {
      if (!pointers.current.has(event.pointerId)) return; const before = new Map(pointers.current), next = canvasPoint(event); pointers.current.set(event.pointerId, next); moved.current = true;
      if (pointers.current.size === 1) { const old = before.get(event.pointerId)!; camera.current.panX += next.x - old.x; camera.current.panY += next.y - old.y; }
      else { const old = [...before.values()], current = [...pointers.current.values()]; const oldDistance = Math.hypot(old[1].x - old[0].x, old[1].y - old[0].y), newDistance = Math.hypot(current[1].x - current[0].x, current[1].y - current[0].y); const center = { x: (current[0].x + current[1].x) / 2, y: (current[0].y + current[1].y) / 2 }; zoomAt(oldDistance ? newDistance / oldDistance : 1, center.x, center.y); }
      draw();
    }}
    onPointerUp={(event) => { pointers.current.delete(event.pointerId); if (!pointers.current.size) setPanning(false); }}
    onPointerCancel={(event) => { pointers.current.delete(event.pointerId); if (!pointers.current.size) setPanning(false); }}
    onClick={(event) => {
      if (moved.current) { moved.current = false; return; }
      const rect = event.currentTarget.getBoundingClientRect(), click = { x: event.clientX - rect.left, y: event.clientY - rect.top }, t = transform();
      let junction: string | null = null, bestJunction = 20; city.intersections.forEach((item) => { const p = screenPoint(item.position, t), distance = Math.hypot(p.x - click.x, p.y - click.y); if (distance < bestJunction) { bestJunction = distance; junction = item.id; } });
      if (junction) { onSelect({ type: "intersection", id: junction }); return; }
      let person: number | null = null, bestPerson = 14; snapshot.people.forEach((item) => { const p = screenPoint(item.position, t), distance = Math.hypot(p.x - click.x, p.y - click.y); if (distance < bestPerson) { bestPerson = distance; person = item.id; } });
      if (person) onSelect({ type: "person", id: person });
    }} />;
}
