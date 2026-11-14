(() => {
  "use strict";

  const canvas = document.querySelector("#city-canvas");
  const dashboard = document.querySelector(".dashboard");
  let ctx = canvas.getContext("2d");
  let renderStructure = null;
  const $ = (id) => document.getElementById(id);
  const state = {
    city: null, snapshot: null, selected: null, layerFilter: "all", showTransforms: false, showCoordinates: true, transform: null,
    camera: { zoom: 1, panX: 0, panY: 0 }, pointers: new Map(), laneDirections: new Map(), presetID: null,
    compute: { tick: null, measuredAt: 0, hz: null },
  };
  try { state.showCoordinates = localStorage.getItem("traffic-show-coordinates") !== "false"; } catch {}
  $("show-coordinates").checked = state.showCoordinates;
  $("show-coordinates").addEventListener("change", event => {
    state.showCoordinates = event.target.checked;
    try { localStorage.setItem("traffic-show-coordinates", String(state.showCoordinates)); } catch {}
    draw();
  });

  function setConnection(status, label) {
    const root = document.querySelector(".connection");
    root.className = `connection ${status}`;
    $("connection-label").textContent = label;
  }

  function fitCanvas() {
    const rect = canvas.getBoundingClientRect();
    const ratio = Math.min(window.devicePixelRatio || 1, 2);
    canvas.width = Math.max(1, Math.round(rect.width * ratio));
    canvas.height = Math.max(1, Math.round(rect.height * ratio));
    ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    if (!state.city) return;
    const b = state.city.bounds, padding = 34;
    const baseScale = Math.min((rect.width - padding * 2) / (b.max_x - b.min_x), (rect.height - padding * 2) / (b.max_y - b.min_y));
    const baseOx = (rect.width - (b.max_x - b.min_x) * baseScale) / 2 - b.min_x * baseScale;
    const baseOy = (rect.height - (b.max_y - b.min_y) * baseScale) / 2 - b.min_y * baseScale;
    state.transform = { baseScale, baseOx, baseOy, width: rect.width, height: rect.height };
    draw();
  }

  function currentTransform() {
    const t = state.transform, c = state.camera;
    return { scale: t.baseScale * c.zoom, ox: t.baseOx + c.panX, oy: t.baseOy + c.panY, width: t.width, height: t.height };
  }

  function screen(point) {
    const t = currentTransform();
    return { x: point.x * t.scale + t.ox, y: point.y * t.scale + t.oy };
  }

  function zoomAt(factor, x, y) {
    if (!state.transform) return;
    const old = currentTransform();
    const world = { x: (x - old.ox) / old.scale, y: (y - old.oy) / old.scale };
    state.camera.zoom = Math.max(.03, Math.min(64, state.camera.zoom * factor));
    const scale = state.transform.baseScale * state.camera.zoom;
    state.camera.panX = x - state.transform.baseOx - world.x * scale;
    state.camera.panY = y - state.transform.baseOy - world.y * scale;
    $("view-scale").textContent = `${Math.round(state.camera.zoom * 100)}%`;
    draw();
  }

  function resetView() {
    state.camera = { zoom: 1, panX: 0, panY: 0 };
    $("view-scale").textContent = "100%";
    draw();
  }

  function traceBezier(points, tension = .55) {
    if (!points.length) return;
    ctx.moveTo(points[0].x, points[0].y);
    if (points.length === 1) return;
    for (let index = 0; index < points.length - 1; index++) {
      const before = points[Math.max(0, index - 1)], start = points[index], end = points[index + 1], after = points[Math.min(points.length - 1, index + 2)];
      const control1 = { x: start.x + (end.x - before.x) * tension / 6, y: start.y + (end.y - before.y) * tension / 6 };
      const control2 = { x: end.x - (after.x - start.x) * tension / 6, y: end.y - (after.y - start.y) * tension / 6 };
      ctx.bezierCurveTo(control1.x, control1.y, control2.x, control2.y, end.x, end.y);
    }
  }

  function drawPolyline(points) {
    traceBezier(points.map(screen));
  }

  function wayLength(way) {
    if (way.length) return way.length;
    return way.points.slice(1).reduce((sum, point, index) => sum + Math.hypot(point.x - way.points[index].x, point.y - way.points[index].y), 0);
  }

  function sampleWay(way, along, lane = 0, extraOffset = 0) {
    const total = wayLength(way), target = Math.max(0, Math.min(total, along));
    let covered = 0;
    for (let index = 0; index < way.points.length - 1; index++) {
      const a = way.points[index], b = way.points[index + 1], dx = b.x - a.x, dy = b.y - a.y, length = Math.hypot(dx, dy);
      if (covered + length >= target || index === way.points.length - 2) {
        const portion = length ? (target - covered) / length : 0, tx = dx / length, ty = dy / length;
        const offset = (way.carriageway_offset || 0) + (lane - (way.lanes - 1) / 2) * way.lane_width + extraOffset;
        return { x: a.x + dx * portion - ty * offset, y: a.y + dy * portion + tx * offset, tx, ty };
      }
      covered += length;
    }
    return { ...way.points[0], tx: 1, ty: 0 };
  }

  function drawWayRange(way, from, to, lane, color, width, extraOffset = 0, lineCap = "round") {
    ctx.strokeStyle = color; ctx.lineWidth = width; ctx.lineCap = lineCap; ctx.lineJoin = "round"; ctx.beginPath();
    const distance = Math.abs(to - from), steps = way.geometry === "sampled" ? Math.max(4,Math.ceil(distance/6)) : Math.max(4, Math.min(36, Math.ceil(distance / 160)));
    const points = [];
    for (let index = 0; index <= steps; index++) {
      points.push(screen(sampleWay(way, from + (to - from) * index / steps, lane, extraOffset)));
    }
    if (way.geometry === "sampled") {points.forEach((p,i)=>i?ctx.lineTo(p.x,p.y):ctx.moveTo(p.x,p.y));} else traceBezier(points);
    ctx.stroke();
  }

  function draw() {
    if (!state.city || !state.transform) return;
    const { width, height } = currentTransform();
    ctx.clearRect(0, 0, width, height);
    drawBackdrop(width, height);
    for (const layer of ["ground", "ramp", "bridge"]) {
      if (state.layerFilter !== "all" && state.layerFilter !== layer) continue;
      renderStructure = layer;
      drawWays(); drawIntersectionPads(); drawSelectedTrip(); drawSignals();
      drawLaneDirectionMarkings(); drawIntersections(); drawTaxiStops(); drawPeople();
    }
    renderStructure = null;
    const groups = new Map();
    state.city.intersections.forEach(n => { if(n.interchange && !groups.has(n.interchange)) groups.set(n.interchange, n); });
    ctx.font="12px Times New Roman"; ctx.fillStyle="#dddddd";
    groups.forEach((n,id) => { const members=state.city.intersections.filter(i=>i.interchange===id), p=screen({x:members.reduce((a,n)=>a+n.position.x,0)/members.length,y:members.reduce((a,n)=>a+n.position.y,0)/members.length});ctx.fillText((state.city.interchanges||[]).find(g=>g.id===id)?.name || `${id} interchange`,p.x+12,p.y-28); });
    drawCoordinateRulers();

  }

  function drawCoordinateRulers() {
    if (!state.showCoordinates) return;
    const t=currentTransform(), b=state.city.bounds, units=state.city.world_units_per_km || 1000;
    const nice=value=>{const power=10**Math.floor(Math.log10(value)), n=value/power;return (n>=5?5:n>=2?2:1)*power;};
    const step=nice(110/t.scale/units), stepWorld=step*units;
    const format=value=>Number(value.toFixed(Math.max(0,-Math.floor(Math.log10(step))+1))).toString();
    const stats=document.querySelector(".stats-overlay").getBoundingClientRect();
    const details=document.querySelector(".right-stack").getBoundingClientRect();
    const detailsOpen=details.width>0 && !document.querySelector(".right-stack").classList.contains("collapsed");
    const left=80, top=26, right=detailsOpen?Math.min(t.width-68,details.left-14):t.width-68;
    const bottom=Math.min(t.height-50,stats.top-12);
    ctx.save();ctx.font="12px Times New Roman";ctx.fillStyle="#bcbcbc";ctx.strokeStyle="#777777";ctx.lineWidth=1;
    ctx.shadowColor="#080808";ctx.shadowBlur=5;
    // Edge rulers only: no lines cross the road surface.
    ctx.beginPath();ctx.moveTo(left,top);ctx.lineTo(right,top);ctx.moveTo(left,top);ctx.lineTo(left,bottom);ctx.stroke();
    const ticks=(min,max,origin,screenMin,screenMax,drawTick)=>{
      const start=Math.max(min,(screenMin-origin)/t.scale), end=Math.min(max,(screenMax-origin)/t.scale);
      for(let i=Math.ceil((start-min)/stepWorld-1e-9);min+i*stepWorld<=end+1e-8;i++)drawTick(origin+(min+i*stepWorld)*t.scale,format(i*step));
    };
    ctx.textAlign="center";ctx.textBaseline="bottom";
    ticks(b.min_x,b.max_x,t.ox,left,right,(x,label)=>{ctx.beginPath();ctx.moveTo(x,top);ctx.lineTo(x,top+5);ctx.stroke();ctx.fillText(label,x,top-4);});
    ctx.textAlign="right";ctx.textBaseline="middle";
    ticks(b.min_y,b.max_y,t.oy,top,bottom,(y,label)=>{ctx.beginPath();ctx.moveTo(left,y);ctx.lineTo(left+5,y);ctx.stroke();ctx.fillText(label,left-7,y);});
    ctx.textAlign="left";ctx.fillText("X · km",left+10,top-12);
    ctx.fillText("Y · km",left+10,top+20);
    const km=nice(130/t.scale/units), pixels=km*units*t.scale, x=(t.width-pixels)/2, y=t.height-24;
    ctx.beginPath();ctx.moveTo(x,y-5);ctx.lineTo(x,y);ctx.lineTo(x+pixels,y);ctx.lineTo(x+pixels,y-5);ctx.stroke();
    ctx.textAlign="center";ctx.textBaseline="bottom";ctx.fillText(`${Number(km.toPrecision(4))} km`,t.width/2,y-6);
    ctx.textBaseline="top";ctx.font="11px Times New Roman";
    ctx.fillText(`${Number(((b.max_x-b.min_x)/units).toFixed(3))} × ${Number(((b.max_y-b.min_y)/units).toFixed(3))} km`,t.width/2,y+4);
    ctx.restore();
  }

  function wayVisible(way) {
    const structure = way.structure || "ground";
    return renderStructure ? structure === renderStructure : state.layerFilter === "all" || structure === state.layerFilter;
  }
  function nodeVisible(node) {
    const layer = node.layer ? "bridge" : "ground";
    const filter = renderStructure || state.layerFilter;
    if (filter === "all") return true;
    if (filter === "ramp") return !renderStructure && state.city.ways.some(w=>w.structure === "ramp" && (w.from === node.id || w.to === node.id));
    return layer === filter;
  }

  function drawBackdrop(width, height) {
    ctx.fillStyle = "#080808"; ctx.fillRect(0, 0, width, height);

  }

  function drawWays() {
    const scale = currentTransform().scale;
    const minimumWidths = { arterial: 4.8, ramp: 3.2, collector: 2.6, local: 1.35 };
    state.city.ways.forEach((way) => {
      if (!wayVisible(way)) return;
      const length = wayLength(way), centre = (way.lanes - 1) / 2;
      const radius = state.city.simulation.intersection_radius || 22;
      const start = Math.min(radius, length/2), end = Math.max(start, length-radius);
      const physicalWidth = way.lanes * way.lane_width * scale;
      const width = Math.max(physicalWidth, minimumWidths[way.road_class] || 1.35);
      if (way.structure === "bridge") {
        ctx.save(); ctx.shadowColor="#000000"; ctx.shadowBlur=10*scale; ctx.shadowOffsetY=4*scale;
        drawWayRange(way, start, end, centre, "#090909", width+8*scale, 0, "butt"); ctx.restore();
      }
      drawWayRange(way, start, end, centre, "rgba(218,218,218,.3)", width + .8 * scale, 0, "butt");
      drawWayRange(way, start, end, centre, "#121212", Math.max(.1, width - .8 * scale), 0, "butt");
      if (way.structure === "bridge") for (const side of [-1,1]) drawWayRange(way,start,end,centre,"#9a9a9a",1.2*scale,side*(way.lanes*way.lane_width/2+2),"butt");
      if (way.lane_width * scale >= 1.2) {
        for (let boundary = 1; boundary < way.lanes; boundary++) {
          ctx.setLineDash([Math.max(2, 5 * scale), Math.max(2, 5 * scale)]);
          drawWayRange(way, start, end, centre, "rgba(218,218,218,.35)", Math.max(.45, .65 * scale), (boundary - way.lanes / 2) * way.lane_width);
          ctx.setLineDash([]);
        }
      }

    });
  }

  function drawIntersectionPads() {
    const scale = currentTransform().scale, radius = state.city.simulation.intersection_radius || 22;
    state.city.intersections.forEach((intersection) => {
      if (!nodeVisible(intersection)) return;
      if(intersection.traffic_control.mode === "flow") {
        const ways=Object.fromEntries(state.city.ways.map(w=>[w.id,w]));
        for(const m of intersection.traffic_control.phases[0].allowed_movements||[]){const a=ways[m.from_way_id],b=ways[m.to_way_id];if(!a||!b)continue;for(const c of m.lane_connections||[]){
          const start=sampleWay(a,Math.max(0,wayLength(a)-radius),c.from_lane),end=sampleWay(b,Math.min(radius,wayLength(b)),c.to_lane),p=screen(start),q=screen(end),h=Math.min(radius*.9,Math.hypot(start.x-end.x,start.y-end.y)*.45)*scale;
          ctx.beginPath();ctx.moveTo(p.x,p.y);ctx.bezierCurveTo(p.x+start.tx*h,p.y+start.ty*h,q.x-end.tx*h,q.y-end.ty*h,q.x,q.y);ctx.strokeStyle="#121212";ctx.lineWidth=Math.min(a.lane_width,b.lane_width)*scale+.5;ctx.lineCap="butt";ctx.stroke();
        }}return;
      }
      const incoming=state.city.ways.filter((way)=>way.to===intersection.id),outgoing=state.city.ways.filter((way)=>way.from===intersection.id),corners=[];
      [...incoming.map((way)=>({way,along:Math.max(0,wayLength(way)-radius)})),...outgoing.map((way)=>({way,along:Math.min(radius,wayLength(way))}))].forEach(({way,along})=>{const sample=sampleWay(way,along,(way.lanes-1)/2),half=way.lanes*way.lane_width/2+2;corners.push({x:sample.x-sample.ty*half,y:sample.y+sample.tx*half},{x:sample.x+sample.ty*half,y:sample.y-sample.tx*half});});
      // Use the mouth edges as anchors, then blend the curb between approaches.
      corners.sort((a,b)=>Math.atan2(a.y-intersection.position.y,a.x-intersection.position.x)-Math.atan2(b.y-intersection.position.y,b.x-intersection.position.x));
      const edges = [];
      [...incoming, ...outgoing].forEach(way => {
        const inbound = way.to === intersection.id;
        const sample = sampleWay(way, inbound ? Math.max(0,wayLength(way)-radius) : Math.min(radius,wayLength(way)), (way.lanes-1)/2);
        const half = way.lanes*way.lane_width/2;
        for (const side of [-1,1]) edges.push({ x:sample.x-side*sample.ty*half, y:sample.y+side*sample.tx*half, dx:sample.tx*(inbound?1:-1), dy:sample.ty*(inbound?1:-1), way:way.id });
      });
      edges.sort((a,b)=>Math.atan2(a.y-intersection.position.y,a.x-intersection.position.x)-Math.atan2(b.y-intersection.position.y,b.x-intersection.position.x));
      if (!edges.length) return;
      const traceEdge = (a,b) => {
        const p=screen(a), q=screen(b);
        if (a.way===b.way) { ctx.lineTo(q.x,q.y); return; }
        const handle=Math.min(radius*.65,Math.hypot(a.x-b.x,a.y-b.y)*.35)*scale;
        ctx.bezierCurveTo(p.x+a.dx*handle,p.y+a.dy*handle,q.x+b.dx*handle,q.y+b.dy*handle,q.x,q.y);
      };
      ctx.fillStyle="#121212";ctx.strokeStyle="rgba(218,218,218,.3)";ctx.lineWidth=.8*scale;ctx.lineCap="butt";ctx.lineJoin="round";
      ctx.beginPath();const first=screen(edges[0]);ctx.moveTo(first.x,first.y);
      edges.forEach((edge,i)=>traceEdge(edge,edges[(i+1)%edges.length]));ctx.closePath();ctx.fill();
      // Mouths stay open: only draw curbs between different approaches.
      edges.forEach((edge,i)=>{const next=edges[(i+1)%edges.length];if(edge.way===next.way)return;ctx.beginPath();const p=screen(edge);ctx.moveTo(p.x,p.y);traceEdge(edge,next);ctx.stroke();});
    });
  }

  function buildLaneDirections(city) {
    const ways = Object.fromEntries(city.ways.map((way) => [way.id, way]));
    const assignments = new Map();
    city.intersections.forEach((intersection) => {
      (intersection.traffic_control?.phases || []).forEach((phase) => {
        (phase.allowed_movements || []).forEach((movement) => {
          const incoming = ways[movement.from_way_id], outgoing = ways[movement.to_way_id];
          if (!incoming || !outgoing) return;
          const fromLanes = movement.from_lanes?.length
            ? movement.from_lanes
            : [...new Set((movement.lane_connections || []).map((connection) => connection.from_lane))];
          const incomingTangent = sampleWay(incoming, wayLength(incoming), 0);
          const outgoingTangent = sampleWay(outgoing, 0, 0);
          const dot = incomingTangent.tx * outgoingTangent.tx + incomingTangent.ty * outgoingTangent.ty;
          const cross = incomingTangent.tx * outgoingTangent.ty - incomingTangent.ty * outgoingTangent.tx;
          const direction = { toWayID: outgoing.id, turn: movement.turn, angle: Math.atan2(cross, dot) };
          if (!assignments.has(incoming.id)) assignments.set(incoming.id, new Map());
          const byLane = assignments.get(incoming.id);
          fromLanes.forEach((lane) => {
            if (!byLane.has(lane)) byLane.set(lane, new Map());
            byLane.get(lane).set(outgoing.id, direction);
          });
        });
      });
    });
    assignments.forEach((byLane) => byLane.forEach((targets, lane) => {
      byLane.set(lane, [...targets.values()].sort((a, b) => a.angle - b.angle || a.toWayID.localeCompare(b.toWayID)));
    }));
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
        const turns = [...new Set(directions.map((direction) => direction.turn || "through"))]
          .sort((a, b) => (order[a] ?? 4) - (order[b] ?? 4));
        // GB 5768.3 limits a combined arrow to two directions. Extremely
        // direction-scarce simulated lanes therefore use adjacent standard
        // groups rather than an invented many-branched symbol.
        const groups = [];
        for (let index = 0; index < turns.length; index += 2) groups.push(turns.slice(index, index + 2));
        const metreScale = way.lane_width / 3.5;
        const nominalMetres = way.speed_limit*3.6 >= 100 ? 9 : way.speed_limit*3.6 > 60 ? 6 : way.speed_limit*3.6 > 40 ? 4.5 : 3;
        const arrowLength = Math.max(6, nominalMetres * metreScale);
        const stopGap = 3 * metreScale;
        const groupSpan = groups.length * arrowLength + Math.max(0, groups.length - 1) * stopGap;
        const repeatSpacing = Math.max(30 * metreScale, groupSpan + arrowLength);
        for (let repetition = 0; repetition < 2; repetition++) {
          for (let group = 0; group < groups.length; group++) {
            const tipAlong = stop - stopGap - repetition * repeatSpacing - group * (arrowLength + stopGap);
            const along = tipAlong - arrowLength / 2;
            if (along - arrowLength / 2 < 0) continue;
            drawLaneDirectionArrow(way, lane, along, groups[group], arrowLength);
          }
        }
      }
    });
  }

  function drawLaneDirectionArrow(way, lane, along, turns, arrowLength) {
    const sample = sampleWay(way, along, lane), position = screen(sample), scale = currentTransform().scale;
    const length = arrowLength * scale, laneWidth = way.lane_width * scale;
    if (laneWidth < 3) return;
    const shaft = Math.min(laneWidth * .10, length * .045);
    const head = Math.min(laneWidth * .16, length * .10);
    ctx.save(); ctx.translate(position.x, position.y); ctx.rotate(Math.atan2(sample.ty, sample.tx));
    ctx.fillStyle = "#dedede"; ctx.strokeStyle = "#dedede";
    ctx.lineWidth = shaft; ctx.lineCap = "butt"; ctx.lineJoin = "round";
    const arrowhead = (x,y,angle) => {
      ctx.save(); ctx.translate(x,y); ctx.rotate(angle);
      ctx.beginPath(); ctx.moveTo(0,0); ctx.lineTo(-head*1.8,-head); ctx.lineTo(-head*1.8,head);ctx.closePath();ctx.fill();ctx.restore();
    };
    ctx.beginPath();ctx.moveTo(-length*.48,0);ctx.lineTo(turns.includes("through") ? length*.46-head : 0,0);ctx.stroke();
    if (turns.includes("through")) arrowhead(length*.46,0,0);
    for (const [turn,sign] of [["left",-1],["right",1]]) if (turns.includes(turn)) {
      const tipY=sign*laneWidth*.36, x=length*.12;
      ctx.beginPath();ctx.moveTo(-length*.12,0);
      ctx.quadraticCurveTo(x,0,x,sign*laneWidth*.14);
      ctx.lineTo(x,tipY-sign*head);ctx.stroke();arrowhead(x,tipY,sign*Math.PI/2);
    }
    if (turns.includes("u_turn")) {
      const sign=state.city.driving_side === "left" ? 1 : -1, y=sign*laneWidth*.30;
      ctx.beginPath();ctx.moveTo(-length*.12,0);
      ctx.bezierCurveTo(length*.30,0,length*.30,y,-length*.12,y);ctx.stroke();
      arrowhead(-length*.12-head,y,Math.PI);
    }
    ctx.restore();
  }

  function drawSelectedTrip() {
    if (state.selected?.type !== "person" || !state.snapshot) return;
    const person = state.snapshot.people.find((item) => item.id === state.selected.id);
    if (!person) return;
    const ways = Object.fromEntries(state.city.ways.map((way) => [way.id, way]));
    person.route.forEach((wayID, index) => {
      const way = ways[wayID]; if (!way || !wayVisible(way)) return;
      ctx.strokeStyle = index < person.route_index ? "rgba(239,200,91,.20)" : "rgba(255,255,255,.32)";
      ctx.lineWidth = 2.5; ctx.beginPath(); drawPolyline(way.points); ctx.stroke();
    });
    const origin = screen(person.origin), current = screen(person.position), destination = screen(person.destination);
    ctx.setLineDash([3,4]); ctx.lineWidth = 1.2 * currentTransform().scale;
    ctx.strokeStyle = "rgba(239,200,91,.72)"; ctx.beginPath(); ctx.moveTo(origin.x, origin.y); ctx.lineTo(current.x, current.y); ctx.stroke();
    ctx.strokeStyle = "rgba(255,120,95,.78)"; ctx.beginPath(); ctx.moveTo(current.x, current.y); ctx.lineTo(destination.x, destination.y); ctx.stroke(); ctx.setLineDash([]);
    ctx.fillStyle = "#efc85b"; ctx.beginPath(); ctx.arc(origin.x, origin.y, 5, 0, Math.PI * 2); ctx.fill();
    ctx.strokeStyle = "#ff785f"; ctx.lineWidth = 2; ctx.beginPath(); ctx.arc(destination.x, destination.y, 6, 0, Math.PI * 2); ctx.stroke();
  }

  function drawSignals() {
    if (!state.snapshot?.signals) return;
    const ways = Object.fromEntries(state.city.ways.map((way) => [way.id, way]));
    state.snapshot.signals.forEach((signal) => {
      const node=state.city.intersections.find(n=>n.id===signal.intersection_id);
      if (!node || !nodeVisible(node) || node.traffic_control.mode === "flow") return;
      const greenLanes = new Map();
      (signal.allowed_movements || []).forEach((movement) => {
        const incoming = ways[movement.from_way_id], outgoing = ways[movement.to_way_id];
        if (!incoming || !outgoing) return;
        const inLength = wayLength(incoming), outLength = wayLength(outgoing), radius = state.city.simulation.intersection_radius || 22, scale = currentTransform().scale;
        const inStop = Math.max(0, inLength - radius), outStart = Math.min(radius, outLength);
        const fromLanes=movement.from_lanes?.length?movement.from_lanes:[0],toLanes=movement.to_lanes?.length?movement.to_lanes:[0];
        const active=greenLanes.get(incoming.id)||new Set();fromLanes.forEach((lane)=>active.add(lane));greenLanes.set(incoming.id,active);
        const count=Math.max(fromLanes.length,toLanes.length),fallback=Array.from({length:count},(_,lane)=>({from_lane:fromLanes[Math.min(lane,fromLanes.length-1)],to_lane:toLanes[Math.min(lane,toLanes.length-1)]})),connections=movement.lane_connections?.length?movement.lane_connections:fallback;
        for (const connection of connections) {
          const start=sampleWay(incoming,inStop,connection.from_lane),end=sampleWay(outgoing,outStart,connection.to_lane),a=screen(start),b=screen(end),handle=Math.min(radius*.9,Math.hypot(end.x-start.x,end.y-start.y)*.45)*scale;
          ctx.strokeStyle="rgba(200,242,107,.82)";ctx.lineWidth=1.45*scale;ctx.beginPath();ctx.moveTo(a.x,a.y);ctx.bezierCurveTo(a.x+start.tx*handle,a.y+start.ty*handle,b.x-end.tx*handle,b.y-end.ty*handle,b.x,b.y);ctx.stroke();
        }
      });
      greenLanes.forEach((lanes,wayID)=>{const way=ways[wayID],scale=currentTransform().scale,radius=state.city.simulation.intersection_radius||22,stop=Math.max(0,wayLength(way)-radius);lanes.forEach((lane)=>drawWayRange(way,Math.max(0,stop-58),stop,lane,"rgba(200,242,107,.86)",2.25*scale));});
      Object.values(ways).filter((way) => way.to === signal.intersection_id).forEach((way) => {
        const scale = currentTransform().scale, radius = state.city.simulation.intersection_radius || 22, stop = Math.max(0, wayLength(way) - radius);
        const sample = sampleWay(way, stop, (way.lanes - 1) / 2), p = screen(sample), green = (signal.allowed_incoming_way_ids || []).includes(way.id);
        ctx.save(); ctx.translate(p.x, p.y); ctx.rotate(Math.atan2(sample.ty, sample.tx)); ctx.strokeStyle = "rgba(238,244,240,.82)"; ctx.lineWidth = 1.4 * scale;
        ctx.beginPath(); ctx.moveTo(0, -way.lanes * way.lane_width * .5 * scale); ctx.lineTo(0, way.lanes * way.lane_width * .5 * scale); ctx.stroke();
        ctx.fillStyle = signal.stage === "amber" ? "#efc85b" : green ? "#c8f26b" : "#ff785f"; ctx.beginPath(); ctx.arc(-2.2 * scale, 0, 1.8 * scale, 0, Math.PI * 2); ctx.fill(); ctx.restore();
      });
    });
  }

  function drawIntersections() {
    const selectedID = state.selected?.type === "intersection" ? state.selected.id : null;
    const denseOverview = state.city.intersections.length > 40 && state.camera.zoom < 4.5;
    state.city.intersections.forEach((intersection) => {
      if (!nodeVisible(intersection)) return;
      const p = screen(intersection.position), signal = state.snapshot?.signals?.find((item) => item.intersection_id === intersection.id);
      const marker=selectedID===intersection.id?7:denseOverview?2.2:5,halo=denseOverview?1.6:2;ctx.fillStyle=selectedID===intersection.id?"#fff":"#dfe4d8";ctx.fillRect(p.x-marker,p.y-marker,marker*2,marker*2);ctx.strokeStyle=signal?.stage==="green"?"rgba(200,242,107,.9)":signal?.stage==="amber"?"rgba(239,200,91,.9)":"rgba(255,120,95,.65)";ctx.lineWidth=selectedID===intersection.id?2:1;ctx.strokeRect(p.x-marker*halo,p.y-marker*halo,marker*halo*2,marker*halo*2);
      if (selectedID === intersection.id || (!denseOverview && state.camera.zoom > .36)) { ctx.fillStyle = "rgba(235,238,228,.66)"; ctx.font = "9px ui-monospace,monospace"; ctx.fillText(`${intersection.id}  ${intersection.name.toUpperCase()}`, p.x + 13, p.y - 8); }
    });
  }

  function drawTransforms() {
    state.city.intersections.forEach((intersection) => intersection.approaches.forEach((approach) => approach.sub_segments.forEach((segment) => {
      const matrix = segment.transform, origin = screen({ x: matrix[0][2], y: matrix[1][2] }), length = 12;
      ctx.lineWidth = 1; ctx.strokeStyle = "rgba(255,120,95,.72)"; ctx.beginPath(); ctx.moveTo(origin.x, origin.y); ctx.lineTo(origin.x + matrix[0][0] * length, origin.y + matrix[1][0] * length); ctx.stroke();
      ctx.strokeStyle = "rgba(200,242,107,.72)"; ctx.beginPath(); ctx.moveTo(origin.x, origin.y); ctx.lineTo(origin.x + matrix[0][1] * length, origin.y + matrix[1][1] * length); ctx.stroke();
    })));
  }

  function drawTaxiStops(){
    const taxi=state.snapshot?.taxi;if(!taxi||renderStructure!=="ground")return;
    for(const stop of taxi.stops){const p=screen(stop.position),order=taxi.orders.find(o=>o.pickup===stop.id&&["Waiting","Assigned","Boarding"].includes(o.state));ctx.save();ctx.strokeStyle=order?"#efc85b":"#7196a1";ctx.lineWidth=1;ctx.strokeRect(p.x-4,p.y-4,8,8);if(order){ctx.fillStyle="#efc85b";ctx.font="12px Times New Roman";ctx.fillText(`#${order.id}`,p.x+8,p.y-5);}ctx.restore();}
  }
  function showOrder(id){const t=state.snapshot?.taxi,o=t?.orders.find(o=>o.id===id);if(!o)return;$("order-title").textContent=`Order #${o.id}`;$("order-state").textContent=o.state;
    const values=[["Taxi",o.vehicle_id?`T-${o.vehicle_id}`:"Unassigned"],["Pickup",o.pickup.replace("stop:","")],["Destination",o.destination.replace("stop:","")],["Requested",`${o.created_s.toFixed(1)} s`],["Pickup estimate",`${o.pickup_eta_s.toFixed(1)} s (free flow)`],["Wait",`${((o.state==="InService"||o.state==="Completed"||o.state==="Boarding")?o.picked_s-o.created_s:state.snapshot.time-o.created_s).toFixed(1)} s`]];
    $("order-facts").replaceChildren(...values.map(([key,value])=>{const row=document.createElement("div"),a=document.createElement("dt"),b=document.createElement("dd");a.textContent=key;b.textContent=value;row.append(a,b);return row;}));
  }

  function drawPeople() {
    if (!state.snapshot) return;
    const scale = currentTransform().scale, ways = Object.fromEntries(state.city.ways.map((way) => [way.id, way]));
    state.snapshot.people.forEach((person) => {
      const structure = person.structure || "ground";
      if (renderStructure && structure !== renderStructure) return;
      const p = screen(person.position), selected = state.selected?.type === "person" && state.selected.id === person.id;
      ctx.save(); ctx.translate(p.x, p.y); ctx.rotate(person.heading_degrees * Math.PI / 180);
      if (person.state.startsWith("walking") || person.state === "arrived") {
        ctx.fillStyle = person.state === "arrived" ? "#ffffff" : "#efc85b"; ctx.shadowColor=ctx.fillStyle;ctx.shadowBlur=4*scale;ctx.beginPath();ctx.arc(0,0,4*scale,0,Math.PI*2);ctx.fill();ctx.shadowBlur=0;ctx.strokeStyle="rgba(7,16,14,.9)";ctx.lineWidth=.8*scale;ctx.stroke();
      } else {
        const way=ways[person.way_id],length=(state.city.simulation.vehicle_size||6)*scale,width=1.8*scale;const taxi=state.snapshot.taxi?.vehicles.find(v=>v.id===person.id);ctx.fillStyle=taxi?({Idle:"#a8b0b5",ToPickup:"#efc85b",Boarding:"#efc85b",WithPassenger:"#69d6ef",Alighting:"#69d6ef"}[taxi.state]||"#eee"):(person.state==="waiting"?"#ff785f":"#c8f26b");ctx.shadowColor=ctx.fillStyle;ctx.shadowBlur=4*scale;ctx.fillRect(-length/2,-width/2,length,width);ctx.shadowBlur=0;ctx.strokeStyle="rgba(6,14,12,.92)";ctx.lineWidth=.7*scale;ctx.strokeRect(-length/2,-width/2,length,width);
      }
      if (selected) { ctx.strokeStyle = "#fff"; ctx.lineWidth = 1.2 * scale; ctx.beginPath(); ctx.arc(0, 0, 8 * scale, 0, Math.PI * 2); ctx.stroke(); }
      ctx.restore();
    });
  }

  function updateSnapshot(snapshot) {
    if(state.snapshot?.mode!==snapshot.mode)state.selected=null;
    state.snapshot = snapshot;
    const taxi=snapshot.taxi;$("simulation-mode").value=snapshot.mode||"traffic";$("taxi-settings").hidden=!taxi;$("traffic-settings").hidden=Boolean(taxi);$("taxi-stats").hidden=!taxi;$("population-label").textContent=taxi?"taxis":"population";
    if(taxi&&state.taxiConfigKey!==JSON.stringify(taxi.config)){state.taxiConfigKey=JSON.stringify(taxi.config);$("taxi-load-feedback").textContent="";$("taxi-fleet").value=taxi.config.fleet;$("taxi-fleet-range").value=taxi.config.fleet;$("taxi-demand").value=taxi.config.orders_per_minute;$("taxi-demand-range").value=taxi.config.orders_per_minute;$("taxi-seed").value=taxi.config.seed;$("taxi-interval").value=taxi.config.dispatch_interval_s;$("taxi-algorithm").value=taxi.config.algorithm;$("taxi-parameters").value=JSON.stringify(taxi.config.parameters||{});}
    if(taxi){
      $("taxi-created").textContent=taxi.created;$("taxi-picked").textContent=taxi.picked;$("taxi-completed").textContent=taxi.completed;$("taxi-cancelled").textContent=taxi.cancelled;$("taxi-wait").textContent=`${taxi.mean_wait_s.toFixed(1)} s`;$("taxi-solver").textContent=taxi.solving?"Computing…":`${taxi.solver_ms.toFixed(1)} ms`;
      const retiring=taxi.vehicles.filter(v=>v.retiring).length;
      $("taxi-fleet-status").textContent=`${taxi.vehicles.length} active${retiring?` · ${retiring} finishing`:""}${taxi.vehicles.length<taxi.config.fleet?` · target ${taxi.config.fleet}`:""}`;
      $("taxi-error").textContent=taxi.last_error||"";
    }

    updateComputeRate(snapshot);
    const minutes = Math.floor(snapshot.time / 60).toString().padStart(2, "0"), seconds = (snapshot.time % 60).toFixed(1).padStart(4, "0");
    $("sim-time").textContent = `${minutes}:${seconds}`; $("people-count").textContent = snapshot.people.length;
    $("driving-count").textContent = snapshot.stats.driving; $("walking-count").textContent = snapshot.stats.walking;
    $("waiting-count").textContent = snapshot.stats.waiting; $("average-speed").textContent = (snapshot.stats.average_speed*3.6).toFixed(1);
    const performance = snapshot.control_performance;
    if (performance) { $("signal-queue").textContent = performance.current_signal_queue; $("throughput").textContent = performance.throughput_per_minute.toFixed(1); $("mean-delay").textContent = `${performance.mean_signal_delay_seconds.toFixed(1)}s`; }
    $("load-current").textContent = snapshot.load?.population ?? snapshot.people.length;
    $("time-scale").value = String(snapshot.time_scale);
    $("pause-button").textContent = snapshot.paused ? "Resume" : "Pause";
    refreshInspector(); draw();
  }

  function updateComputeRate(snapshot) {
    const now = performance.now(), target = 1000 / Math.max(1, Number(state.city?.simulation?.tick_ms || 50));
    if (state.compute.tick !== null && snapshot.tick >= state.compute.tick && now > state.compute.measuredAt) {
      const measured = (snapshot.tick - state.compute.tick) * 1000 / (now - state.compute.measuredAt);
      if (Number.isFinite(measured)) state.compute.hz = state.compute.hz === null ? measured : state.compute.hz * .72 + measured * .28;
    }
    state.compute.tick = snapshot.tick; state.compute.measuredAt = now;
    const measured = snapshot.paused ? 0 : state.compute.hz;
    $("compute-rate").textContent = `${(measured ?? target).toFixed(1)} Hz`;
    $("compute-rate").parentElement.title = `Measured simulation updates per second · target ${target.toFixed(1)} Hz`;
  }

  function updateCity(city) {
    loadPresets();
    state.city = city; state.laneDirections = buildLaneDirections(city);
    $("intersection-count").textContent = city.intersections.filter(n=>n.traffic_control.mode !== "flow").length; $("way-count").textContent = city.ways.length;
    $("lane-count").textContent = city.edges.reduce((sum, edge) => sum + edge.weight, 0);
    state.compute = { tick: null, measuredAt: performance.now(), hz: null };
    const targetHz = 1000 / Math.max(1, Number(city.simulation.tick_ms || 50));
    $("compute-rate").textContent = `${targetHz.toFixed(1)} Hz`;
    $("population-control").max = 2000;
    $("population-control").value = city.simulation.people; $("population-value").textContent = city.simulation.people;
    $("spawn-rate-control").value = (city.simulation.spawn_rate_per_second || 0)*60; $("spawn-rate-value").textContent = `${Number((city.simulation.spawn_rate_per_second || 0)*60).toFixed(0)}/min`;
    $("max-population-control").max = 2000;
    $("max-population-control").value = city.simulation.max_people; $("max-population-value").textContent = city.simulation.max_people;
    $("safe-gap-control").value = city.simulation.safe_gap; $("safe-gap-value").textContent = `${city.simulation.safe_gap} m`;
    fitCanvas();
  }

  function refreshInspector() {
    $("order-inspector").hidden=state.selected?.type!=="order";if(state.selected?.type==="order")showOrder(state.selected.id);
    $("empty-inspector").hidden = Boolean(state.selected); $("person-inspector").hidden = state.selected?.type !== "person"; $("signal-inspector").hidden = state.selected?.type !== "intersection"; $("interchange-inspector").hidden = state.selected?.type !== "interchange";
    if (state.selected?.type === "person") {
      const person = state.snapshot?.people.find((item) => item.id === state.selected.id); if (person) updatePersonInspector(person);
    } else if (state.selected?.type === "intersection") updateSignalInspector(state.selected.id);
    else if (state.selected?.type === "interchange") updateInterchangeInspector(state.selected.id);
  }

  function updatePersonInspector(person) {
    const taxi=state.snapshot?.taxi?.vehicles.find(v=>v.id===person.id);$("taxi-person-detail").textContent=taxi?`${taxi.state} · ${taxi.order_id?"Order #"+taxi.order_id:"Available"} · Completed ${taxi.completed}`:"";
    $("person-id").textContent = `${taxi?"T":"P"}-${String(person.id).padStart(3, "0")}`; $("person-state").textContent = person.state.replaceAll("_", " ").toUpperCase();
    $("person-way").textContent = person.way_id ? `${person.way_id} / ${person.lane + 1}${person.lane_change ? ` → L${person.lane_change.to_lane + 1} (${Math.round(person.lane_change.progress * 100)}%, ${person.lane_change.reason})` : ""}` : "on foot"; $("person-reference").textContent = person.reference.intersection_id;
    $("person-position").textContent = `${person.position.x.toFixed(1)}, ${person.position.y.toFixed(1)}`; $("person-origin").textContent = `${person.origin.x.toFixed(0)}, ${person.origin.y.toFixed(0)}`;
    $("person-destination").textContent = `${person.destination.x.toFixed(0)}, ${person.destination.y.toFixed(0)}`; $("person-route").textContent = person.route.join(" → ");
    $("person-matrix").replaceChildren(...person.reference.local_transform.flat().map((value) => { const span = document.createElement("span"); span.textContent = value.toFixed(2); return span; }));
  }

  function updateSignalInspector(intersectionID) {
    const intersection = state.city?.intersections.find((item) => item.id === intersectionID), signal = state.snapshot?.signals?.find((item) => item.intersection_id === intersectionID);
    if (!intersection || !signal) return;
    $("signal-name").textContent = `${intersection.id} / ${intersection.name}`; $("signal-countdown").textContent = `${signal.remaining_seconds.toFixed(1)}s`;
    $("signal-controller").textContent = signal.controller || "fixed-time"; $("signal-decision").textContent = signal.decision || "scheduled";
    $("signal-green-target").textContent = `${(signal.base_green_seconds || 0).toFixed(1)} → ${(signal.target_green_seconds || 0).toFixed(1)}s`;
    $("signal-pressure").textContent = `${(signal.queue_pressure || 0).toFixed(1)} / ${(signal.competing_pressure || 0).toFixed(1)}`;
    $("signal-plan").replaceChildren(...intersection.traffic_control.phases.map((phase, index) => {
      const row = document.createElement("div"); row.className = `phase-row ${index === signal.phase_index && signal.stage === "green" ? "active" : ""} ${phase.id === signal.next_phase_id ? "next" : ""}`;
      row.title = phase.id === signal.next_phase_id ? "Next phase" : index === signal.phase_index && signal.stage === "green" ? "Active phase" : "";
      const dot = document.createElement("i"), name = document.createElement("span"), duration = document.createElement("strong");
      name.textContent = `Phase ${index + 1}`; duration.textContent = `${phase.duration_seconds}s · ${phase.min_green_seconds || phase.duration_seconds}–${phase.max_green_seconds || phase.duration_seconds}s`; row.append(dot, name, duration); return row;
    }));
    const phases = intersection.traffic_control.phases;
    const container = $("signal-movements");
    const signature = JSON.stringify([intersection.id, phases]);
    if (container.dataset.signature !== signature) {
      container.dataset.signature = signature;
      container.replaceChildren(...phases.map((phase, index) => {
        const card = document.createElement("div"); card.className = "flow-card";
        card.dataset.phaseIndex = index;
        const title = document.createElement("div"); title.textContent = `Phase ${index + 1}`;
        card.append(title);
        const copy = document.createElement("canvas"); copy.width = 680; copy.height = 520;
        copy.style.width = "100%"; copy.style.height = "auto";
        copy.setAttribute("aria-label", `${intersection.id}: ${title.textContent}`);
        drawJunctionCopy(copy, intersection, phase);
        card.append(copy);return card;
      }));
    }
    [...container.children].forEach(card => {
      const active=signal.stage === "green" && Number(card.dataset.phaseIndex) === signal.phase_index;
      card.classList.toggle("flow-active",active);
    });
  }

  function drawJunctionCopy(copy, intersection, phase) {
    const saved = { ctx, renderStructure, layerFilter:state.layerFilter, city: state.city, snapshot: state.snapshot, transform: state.transform, camera: state.camera };
    try {
      ctx = copy.getContext("2d"); renderStructure=null; state.layerFilter="all";
      const ways = saved.city.ways.filter(w => w.from === intersection.id || w.to === intersection.id);
      const radius = saved.city.simulation.intersection_radius || 22;
      // Fit the junction mouth geometry, rather than long approach roads.
      const corners = [];
      ways.forEach(way => {
        const along = way.to === intersection.id ? Math.max(0, wayLength(way)-radius) : Math.min(radius, wayLength(way));
        const point = sampleWay(way, along, (way.lanes-1)/2);
        const half = way.lanes*way.lane_width/2+2;
        for (const side of [-1,1]) corners.push({ x: point.x-side*point.ty*half, y: point.y+side*point.tx*half });
      });
      const minX = Math.min(...corners.map(p => p.x)), maxX = Math.max(...corners.map(p => p.x));
      const minY = Math.min(...corners.map(p => p.y)), maxY = Math.max(...corners.map(p => p.y));
      const margin = Math.max(4, radius*.18);
      const scale = Math.min(copy.width/(maxX-minX+margin*2), copy.height/(maxY-minY+margin*2));
      state.transform = { baseScale: scale, baseOx: copy.width/2-(minX+maxX)/2*scale, baseOy: copy.height/2-(minY+maxY)/2*scale, width: copy.width, height: copy.height };
      state.camera = { zoom: 1, panX: 0, panY: 0 };
      state.city = { ...saved.city, ways, intersections: [intersection] };
      state.snapshot = { signals: [{ intersection_id: intersection.id, stage: "green", allowed_movements: phase.allowed_movements || [], allowed_incoming_way_ids: [...new Set((phase.allowed_movements || []).map(m => m.from_way_id))] }] };
      ctx.fillStyle = "#080808"; ctx.fillRect(0,0,copy.width,copy.height);
      drawWays(); drawIntersectionPads(); drawLaneDirectionMarkings(); drawSignals();
      ctx.fillStyle = "#e6ecdf"; ctx.font = "16px sans-serif";
      ctx.fillText(intersection.id, 12, 22);
    } finally {
      ctx = saved.ctx; renderStructure=saved.renderStructure; state.layerFilter=saved.layerFilter; state.city = saved.city; state.snapshot = saved.snapshot;
      state.transform = saved.transform; state.camera = saved.camera;
    }
  }

  function updateInterchangeInspector(id) {
    const group=(state.city.interchanges||[]).find(g=>g.id===id);if(!group)return;
    const select=$("interchange-movement"), copy=$("interchange-canvas");
    const key=JSON.stringify(group);
    if(select.dataset.group!==key){
      select.dataset.group=key;select.replaceChildren(...group.routes.map((route,i)=>{const o=document.createElement("option");o.value=i;o.textContent=`${route.origin} → ${route.destination} · ${route.turn}`;return o;}));copy.dataset.rendered="";
    }
    const signature=key+select.value;if(copy.dataset.rendered===signature)return;copy.dataset.rendered=signature;
    const route=group.routes[Number(select.value)||0], ids=new Set(route.way_ids);
    const nodes=state.city.intersections.filter(n=>n.interchange===id), nodeIDs=new Set(nodes.map(n=>n.id));
    const ways=state.city.ways.filter(w=>nodeIDs.has(w.from)&&nodeIDs.has(w.to));
    $("interchange-name").textContent=group.name;
    $("interchange-summary").textContent=group.type === "trumpet" ? "1 loop · 1 semi-direct ramp · 2 right-turn ramps" : "4 loops · 4 right-turn ramps · 4 C/D carriageways";
    $("interchange-route-detail").textContent=`${ways.some(w=>ids.has(w.id)&&w.role==="loop_ramp") ? "Loop ramp" : ways.some(w=>ids.has(w.id)&&w.role==="semi_direct_ramp") ? "Semi-direct ramp" : route.turn === "right" ? "Direct right-turn ramp" : "Continuous mainline"} · ${route.way_ids.length} road segments`;
    const saved={ctx,city:state.city,snapshot:state.snapshot,transform:state.transform,camera:state.camera,renderStructure,layerFilter:state.layerFilter};
    try {
      ctx=copy.getContext("2d");state.city={...state.city,ways,intersections:nodes};state.snapshot=null;state.layerFilter="all";
      const points=ways.flatMap(w=>w.points), minX=Math.min(...points.map(p=>p.x)), maxX=Math.max(...points.map(p=>p.x)), minY=Math.min(...points.map(p=>p.y)),maxY=Math.max(...points.map(p=>p.y));
      const scale=Math.min(copy.width/(maxX-minX+100),copy.height/(maxY-minY+100));
      state.transform={baseScale:scale,baseOx:copy.width/2-(minX+maxX)*scale/2,baseOy:copy.height/2-(minY+maxY)*scale/2,width:copy.width,height:copy.height};state.camera={zoom:1,panX:0,panY:0};
      ctx.clearRect(0,0,copy.width,copy.height);
      for(const layer of ["ground","ramp","bridge"]){renderStructure=layer;drawWays();drawIntersectionPads();for(const w of ways){if(ids.has(w.id)&&wayVisible(w)){drawWayRange(w,0,wayLength(w),(w.lanes-1)/2,"#c8f26b",3);for(let d=80;d<wayLength(w);d+=180){const v=sampleWay(w,d,(w.lanes-1)/2),p=screen(v);ctx.save();ctx.translate(p.x,p.y);ctx.rotate(Math.atan2(v.ty,v.tx));ctx.fillStyle="#e7ffb1";ctx.beginPath();ctx.moveTo(5,0);ctx.lineTo(-4,-3);ctx.lineTo(-4,3);ctx.closePath();ctx.fill();ctx.restore();}}}}
    } finally {ctx=saved.ctx;state.city=saved.city;state.snapshot=saved.snapshot;state.transform=saved.transform;state.camera=saved.camera;renderStructure=saved.renderStructure;state.layerFilter=saved.layerFilter;}
  }

  async function control(action, extras = {}) {
    try { const response = await fetch("/api/control", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ action, ...extras }) }); if (!response.ok) throw new Error(await response.text()); const snapshot = await response.json(); updateSnapshot(snapshot); return snapshot; }
    catch (error) { setConnection("error", "Control request failed"); console.error(error);$("taxi-error").textContent=error.message; return null; }
  }

  async function loadPresets() {
    try {
      const response = await fetch("/api/presets");
      if (!response.ok) throw new Error(await response.text());
      const payload = await response.json(), select = $("map-preset");
      select.replaceChildren(...payload.presets.map((preset, index) => {
        const option = document.createElement("option");
        option.value = preset.id; option.textContent = `Preset ${index + 1} · ${preset.name}`; 
        return option;
      }));
      state.presetID = payload.current_preset_id; select.value = state.presetID; select.disabled = payload.presets.length < 2;
    } catch (error) {
      setConnection("error", "Map presets unavailable"); console.error(error);
    }
  }

  async function switchPreset(presetID) {
    const select = $("map-preset"), previous = state.presetID;
    select.disabled = true; state.selected = null; state.snapshot = null; setConnection("", "Switching map…");
    const snapshot = await control("set_preset", { preset_id: presetID });
    if (!snapshot) { select.value = previous; select.disabled = false; return; }
    try {
      const response = await fetch("/api/city");
      if (!response.ok) throw new Error(await response.text());
      state.camera = { zoom: 1, panX: 0, panY: 0 }; $("view-scale").textContent = "100%";
      updateCity(await response.json()); state.presetID = presetID; select.value = presetID;
      setConnection("live", "Live model connected");
    } catch (error) {
      setConnection("error", "Map switch incomplete"); console.error(error);
    } finally {
      select.disabled = false;
    }
  }

  function canvasPoint(event) { const rect = canvas.getBoundingClientRect(); return { x: event.clientX - rect.left, y: event.clientY - rect.top }; }
  canvas.addEventListener("wheel", (event) => { event.preventDefault(); const p = canvasPoint(event); zoomAt(Math.exp(-event.deltaY * .0015), p.x, p.y); }, { passive: false });
  canvas.addEventListener("pointerdown", (event) => { canvas.setPointerCapture(event.pointerId); state.pointers.set(event.pointerId, canvasPoint(event)); canvas.classList.add("is-panning"); });
  canvas.addEventListener("pointermove", (event) => {
    if (!state.pointers.has(event.pointerId)) return;
    const before = new Map(state.pointers), next = canvasPoint(event); state.pointers.set(event.pointerId, next);
    if (state.pointers.size === 1) { const old = before.get(event.pointerId); state.camera.panX += next.x - old.x; state.camera.panY += next.y - old.y; draw(); }
    else if (state.pointers.size === 2) {
      const oldPoints = [...before.values()], newPoints = [...state.pointers.values()];
      const oldDistance = Math.hypot(oldPoints[1].x - oldPoints[0].x, oldPoints[1].y - oldPoints[0].y), newDistance = Math.hypot(newPoints[1].x - newPoints[0].x, newPoints[1].y - newPoints[0].y);
      const oldCenter = { x: (oldPoints[0].x + oldPoints[1].x) / 2, y: (oldPoints[0].y + oldPoints[1].y) / 2 }, center = { x: (newPoints[0].x + newPoints[1].x) / 2, y: (newPoints[0].y + newPoints[1].y) / 2 };
      state.camera.panX += center.x - oldCenter.x; state.camera.panY += center.y - oldCenter.y; zoomAt(oldDistance ? newDistance / oldDistance : 1, center.x, center.y);
    }
  });
  function endPointer(event) { state.pointers.delete(event.pointerId); if (!state.pointers.size) canvas.classList.remove("is-panning"); }
  canvas.addEventListener("pointerup", endPointer); canvas.addEventListener("pointercancel", endPointer);
  canvas.addEventListener("click", (event) => {
    if (!state.snapshot || !state.transform) return; const click = canvasPoint(event);
    const t=state.snapshot.taxi;if(t){const stop=t.stops.find(st=>{const p=screen(st.position);return Math.hypot(p.x-click.x,p.y-click.y)<10;});if(stop){const o=t.orders.find(o=>o.pickup===stop.id&&["Waiting","Assigned","Boarding"].includes(o.state));if(o){state.selected={type:"order",id:o.id};refreshInspector();draw();return;}}}
    let intersectionMatch = null, intersectionDistance = 18;
    state.city.intersections.filter(nodeVisible).sort((a,b)=>(a.layer||0)-(b.layer||0)).forEach((item) => { const p = screen(item.position), d = Math.hypot(p.x - click.x, p.y - click.y); if (d < intersectionDistance) { intersectionMatch = item; intersectionDistance = d; } });
    const group=(state.city.interchanges||[]).find(g => (intersectionMatch?.interchange===g.id) || Math.hypot(screen(g.center).x-click.x,screen(g.center).y-click.y)<45);
    if (group) state.selected={type:"interchange",id:group.id};
    else if (intersectionMatch) state.selected = { type: "intersection", id: intersectionMatch.id };
    else { let personMatch = null, distance = 13; state.snapshot.people.filter(p=>state.layerFilter === "all" || (p.structure||"ground") === state.layerFilter).forEach((person) => { const p = screen(person.position), d = Math.hypot(p.x - click.x, p.y - click.y); if (d < distance) { personMatch = person; distance = d; } }); if (personMatch) state.selected = { type: "person", id: personMatch.id }; }
    refreshInspector(); draw();
  });

  $("pause-button").addEventListener("click", () => control(state.snapshot?.paused ? "resume" : "pause")); $("step-button").addEventListener("click", () => control("step", { delta: .05 }));
  $("reset-button").addEventListener("click", () => { state.selected = null; control("reset"); }); $("time-scale").addEventListener("change", (event) => control("set_time_scale", { time_scale: Number(event.target.value) }));
  $("inspector-toggle").addEventListener("click", () => {
    const collapsed = document.querySelector(".right-stack").classList.toggle("collapsed");
    $("inspector-toggle").textContent = collapsed ? "‹" : "›";
    $("inspector-toggle").setAttribute("aria-expanded", String(!collapsed));
    $("inspector-toggle").setAttribute("aria-label", collapsed ? "Expand details" : "Collapse details");
    $("inspector-toggle").title = collapsed ? "Expand details" : "Collapse details";
    draw();
  });
  const settingsToggle = $("settings-toggle"), settingsPanel = $("settings-panel");
  function toggleSettings(open) {
    settingsPanel.hidden = !open; settingsToggle.setAttribute("aria-expanded", String(open));
    if (open) {layoutSettings();$("settings-close").focus();} else settingsToggle.focus();
  }
  let settingsRect={x:88,y:68,width:460,height:650},settingsMaximized=false,settingsGesture=null;
  try{const saved=JSON.parse(localStorage.getItem("traffic-settings-window"));if(saved&&[saved.x,saved.y,saved.width,saved.height].every(Number.isFinite))settingsRect=saved;}catch{}
  function layoutSettings(){
    const availableW=window.innerWidth-16,availableH=window.innerHeight-16;
    let r=settingsMaximized?{x:8,y:8,width:availableW,height:availableH}:{...settingsRect};
    r.width=Math.max(Math.min(340,availableW),Math.min(r.width,availableW));r.height=Math.max(Math.min(300,availableH),Math.min(r.height,availableH));
    r.x=Math.max(8,Math.min(r.x,window.innerWidth-r.width-8));r.y=Math.max(8,Math.min(r.y,window.innerHeight-r.height-8));
    Object.assign(settingsPanel.style,{left:`${r.x}px`,top:`${r.y}px`,width:`${r.width}px`,height:`${r.height}px`});
    if(!settingsMaximized)settingsRect=r;
    $("settings-maximize").textContent=settingsMaximized?"❐":"□";$("settings-maximize").setAttribute("aria-label",settingsMaximized?"Restore settings size":"Maximize settings");$("settings-maximize").title=settingsMaximized?"Restore settings size":"Maximize settings";
  }
  function saveSettingsRect(){try{localStorage.setItem("traffic-settings-window",JSON.stringify(settingsRect));}catch{}}
  function maximizeSettings(){settingsMaximized=!settingsMaximized;layoutSettings();}
  $("settings-maximize").addEventListener("click",maximizeSettings);
  $("settings-drag").addEventListener("dblclick",e=>{if(!e.target.closest("button"))maximizeSettings();});
  function startSettingsGesture(event,kind){
    if(event.button!==0||kind==="move"&&event.target.closest("button"))return;
    event.preventDefault();if(settingsMaximized){settingsMaximized=false;layoutSettings();}
    settingsGesture={kind,id:event.pointerId,x:event.clientX,y:event.clientY,rect:{...settingsRect}};
    try{event.currentTarget.setPointerCapture(event.pointerId);}catch{}
  }
  $("settings-drag").addEventListener("pointerdown",e=>startSettingsGesture(e,"move"));
  $("settings-resize").addEventListener("pointerdown",e=>startSettingsGesture(e,"resize"));
  window.addEventListener("pointermove",e=>{const g=settingsGesture;if(!g||g.id!==e.pointerId)return;const dx=e.clientX-g.x,dy=e.clientY-g.y;
    settingsRect=g.kind==="move"?{...g.rect,x:g.rect.x+dx,y:g.rect.y+dy}:{...g.rect,width:g.rect.width+dx,height:g.rect.height+dy};layoutSettings();
  });
  function finishSettingsGesture(){if(settingsGesture){settingsGesture=null;saveSettingsRect();}}
  window.addEventListener("pointerup",finishSettingsGesture);window.addEventListener("pointercancel",finishSettingsGesture);
  for(const [id,kind] of [["settings-drag","move"],["settings-resize","resize"]])$(id).addEventListener("keydown",e=>{if(e.target!==e.currentTarget||!["ArrowLeft","ArrowRight","ArrowUp","ArrowDown"].includes(e.key))return;e.preventDefault();settingsMaximized=false;const d=e.shiftKey?32:8,x=e.key==="ArrowLeft"?-d:e.key==="ArrowRight"?d:0,y=e.key==="ArrowUp"?-d:e.key==="ArrowDown"?d:0;
    if(kind==="move"){settingsRect.x+=x;settingsRect.y+=y;}else{settingsRect.width+=x;settingsRect.height+=y;}layoutSettings();saveSettingsRect();});
  window.addEventListener("resize",layoutSettings);
  settingsToggle.addEventListener("click", () => toggleSettings(settingsPanel.hidden));
  $("settings-close").addEventListener("click", () => toggleSettings(false));
  document.addEventListener("keydown", event => { if (event.key === "Escape" && !settingsPanel.hidden) toggleSettings(false); });
  $("interchange-movement").addEventListener("change",()=>{if(state.selected?.type==="interchange")updateInterchangeInspector(state.selected.id);});
  $("layer-filter").addEventListener("change", event => {
    state.layerFilter=event.target.value; state.selected=null; refreshInspector(); draw();
  });
  $("map-preset").addEventListener("change", (event) => switchPreset(event.target.value));
  $("zoom-in-button").addEventListener("click", () => zoomAt(1.5, state.transform.width / 2, state.transform.height / 2)); $("zoom-out-button").addEventListener("click", () => zoomAt(1 / 1.5, state.transform.width / 2, state.transform.height / 2)); $("fit-button").addEventListener("click", resetView);
  $("fullscreen-button").addEventListener("click", async () => { if (document.fullscreenElement) await document.exitFullscreen(); else await dashboard.requestFullscreen(); });
  const loadControls = [["population-control","population-value",(value) => value],["spawn-rate-control","spawn-rate-value",(value) => `${Number(value).toFixed(0)}/min`],["max-population-control","max-population-value",(value) => value]];
  loadControls.forEach(([controlID, valueID, format]) => $(controlID).addEventListener("input", (event) => { $(valueID).textContent = format(event.target.value); }));
  $("safe-gap-control").addEventListener("input",(event)=>{const safeGap=Number(event.target.value);$("safe-gap-value").textContent=`${safeGap} m`;control("set_spacing",{safe_gap:safeGap});});
  $("apply-load-button").addEventListener("click", () => {
    const population = Number($("population-control").value), spawnRate = Number($("spawn-rate-control").value)/60, maxPopulation = Math.max(population, Number($("max-population-control").value));
    $("max-population-control").value = maxPopulation; $("max-population-value").textContent = maxPopulation;
    control("set_load", { population, spawn_rate_per_second: spawnRate, max_population: maxPopulation });
  });
  document.addEventListener("fullscreenchange", () => { $("fullscreen-button").textContent = document.fullscreenElement ? "Exit full screen" : "Full screen"; setTimeout(fitCanvas, 0); }); window.addEventListener("resize", fitCanvas);

  function taxiConfiguration(){return {fleet:Number($("taxi-fleet").value),orders_per_minute:Number($("taxi-demand").value),algorithm:$("taxi-algorithm").value,seed:Number($("taxi-seed").value),dispatch_interval_s:Number($("taxi-interval").value),max_wait_s:600,dwell_s:20,parameters:JSON.parse($("taxi-parameters").value||"{}")};}
  async function applyMode(){try{const mode=$("simulation-mode").value;await control("set_mode",{mode,taxi:mode==="taxi"?taxiConfiguration():{}});}catch(error){$("taxi-error").textContent=error.message;}}
  for(const id of ["taxi-fleet","taxi-demand"]){
    $(id+"-range").addEventListener("input",()=>{$(id).value=$(id+"-range").value;$("taxi-load-feedback").textContent="Ready to apply";});
    $(id).addEventListener("input",()=>{$(id+"-range").value=$(id).value;$("taxi-load-feedback").textContent="Ready to apply";});
  }
  $("taxi-load-apply").addEventListener("click",async()=>{
    if(!$("taxi-fleet").reportValidity()||!$("taxi-demand").reportValidity())return;
    const button=$("taxi-load-apply");button.disabled=true;
    try{const snapshot=await control("set_taxi_load",{taxi:{fleet:Number($("taxi-fleet").value),orders_per_minute:Number($("taxi-demand").value)}});
      $("taxi-load-feedback").textContent=snapshot?"Applied · active trips continue":"Could not apply changes";
    }finally{button.disabled=false;}
  });
  $("simulation-mode").addEventListener("change",applyMode);$("taxi-apply").addEventListener("click",applyMode);
  $("taxi-export").addEventListener("click",()=>{const data={schema_version:1,city:state.city,snapshot:state.snapshot},url=URL.createObjectURL(new Blob([JSON.stringify(data,null,2)],{type:"application/json"}));const a=document.createElement("a");a.href=url;a.download="taxi-run.json";a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);});
  fetch("/api/dispatch/algorithms").then(r=>r.json()).then(items=>{$("taxi-algorithm").replaceChildren(...items.map(item=>{const o=document.createElement("option");o.value=item.id;o.textContent=item.name;return o;}));$("taxi-algorithm").value="nearest";}).catch(error=>{$("taxi-error").textContent=error.message;});
  loadPresets();
  const events = new EventSource("/api/stream"); events.addEventListener("open", () => setConnection("live", "Live model connected")); events.addEventListener("city", (event) => updateCity(JSON.parse(event.data))); events.addEventListener("snapshot", (event) => updateSnapshot(JSON.parse(event.data))); events.addEventListener("error", () => setConnection("error", "Reconnecting to model"));
})();
