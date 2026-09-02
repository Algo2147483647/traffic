"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import TrafficCanvas from "./TrafficCanvas";
import { demoCity, demoSnapshot, type City, type Matrix, type Selection, type Snapshot } from "./simulation";

function MatrixView({ matrix }: { matrix: Matrix }) {
  return <div className="matrix" aria-label="Three by three local transformation matrix">{matrix.flat().map((value, index) => <span key={index}>{value.toFixed(2)}</span>)}</div>;
}

export default function Home() {
  const [city, setCity] = useState<City>(demoCity);
  const [snapshot, setSnapshot] = useState<Snapshot>(() => demoSnapshot(demoCity, 0, false, 1));
  const [mode, setMode] = useState<"connecting" | "live" | "demo">("connecting");
  const [selected, setSelected] = useState<Selection>(null);
  const [showTransforms, setShowTransforms] = useState(false);
  const [zoom, setZoom] = useState(1);
  const [viewCommand, setViewCommand] = useState<{ id: number; type: "in" | "out" | "fit" } | null>(null);
  const [fullscreen, setFullscreen] = useState(false);
  const [population, setPopulation] = useState(demoCity.simulation.people);
  const [spawnRate, setSpawnRate] = useState(demoCity.simulation.spawn_rate_per_second);
  const [maxPopulation, setMaxPopulation] = useState(demoCity.simulation.max_people);
  const [safeGap, setSafeGap] = useState(demoCity.simulation.safe_gap);
  const demoState = useRef({ time: 0, paused: false, scale: 1, population, spawnRate, maxPopulation, safeGap, spawnRemainder: 0 });
  const backend = useRef("");
  const stage = useRef<HTMLElement>(null);

  useEffect(() => {
    const host = window.location.hostname;
    backend.current = host === "localhost" || host === "127.0.0.1" ? "http://127.0.0.1:8080" : "";
    let events: EventSource | null = null, interval = 0, cancelled = false;
    const startDemo = () => {
      if (cancelled) return;
      setMode("demo"); let previous = performance.now();
      interval = window.setInterval(() => {
        const now = performance.now(), dt = Math.min(.2, (now - previous) / 1000); previous = now;
        if (!demoState.current.paused) {
          demoState.current.time += dt * demoState.current.scale;
          demoState.current.spawnRemainder += demoState.current.spawnRate * dt * demoState.current.scale;
          const additions = Math.floor(demoState.current.spawnRemainder);
          if (additions > 0) {
            demoState.current.population = Math.min(demoState.current.maxPopulation, demoState.current.population + additions);
            demoState.current.spawnRemainder -= additions;
          }
        }
        const state = demoState.current;
        setSnapshot(demoSnapshot(demoCity, state.time, state.paused, state.scale, state.population, state.spawnRate, state.maxPopulation, state.safeGap));
      }, 90);
    };
    fetch(`${backend.current}/api/city`).then((response) => {
      if (!response.ok || !(response.headers.get("content-type") || "").includes("json")) throw new Error("No backend");
      return response.json();
    }).then((data: City) => {
      if (cancelled) return;
      setCity(data); setMode("live");
      setPopulation(data.simulation.people); setSpawnRate(data.simulation.spawn_rate_per_second || 0); setMaxPopulation(data.simulation.max_people || data.simulation.people); setSafeGap(data.simulation.safe_gap || 4);
      events = new EventSource(`${backend.current}/api/stream`);
      events.addEventListener("snapshot", (event) => setSnapshot(JSON.parse((event as MessageEvent).data)));
      events.addEventListener("error", () => setMode("connecting"));
    }).catch(startDemo);
    return () => { cancelled = true; events?.close(); window.clearInterval(interval); };
  }, []);

  useEffect(() => { const update = () => setFullscreen(Boolean(document.fullscreenElement)); document.addEventListener("fullscreenchange", update); return () => document.removeEventListener("fullscreenchange", update); }, []);

  const person = useMemo(() => selected?.type === "person" ? snapshot.people.find((item) => item.id === selected.id) || null : null, [snapshot, selected]);
  const intersection = useMemo(() => selected?.type === "intersection" ? city.intersections.find((item) => item.id === selected.id) || null : null, [city, selected]);
  const signal = useMemo(() => intersection ? snapshot.signals?.find((item) => item.intersection_id === intersection.id) || null : null, [intersection, snapshot]);

  const request = async (action: string, extras: Record<string, number> = {}) => {
    if (mode === "live") {
      const response = await fetch(`${backend.current}/api/control`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ action, ...extras }) });
      if (response.ok) setSnapshot(await response.json());
      return;
    }
    if (action === "pause") demoState.current.paused = true;
    if (action === "resume") demoState.current.paused = false;
    if (action === "step") demoState.current.time += extras.delta || .05;
    if (action === "reset") { demoState.current.time = 0; setSelected(null); }
    if (action === "set_time_scale") demoState.current.scale = extras.time_scale;
    if (action === "set_load") {
      demoState.current.population = extras.population;
      demoState.current.spawnRate = extras.spawn_rate_per_second;
      demoState.current.maxPopulation = extras.max_population;
      demoState.current.spawnRemainder = 0;
    }
    if (action === "set_spacing") demoState.current.safeGap = extras.safe_gap;
    const state = demoState.current;
    setSnapshot(demoSnapshot(demoCity, state.time, state.paused, state.scale, state.population, state.spawnRate, state.maxPopulation, state.safeGap));
  };
  const applyLoad = () => {
    const cappedMax = Math.max(population, maxPopulation);
    if (cappedMax !== maxPopulation) setMaxPopulation(cappedMax);
    request("set_load", { population, spawn_rate_per_second: spawnRate, max_population: cappedMax });
  };
  const view = (type: "in" | "out" | "fit") => setViewCommand({ id: Date.now(), type });
  const toggleFullscreen = async () => { if (document.fullscreenElement) await document.exitFullscreen(); else await stage.current?.requestFullscreen(); };
  const clock = `${Math.floor(snapshot.time / 60).toString().padStart(2, "0")}:${(snapshot.time % 60).toFixed(1).padStart(4, "0")}`;

  return <main className="city-stage" ref={stage}>
    <TrafficCanvas city={city} snapshot={snapshot} selected={selected} onSelect={setSelected} transforms={showTransforms} viewCommand={viewCommand} onZoom={setZoom}/>

    <header className="glass top-overlay">
      <div className="brand-lockup"><span className="mark">M</span><div><p className="eyebrow">Traffic field / live city</p><h1>{city.name}</h1></div></div>
      <div className="simulation-controls" aria-label="Simulation controls">
        <div className="clock-block"><span>SIM TIME</span><strong>{clock}</strong></div>
        <button onClick={() => request(snapshot.paused ? "resume" : "pause")}>{snapshot.paused ? "Resume" : "Pause"}</button>
        <button className="quiet" onClick={() => request("step", { delta: .05 })}>Step</button>
        <button className="quiet" onClick={() => request("reset")}>Reset</button>
        <label className="select-control">RATE<select value={snapshot.time_scale} onChange={(event) => request("set_time_scale", { time_scale: Number(event.target.value) })}><option value="0.5">0.5×</option><option value="1">1×</option><option value="2">2×</option><option value="4">4×</option><option value="8">8×</option><option value="10">10×</option></select></label>
        <span className="divider" aria-hidden="true"/>
        <button className="quiet icon-button" aria-label="Zoom out" onClick={() => view("out")}>−</button>
        <button className="quiet icon-button" aria-label="Zoom in" onClick={() => view("in")}>+</button>
        <button className="quiet" onClick={() => view("fit")}>Fit</button>
        <button className="quiet" onClick={toggleFullscreen}>{fullscreen ? "Exit" : "Full screen"}</button>
        <span className="view-scale">{Math.round(zoom * 100)}%</span>
      </div>
      <div className={`connection ${mode}`} role="status"><span className="connection-dot"/><span>{mode === "live" ? "GO LIVE" : mode === "demo" ? "DEMO" : "CONNECTING"}</span></div>
    </header>

    <section className="glass stats-overlay" aria-label="Live traffic statistics">
      <div className="stat-primary"><strong>{snapshot.people.length}</strong><span>population</span></div>
      <div className="stat-row"><span><i className="dot driving"/>Driving <b>{snapshot.stats.driving}</b></span><span><i className="dot waiting"/>Waiting <b>{snapshot.stats.waiting}</b></span><span><i className="dot walking"/>Walking <b>{snapshot.stats.walking}</b></span></div>
      <div className="stat-meta"><span>{snapshot.stats.average_speed.toFixed(1)} avg speed</span><span>{city.ways.length} ways / {city.intersections.length} signals</span></div>
      {snapshot.control_performance && <div className="efficiency-row"><span><b>{snapshot.control_performance.current_signal_queue}</b> signal queue</span><span><b>{snapshot.control_performance.throughput_per_minute.toFixed(1)}</b> trips/min</span><span><b>{snapshot.control_performance.mean_signal_delay_seconds.toFixed(1)}s</b> mean signal delay</span></div>}
    </section>

    <aside className="right-stack">
      <section className="glass load-overlay">
        <div className="panel-heading"><div><p>TRAFFIC LOAD</p><h2>Population density</h2></div><span>{snapshot.load?.population ?? snapshot.people.length}</span></div>
        <label className="range-control"><span>Active population <b>{population}</b></span><input type="range" min="20" max="1200" step="20" value={population} onChange={(event) => setPopulation(Number(event.target.value))}/></label>
        <label className="range-control"><span>Spawn rate <b>{spawnRate.toFixed(1)}/s</b></span><input type="range" min="0" max="20" step="0.5" value={spawnRate} onChange={(event) => setSpawnRate(Number(event.target.value))}/></label>
        <label className="range-control"><span>Population cap <b>{maxPopulation}</b></span><input type="range" min="100" max="2000" step="100" value={maxPopulation} onChange={(event) => setMaxPopulation(Number(event.target.value))}/></label>
        <label className="range-control"><span>Vehicle spacing <b>{safeGap.toFixed(0)} m</b></span><input type="range" min="4" max="80" step="2" value={safeGap} onChange={(event) => { const value=Number(event.target.value); setSafeGap(value); request("set_spacing",{safe_gap:value}); }}/></label>
        <button className="apply-load" onClick={applyLoad}>Apply traffic load</button>
      </section>

      {selected && <section className="glass inspector-overlay">
        <button className="close-inspector" aria-label="Close inspector" onClick={() => setSelected(null)}>×</button>
        {person && <div><div className="panel-heading"><div><p>PERSON / VEHICLE</p><h2>P-{String(person.id).padStart(3, "0")}</h2></div><span>{person.state.replaceAll("_", " ")}</span></div><dl className="person-facts"><div><dt>Way / lane</dt><dd>{person.way_id ? `${person.way_id} / ${person.lane + 1}` : "on foot"}</dd></div>{person.lane_change && <div><dt>Lane change</dt><dd>L{person.lane_change.from_lane+1} → L{person.lane_change.to_lane+1} · {Math.round(person.lane_change.progress*100)}% · {person.lane_change.reason}</dd></div>}<div><dt>Reference</dt><dd>{person.reference.intersection_id}</dd></div><div><dt>Origin</dt><dd>{person.origin.x.toFixed(0)}, {person.origin.y.toFixed(0)}</dd></div><div><dt>Destination</dt><dd>{person.destination.x.toFixed(0)}, {person.destination.y.toFixed(0)}</dd></div><div><dt>Route</dt><dd className="route-value">{person.route.join(" → ")}</dd></div></dl><p className="matrix-label">LOCAL TRANSFORM</p><MatrixView matrix={person.reference.local_transform}/></div>}
        {intersection && signal && <div><div className="panel-heading"><div><p>SIGNAL PLAN / {intersection.id}</p><h2>{intersection.name}</h2></div><span className="signal-countdown">{signal.remaining_seconds.toFixed(1)}s</span></div><p className="active-phase"><i/>Now: {signal.phase_name} · {(signal.stage || "green").replace("_"," ")}</p><p className="next-phase">Next: {signal.next_phase_name || intersection.traffic_control.phases[(signal.phase_index + 1) % intersection.traffic_control.phases.length]?.name}</p><p className="next-phase">{intersection.traffic_control.planning_basis}</p><div className="controller-card"><div><span>Controller</span><strong>{signal.controller}</strong></div><div><span>Decision</span><strong>{signal.decision}</strong></div><div><span>Green target</span><strong>{signal.base_green_seconds.toFixed(1)} → {signal.target_green_seconds.toFixed(1)}s</strong></div><div><span>Pressure</span><strong>{signal.queue_pressure.toFixed(1)} / {signal.competing_pressure.toFixed(1)}</strong></div></div><div className="signal-plan">{intersection.traffic_control.phases.map((phase, index) => <div className={`phase-row ${index === signal.phase_index ? "active" : ""}`} key={phase.id}><i/><span>{phase.name}</span><strong>{phase.duration_seconds}s · {phase.min_green_seconds ?? phase.duration_seconds}–{phase.max_green_seconds ?? phase.duration_seconds}s</strong></div>)}</div><p className="matrix-label">PERMITTED MOVEMENTS</p><div className="movement-list">{(signal.allowed_movements || []).map((movement) => <span key={`${movement.from_way_id}-${movement.to_way_id}`}>{movement.turn || "flow"} · {movement.from_way_id} L{(movement.from_lanes || []).map((lane)=>lane+1).join(",")} → {movement.to_way_id} L{(movement.to_lanes || []).map((lane)=>lane+1).join(",")} · {(movement.lane_connections || []).length} paths</span>)}</div></div>}
      </section>}
    </aside>

    <div className="glass map-key" aria-label="Map legend"><span><i className="dot driving"/>Vehicle</span><span><i className="dot walking"/>On foot</span><span><i className="dot waiting"/>Stopped</span><span><i className="flow-line"/>Permitted turn</span><label><input type="checkbox" checked={showTransforms} onChange={(event) => setShowTransforms(event.target.checked)}/>Axes</label></div>
    <p className="map-help">Drag to pan · wheel or pinch to zoom · select a vehicle or junction</p>
  </main>;
}
