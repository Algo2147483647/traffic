(() => {
  "use strict";

  const canvas = document.querySelector("#city-canvas");
  const dashboard = document.querySelector(".dashboard");
  let ctx = canvas.getContext("2d");
  const $ = (id) => document.getElementById(id);
  const state = {
    city: null, snapshot: null, selected: null, showTransforms: false, transform: null,
    camera: { zoom: 1, panX: 0, panY: 0 }, pointers: new Map(), laneDirections: new Map(), presetID: null,
    compute: { tick: null, measuredAt: 0, hz: null },
  };

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
    const distance = Math.abs(to - from), steps = Math.max(4, Math.min(36, Math.ceil(distance / 160)));
    const points = [];
    for (let index = 0; index <= steps; index++) {
      points.push(screen(sampleWay(way, from + (to - from) * index / steps, lane, extraOffset)));
    }
    traceBezier(points);
    ctx.stroke();
  }

  function draw() {
    if (!state.city || !state.transform) return;
    const { width, height } = currentTransform();
    ctx.clearRect(0, 0, width, height);
    drawBackdrop(width, height);
    drawWays();
    drawIntersectionPads();
    drawSelectedTrip();
    drawSignals();
    drawLaneDirectionMarkings();
    drawIntersections();
    if (state.showTransforms) drawTransforms();
    drawPeople();
  }

  function drawBackdrop(width, height) {
    ctx.fillStyle = "#080808"; ctx.fillRect(0, 0, width, height);

  }

  function drawWays() {
    const scale = currentTransform().scale;
    const minimumWidths = { arterial: 4.8, ramp: 3.2, collector: 2.6, local: 1.35 };
    state.city.ways.forEach((way) => {
      const length = wayLength(way), centre = (way.lanes - 1) / 2;
      const radius = state.city.simulation.intersection_radius || 22;
      const start = Math.min(radius, length/2), end = Math.max(start, length-radius);
      const physicalWidth = way.lanes * way.lane_width * scale;
      const width = Math.max(physicalWidth, minimumWidths[way.road_class] || 1.35);
      drawWayRange(way, start, end, centre, "rgba(218,218,218,.3)", width + .8 * scale, 0, "butt");
      drawWayRange(way, start, end, centre, "#121212", Math.max(.1, width - .8 * scale), 0, "butt");
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
        const nominalMetres = way.speed_limit >= 100 ? 9 : way.speed_limit > 60 ? 6 : way.speed_limit > 40 ? 4.5 : 3;
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
      const way = ways[wayID]; if (!way) return;
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
          const start=sampleWay(incoming,inStop,connection.from_lane),end=sampleWay(outgoing,outStart,connection.to_lane),a=screen(start),b=screen(end),handle=radius*.82*scale;
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

  function drawPeople() {
    if (!state.snapshot) return;
    const scale = currentTransform().scale, ways = Object.fromEntries(state.city.ways.map((way) => [way.id, way]));
    state.snapshot.people.forEach((person) => {
      const p = screen(person.position), selected = state.selected?.type === "person" && state.selected.id === person.id;
      ctx.save(); ctx.translate(p.x, p.y); ctx.rotate(person.heading_degrees * Math.PI / 180);
      if (person.state.startsWith("walking") || person.state === "arrived") {
        ctx.fillStyle = person.state === "arrived" ? "#ffffff" : "#efc85b"; ctx.shadowColor=ctx.fillStyle;ctx.shadowBlur=4*scale;ctx.beginPath();ctx.arc(0,0,4*scale,0,Math.PI*2);ctx.fill();ctx.shadowBlur=0;ctx.strokeStyle="rgba(7,16,14,.9)";ctx.lineWidth=.8*scale;ctx.stroke();
      } else {
        const way=ways[person.way_id],length=(state.city.simulation.vehicle_size||12)*scale,width=Math.min(6.5,(way?.lane_width||7)*.72)*scale;ctx.fillStyle=person.state==="waiting"?"#ff785f":"#c8f26b";ctx.shadowColor=ctx.fillStyle;ctx.shadowBlur=4*scale;ctx.fillRect(-length/2,-width/2,length,width);ctx.shadowBlur=0;ctx.strokeStyle="rgba(6,14,12,.92)";ctx.lineWidth=.7*scale;ctx.strokeRect(-length/2,-width/2,length,width);
      }
      if (selected) { ctx.strokeStyle = "#fff"; ctx.lineWidth = 1.2 * scale; ctx.beginPath(); ctx.arc(0, 0, 8 * scale, 0, Math.PI * 2); ctx.stroke(); }
      ctx.restore();
    });
  }

  function updateSnapshot(snapshot) {
    state.snapshot = snapshot;
    updateComputeRate(snapshot);
    const minutes = Math.floor(snapshot.time / 60).toString().padStart(2, "0"), seconds = (snapshot.time % 60).toFixed(1).padStart(4, "0");
    $("sim-time").textContent = `${minutes}:${seconds}`; $("people-count").textContent = snapshot.people.length;
    $("driving-count").textContent = snapshot.stats.driving; $("walking-count").textContent = snapshot.stats.walking;
    $("waiting-count").textContent = snapshot.stats.waiting; $("average-speed").textContent = snapshot.stats.average_speed.toFixed(1);
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
    state.city = city; state.laneDirections = buildLaneDirections(city);
    $("intersection-count").textContent = city.intersections.length; $("way-count").textContent = city.ways.length;
    $("lane-count").textContent = city.edges.reduce((sum, edge) => sum + edge.weight, 0);
    state.compute = { tick: null, measuredAt: performance.now(), hz: null };
    const targetHz = 1000 / Math.max(1, Number(city.simulation.tick_ms || 50));
    $("compute-rate").textContent = `${targetHz.toFixed(1)} Hz`;
    $("population-control").max = 10000;
    $("population-control").value = city.simulation.people; $("population-value").textContent = city.simulation.people;
    $("spawn-rate-control").value = city.simulation.spawn_rate_per_second || 0; $("spawn-rate-value").textContent = `${Number(city.simulation.spawn_rate_per_second || 0).toFixed(1)}/s`;
    $("max-population-control").max = 10000;
    $("max-population-control").value = city.simulation.max_people; $("max-population-value").textContent = city.simulation.max_people;
    $("safe-gap-control").value = city.simulation.safe_gap; $("safe-gap-value").textContent = `${city.simulation.safe_gap} m`;
    fitCanvas();
  }

  function refreshInspector() {
    $("empty-inspector").hidden = Boolean(state.selected); $("person-inspector").hidden = state.selected?.type !== "person"; $("signal-inspector").hidden = state.selected?.type !== "intersection";
    if (state.selected?.type === "person") {
      const person = state.snapshot?.people.find((item) => item.id === state.selected.id); if (person) updatePersonInspector(person);
    } else if (state.selected?.type === "intersection") updateSignalInspector(state.selected.id);
  }

  function updatePersonInspector(person) {
    $("person-id").textContent = `P-${String(person.id).padStart(3, "0")}`; $("person-state").textContent = person.state.replaceAll("_", " ").toUpperCase();
    $("person-way").textContent = person.way_id ? `${person.way_id} / ${person.lane + 1}${person.lane_change ? ` → L${person.lane_change.to_lane + 1} (${Math.round(person.lane_change.progress * 100)}%, ${person.lane_change.reason})` : ""}` : "on foot"; $("person-reference").textContent = person.reference.intersection_id;
    $("person-position").textContent = `${person.position.x.toFixed(1)}, ${person.position.y.toFixed(1)}`; $("person-origin").textContent = `${person.origin.x.toFixed(0)}, ${person.origin.y.toFixed(0)}`;
    $("person-destination").textContent = `${person.destination.x.toFixed(0)}, ${person.destination.y.toFixed(0)}`; $("person-route").textContent = person.route.join(" → ");
    $("person-matrix").replaceChildren(...person.reference.local_transform.flat().map((value) => { const span = document.createElement("span"); span.textContent = value.toFixed(2); return span; }));
  }

  function updateSignalInspector(intersectionID) {
    const intersection = state.city?.intersections.find((item) => item.id === intersectionID), signal = state.snapshot?.signals?.find((item) => item.intersection_id === intersectionID);
    if (!intersection || !signal) return;
    $("signal-name").textContent = `${intersection.id} / ${intersection.name}`; $("signal-countdown").textContent = `${signal.remaining_seconds.toFixed(1)}s`;
    $("signal-phase").textContent = `${signal.phase_name} · ${(signal.stage || "green").replace("_", " ")}`;
    $("signal-next").textContent = `Next: ${signal.next_phase_name || intersection.traffic_control.phases[(signal.phase_index + 1) % intersection.traffic_control.phases.length].name}`;
    $("signal-controller").textContent = signal.controller || "fixed-time"; $("signal-decision").textContent = signal.decision || "scheduled";
    $("signal-green-target").textContent = `${(signal.base_green_seconds || 0).toFixed(1)} → ${(signal.target_green_seconds || 0).toFixed(1)}s`;
    $("signal-pressure").textContent = `${(signal.queue_pressure || 0).toFixed(1)} / ${(signal.competing_pressure || 0).toFixed(1)}`;
    $("signal-plan").replaceChildren(...intersection.traffic_control.phases.map((phase, index) => {
      const row = document.createElement("div"); row.className = `phase-row ${index === signal.phase_index ? "active" : ""}`;
      const dot = document.createElement("i"), name = document.createElement("span"), duration = document.createElement("strong");
      name.textContent = phase.name; duration.textContent = `${phase.duration_seconds}s · ${phase.min_green_seconds || phase.duration_seconds}–${phase.max_green_seconds || phase.duration_seconds}s`; row.append(dot, name, duration); return row;
    }));
    const phases = intersection.traffic_control.phases;
    const container = $("signal-movements");
    const signature = JSON.stringify([intersection.id, phases]);
    if (container.dataset.signature !== signature) {
      container.dataset.signature = signature;
      container.replaceChildren(...phases.map((phase, index) => {
        const card = document.createElement("div"); card.className = "flow-card";
        card.dataset.phaseIndex = index;
        const title = document.createElement("div"); title.textContent = phase.name || `Phase ${index + 1}`;
        const status = document.createElement("small"); card.append(title,status);
        const copy = document.createElement("canvas"); copy.width = 680; copy.height = 520;
        copy.style.width = "100%"; copy.style.height = "auto";
        copy.setAttribute("aria-label", `${intersection.id}: ${title.textContent}`);
        drawJunctionCopy(copy, intersection, phase);
        card.append(copy);return card;
      }));
    }
    [...container.children].forEach(card => {
      const active=signal.stage === "green" && Number(card.dataset.phaseIndex) === signal.phase_index;
      card.classList.toggle("flow-active",active);card.querySelector("small").textContent=active ? "Active phase" : "Waiting";
    });
  }

  function drawJunctionCopy(copy, intersection, phase) {
    const saved = { ctx, city: state.city, snapshot: state.snapshot, transform: state.transform, camera: state.camera };
    try {
      ctx = copy.getContext("2d");
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
      ctx = saved.ctx; state.city = saved.city; state.snapshot = saved.snapshot;
      state.transform = saved.transform; state.camera = saved.camera;
    }
  }

  async function control(action, extras = {}) {
    try { const response = await fetch("/api/control", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ action, ...extras }) }); if (!response.ok) throw new Error(await response.text()); const snapshot = await response.json(); updateSnapshot(snapshot); return snapshot; }
    catch (error) { setConnection("error", "Control request failed"); console.error(error); return null; }
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
    let intersectionMatch = null, intersectionDistance = 18;
    state.city.intersections.forEach((item) => { const p = screen(item.position), d = Math.hypot(p.x - click.x, p.y - click.y); if (d < intersectionDistance) { intersectionMatch = item; intersectionDistance = d; } });
    if (intersectionMatch) state.selected = { type: "intersection", id: intersectionMatch.id };
    else { let personMatch = null, distance = 13; state.snapshot.people.forEach((person) => { const p = screen(person.position), d = Math.hypot(p.x - click.x, p.y - click.y); if (d < distance) { personMatch = person; distance = d; } }); if (personMatch) state.selected = { type: "person", id: personMatch.id }; }
    refreshInspector(); draw();
  });

  $("pause-button").addEventListener("click", () => control(state.snapshot?.paused ? "resume" : "pause")); $("step-button").addEventListener("click", () => control("step", { delta: .05 }));
  $("reset-button").addEventListener("click", () => { state.selected = null; control("reset"); }); $("time-scale").addEventListener("change", (event) => control("set_time_scale", { time_scale: Number(event.target.value) }));
  $("map-preset").addEventListener("change", (event) => switchPreset(event.target.value));
  $("show-transforms").addEventListener("change", (event) => { state.showTransforms = event.target.checked; draw(); });
  $("zoom-in-button").addEventListener("click", () => zoomAt(1.5, state.transform.width / 2, state.transform.height / 2)); $("zoom-out-button").addEventListener("click", () => zoomAt(1 / 1.5, state.transform.width / 2, state.transform.height / 2)); $("fit-button").addEventListener("click", resetView);
  $("fullscreen-button").addEventListener("click", async () => { if (document.fullscreenElement) await document.exitFullscreen(); else await dashboard.requestFullscreen(); });
  const loadControls = [["population-control","population-value",(value) => value],["spawn-rate-control","spawn-rate-value",(value) => `${Number(value).toFixed(1)}/s`],["max-population-control","max-population-value",(value) => value]];
  loadControls.forEach(([controlID, valueID, format]) => $(controlID).addEventListener("input", (event) => { $(valueID).textContent = format(event.target.value); }));
  $("safe-gap-control").addEventListener("input",(event)=>{const safeGap=Number(event.target.value);$("safe-gap-value").textContent=`${safeGap} m`;control("set_spacing",{safe_gap:safeGap});});
  $("apply-load-button").addEventListener("click", () => {
    const population = Number($("population-control").value), spawnRate = Number($("spawn-rate-control").value), maxPopulation = Math.max(population, Number($("max-population-control").value));
    $("max-population-control").value = maxPopulation; $("max-population-value").textContent = maxPopulation;
    control("set_load", { population, spawn_rate_per_second: spawnRate, max_population: maxPopulation });
  });
  document.addEventListener("fullscreenchange", () => { $("fullscreen-button").textContent = document.fullscreenElement ? "Exit full screen" : "Full screen"; setTimeout(fitCanvas, 0); }); window.addEventListener("resize", fitCanvas);

  loadPresets();
  const events = new EventSource("/api/stream"); events.addEventListener("open", () => setConnection("live", "Live model connected")); events.addEventListener("city", (event) => updateCity(JSON.parse(event.data))); events.addEventListener("snapshot", (event) => updateSnapshot(JSON.parse(event.data))); events.addEventListener("error", () => setConnection("error", "Reconnecting to model"));
})();
