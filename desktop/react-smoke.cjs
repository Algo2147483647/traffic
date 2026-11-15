// Runs inside the real Electron renderer. No React/store internals are accessed.
module.exports = async function reactSmoke() {
  const wait = (ms) => new Promise((r) => setTimeout(r, ms)),
    errors = [];
  window.addEventListener("error", (e) => errors.push(e.message));
  window.addEventListener("unhandledrejection", (e) =>
    errors.push(String(e.reason)),
  );
  const assert = (v, m) => {
      if (!v) throw Error(m);
    },
    q = (s) => document.querySelector(s);
  const get = (p) => fetch(p).then((r) => r.json());
  const control = async (body) => {
    const r = await fetch("/api/control", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!r.ok) throw Error(await r.text());
    return r.json();
  };
  const until = async (test, label) => {
    for (let i = 0; i < 120; i++) {
      if (await test()) return;
      await wait(50);
    }
    throw Error(label);
  };
  const click = async (s) => {
    assert(q(s), "Missing " + s);
    q(s).click();
    await wait(80);
  };
  const fill = async (s, value) => {
    const e = q(s);
    assert(e, "Missing " + s);
    e.focus();
    const p =
      e.tagName === "SELECT"
        ? HTMLSelectElement.prototype
        : e.tagName === "TEXTAREA"
          ? HTMLTextAreaElement.prototype
          : HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(p, "value").set.call(e, String(value));
    e.dispatchEvent(new Event("input", { bubbles: true }));
    e.dispatchEvent(new Event("change", { bubbles: true }));
    await wait(20);
    e.blur();
    e.dispatchEvent(new FocusEvent("focusout", { bubbles: true }));
    await wait(80);
  };
  await until(
    () => q("#map-preset")?.options.length === 2,
    "React did not mount/load presets",
  );
  await click("#settings-toggle");
  assert(!q("#settings-panel").hidden, "Settings open");
  const panel = q("#settings-panel"),
    r = panel.getBoundingClientRect();
  q("#settings-drag").dispatchEvent(
    new PointerEvent("pointerdown", {
      pointerId: 41,
      button: 0,
      clientX: r.left + 60,
      clientY: r.top + 20,
      bubbles: true,
    }),
  );
  window.dispatchEvent(
    new PointerEvent("pointermove", {
      pointerId: 41,
      clientX: r.left + 140,
      clientY: r.top + 65,
    }),
  );
  window.dispatchEvent(new PointerEvent("pointerup", { pointerId: 41 }));
  await wait(80);
  assert(panel.getBoundingClientRect().left > r.left, "Settings drag");
  const before = panel.getBoundingClientRect();
  q("#settings-resize").dispatchEvent(
    new PointerEvent("pointerdown", {
      pointerId: 42,
      button: 0,
      clientX: before.right,
      clientY: before.bottom,
      bubbles: true,
    }),
  );
  window.dispatchEvent(
    new PointerEvent("pointermove", {
      pointerId: 42,
      clientX: before.right + 60,
      clientY: before.bottom + 20,
    }),
  );
  window.dispatchEvent(new PointerEvent("pointerup", { pointerId: 42 }));
  await wait(80);
  assert(panel.getBoundingClientRect().width > before.width, "Settings resize");
  await click("#settings-maximize");
  assert(panel.getBoundingClientRect().width > innerWidth - 20, "Maximize");
  await click("#settings-maximize");
  await control({ action: "pause" });
  await fill("#walk-speed-control", "4.2");
  await until(
    async () => (await get("/api/snapshot")).walk_speed_mps === 4.2,
    "Walking speed",
  );
  await fill("#time-scale", 2);
  await until(
    async () => (await get("/api/snapshot")).time_scale === 2,
    "Time scale",
  );
  await fill("#time-scale", 1);
  await fill("#map-preset", "preset-1");
  await until(
    async () => (await get("/api/presets")).current_preset_id === "preset-1",
    "Map switch",
  );
  await control({ action: "pause" });
  await wait(150);
  const labels = new Map(),
    original = CanvasRenderingContext2D.prototype.fillText;
  CanvasRenderingContext2D.prototype.fillText = function (text, x, y, ...args) {
    if (this.canvas.id === "city-canvas") labels.set(String(text), { x, y });
    return original.call(this, text, x, y, ...args);
  };
  await click("#fit-button");
  assert(labels.has("X · km"), "Coordinate rulers");
  if (q("#show-coordinates").checked) await click("#show-coordinates");
  labels.clear();
  await click("#fit-button");
  assert(!labels.has("X · km"), "Hide rulers");
  await click("#show-coordinates");
  await fill("#layer-filter", "slice");
  await fill("#height-min", -66);
  await fill("#height-max", -49);
  assert(!q("#height-min").disabled, "Height fields");
  await fill("#height-min", 10);
  assert(q("#height-feedback").textContent.length > 0, "Invalid slice range");
  await fill("#height-min", -66);
  await fill("#layer-filter", "all");
  await fill("#taxi-fleet", 6);
  await fill("#taxi-demand", 8);
  await fill("#simulation-mode", "taxi");
  await until(
    async () => (await get("/api/snapshot")).taxi?.config.fleet === 6,
    "Taxi controls",
  );
  await fill("#shared-rate", 60);
  await fill("#shared-share", 30);
  await fill("#shared-metro", 40);
  await click("#shared-apply");
  await until(
    async () => (await get("/api/snapshot")).demand?.config.metro_share === 0.4,
    "Mixed demand",
  );
  await fill("#taxi-fleet", 8);
  await click("#taxi-load-apply");
  await until(
    async () => (await get("/api/snapshot")).taxi.config.fleet === 8,
    "Live fleet",
  );
  await fill("#road-algorithm-parameters", "{invalid");
  await click("#road-algorithm-apply");
  assert(q("#settings-feedback").textContent, "Invalid JSON feedback");
  await fill("#road-algorithm-parameters", "{}");
  await click("#road-algorithm-apply");
  await click("#rail-service-toggle");
  assert(!q("#rail-service-panel").hidden, "Rail page");
  await fill("#rail-service-headway", 900);
  await click("#rail-service-apply");
  await until(
    async () =>
      (await get("/api/snapshot")).metro.config.service?.target_headway_s ===
      900,
    "Timetable reduction",
  );
  await fill("#rail-service-headway", 120);
  await click("#rail-service-apply");
  await until(
    async () =>
      (await get("/api/snapshot")).metro.config.service?.target_headway_s ===
      120,
    "Timetable growth",
  );
  assert(q("#rail-service-summary table"), "Fleet table");
  await click("#rail-service-close");
  // Physical elevation configuration remains editable and is submitted from React draft state.
  await fill("#metro-levels input", -52.25);
  await fill("#metro-speed", 350);
  await click("#metro-apply");
  await until(
    async () =>
      (await get("/api/snapshot")).metro.lines[0].elevation_m === -52.25,
    "Metro elevation",
  );
  const radioBefore = await get("/api/communications");
  assert(
    radioBefore.layout.sites.length === 700 &&
      radioBefore.observation.cells.length === 2100,
    "Radio preset",
  );
  await fill(
    "#radio-site-select",
    radioBefore.layout.sites.find(
      (s) => Math.hypot(s.position_m.x, s.position_m.y) < 256,
    ).id,
  );
  assert(q("#radio-inspector"), "Radio inspector");
  assert(q("#radio-terminal").options.length > 1, "Radio terminals");
  await fill("#radio-terminal", q("#radio-terminal").options[1].value);
  assert(q("#radio-terminal-status").textContent, "Terminal details");
  await click("#view-communications");
  assert(q("#radio-charts svg"), "Radio charts");
  assert(!q("#show-radio-sectors"), "Hex outlines removed");
  await click("#view-traffic");
  assert(
    q("#view-traffic").getAttribute("aria-pressed") === "true",
    "Traffic view",
  );
  await click("#view-all");
  await click("#show-radio");
  await click("#show-radio");
  const radioAfter = await get("/api/communications");
  assert(
    radioAfter.observation.time_us === radioBefore.observation.time_us,
    "Presentation changed clock",
  );
  // Test actual interchange route previews, then ordinary signal phase copies.
  await click("#settings-close");
  await click("#fit-button");
  await wait(100);
  let p = labels.get("Central cloverleaf");
  assert(p, "Cloverleaf label");
  const canvas = q("#city-canvas");
  canvas.dispatchEvent(
    new MouseEvent("click", {
      clientX: p.x - 12,
      clientY: p.y + 28,
      bubbles: true,
    }),
  );
  await wait(100);
  assert(q("#interchange-canvas"), "Interchange inspector");
  await fill("#interchange-movement", 1);
  assert(
    q("#interchange-route-detail").textContent.includes("road segments"),
    "Interchange route details",
  );
  await click("#settings-toggle");
  await fill("#map-preset", "preset-1");
  await wait(250);
  await control({ action: "pause" });
  labels.clear();
  await click("#fit-button");
  await wait(100);
  for (let i = 0; i < 4; i++) await click("#zoom-in-button");
  const signalCity = await get("/api/city");
  const signal = signalCity.intersections.find(
    (n) =>
      n.traffic_control.mode !== "flow" &&
      !n.interchange &&
      n.traffic_control.phases.length > 0 &&
      labels.has(n.id + "  " + n.name.toUpperCase()) &&
      labels.get(n.id + "  " + n.name.toUpperCase()).x > 200 &&
      labels.get(n.id + "  " + n.name.toUpperCase()).x < innerWidth - 500 &&
      labels.get(n.id + "  " + n.name.toUpperCase()).y > 100 &&
      labels.get(n.id + "  " + n.name.toUpperCase()).y < innerHeight - 200,
  );
  assert(
    signal,
    "Visible grid signal: " +
      JSON.stringify({
        labels: [...labels.keys()].slice(-20),
        count: signalCity.intersections.filter(
          (n) => n.traffic_control.phases.length > 0,
        ).length,
        zoom: q("#view-scale")?.textContent,
      }),
  );
  const entry = [
    signal.id,
    labels.get(signal.id + "  " + signal.name.toUpperCase()),
  ];
  p = entry[1];
  canvas.dispatchEvent(
    new MouseEvent("click", {
      clientX: p.x - 13,
      clientY: p.y + 8,
      bubbles: true,
    }),
  );
  await wait(100);
  assert(
    q("#signal-movements canvas") && q("#signal-plan .phase-row"),
    "Signal phase copies",
  );
  await fill("#map-preset", "preset-1");
  await wait(200);
  await control({ action: "pause" });
  await control({
    action: "set_demand",
    demand: {
      trips_per_minute: 60,
      taxi_share: 0,
      metro_share: 1,
      seed: 71429,
    },
  });
  let snap;
  for (let i = 0; i < 180; i++) {
    snap = await control({ action: "step", delta: 4 });
    assert(snap.demand.conserved, "Task conservation");
    assert(
      snap.metro.generated ===
        snap.metro.waiting + snap.metro.onboard + snap.metro.completed,
      "Passenger conservation",
    );
    if (
      snap.metro.transferring > 0 &&
      snap.metro.station_details.some((d) => d.passenger_ids.length > 1)
    )
      break;
  }
  assert(snap.metro.transferring > 0, "Real transfers");
  await wait(150);
  await click("#settings-close");
  labels.clear();
  await click("#fit-button");
  await wait(100);
  await click("#zoom-in-button");
  await wait(100);
  const labelFor = (site) => {
    for (const id of site.stations) {
      const station = snap.metro.stations.find((s) => s.id === id);
      const label = [...labels].find(([k]) =>
        k.startsWith(station.name + " / "),
      );
      if (label) return label[1];
    }
    return null;
  };
  const site = snap.metro.interchanges.find(
    (site) =>
      snap.metro.station_details.find((d) => d.id === site.id)?.passenger_ids
        .length && labelFor(site),
  );
  assert(site, "Exchange with passengers");
  p = labelFor(site);
  p = { x: p.x - 8, y: p.y + 10 };
  const cr = canvas.getBoundingClientRect(),
    dx = cr.width / 2 - p.x,
    dy = cr.height / 2 - p.y;
  canvas.dispatchEvent(
    new PointerEvent("pointerdown", {
      pointerId: 42,
      clientX: cr.width / 2,
      clientY: cr.height / 2,
    }),
  );
  canvas.dispatchEvent(
    new PointerEvent("pointermove", {
      pointerId: 42,
      clientX: cr.width / 2 + dx,
      clientY: cr.height / 2 + dy,
    }),
  );
  canvas.dispatchEvent(new PointerEvent("pointerup", { pointerId: 42 }));
  await wait(100);
  p = { x: cr.width / 2, y: cr.height / 2 };
  canvas.dispatchEvent(
    new MouseEvent("click", { clientX: p.x, clientY: p.y, bubbles: true }),
  );
  await wait(100);
  assert(
    q("#metro-station-name")?.textContent === site.name,
    "Exchange inspector",
  );
  assert(q("#metro-passenger-path li"), "Passenger itinerary");
  await fill(
    "#metro-passenger-select",
    q("#metro-passenger-select").options[0].value,
  );
  await click("#metro-route-fit");
  assert(
    q("#metro-route-legend") && !q("#metro-route-legend").hidden,
    "Route status legend",
  );
  await click("#inspector-toggle");
  assert(
    q(".right-stack").classList.contains("collapsed"),
    "Collapse inspector",
  );
  await click("#inspector-toggle");
  assert(errors.length === 0, errors.join("; "));
  return {
    framework: "React + TypeScript + Canvas",
    presets: 2,
    settings: true,
    interchange: true,
    signalPhases: true,
    transferring: snap.metro.transferring,
    routeSteps: q("#metro-passenger-path").children.length,
    conserved: snap.demand.conserved,
    radioSites: 700,
    radioCells: 2100,
    errors,
  };
};
