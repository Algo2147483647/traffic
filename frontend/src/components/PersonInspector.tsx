import { useAppState } from "../state/store";
export function PersonInspector() {
  const { scene } = useAppState(),
    p = scene.snapshot?.people.find((p) => p.id === scene.selected?.id);
  if (!p) return null;
  const taxi = scene.snapshot.taxi?.vehicles.find((v) => v.id === p.id),
    xy = (v, d = 1) => `${v.x.toFixed(d)}, ${v.y.toFixed(d)}`;
  const facts = [
    [
      "way",
      "Way / lane",
      p.way_id
        ? `${p.way_id} / ${p.lane + 1}${p.lane_change ? ` → L${p.lane_change.to_lane + 1} (${Math.round(p.lane_change.progress * 100)}%, ${p.lane_change.reason})` : ""}`
        : "on foot",
    ],
    ["reference", "Reference", p.reference.intersection_id],
    ["position", "World", xy(p.position)],
    ["origin", "Origin", xy(p.origin, 0)],
    ["destination", "Destination", xy(p.destination, 0)],
    ["route", "Route", p.route.join(" → ")],
  ];
  return (
    <div id="person-inspector">
      <div className="panel-heading">
        <div>
          <p>PERSON / VEHICLE</p>
          <h2 id="person-id">
            {taxi ? "T" : "P"}-{String(p.id).padStart(3, "0")}
          </h2>
        </div>
        <span id="person-state">
          {p.state.replaceAll("_", " ").toUpperCase()}
        </span>
      </div>
      <dl className="person-facts">
        {facts.map(([id, label, value]) => (
          <div key={id}>
            <dt>{label}</dt>
            <dd id={"person-" + id}>{value}</dd>
          </div>
        ))}
      </dl>
      <p id="taxi-person-detail">
        {taxi
          ? `${taxi.state} · ${taxi.order_id ? "Order #" + taxi.order_id : "Available"} · Completed ${taxi.completed}`
          : ""}
      </p>
      <p className="matrix-label">LOCAL TRANSFORM</p>
      <div id="person-matrix" className="matrix">
        {p.reference.local_transform.flat().map((v, i) => (
          <span key={i}>{v.toFixed(2)}</span>
        ))}
      </div>
    </div>
  );
}
export function OrderInspector() {
  const { scene } = useAppState(),
    o = scene.snapshot?.taxi?.orders.find((o) => o.id === scene.selected?.id);
  if (!o) return null;
  const values = [
    ["Taxi", o.vehicle_id ? `T-${o.vehicle_id}` : "Unassigned"],
    ["Pickup", o.pickup.replace("stop:", "")],
    ["Destination", o.destination.replace("stop:", "")],
    ["Requested", o.created_s.toFixed(1) + " s"],
    ["Pickup estimate", o.pickup_eta_s.toFixed(1) + " s (free flow)"],
    [
      "Wait",
      (["InService", "Completed", "Boarding"].includes(o.state)
        ? o.picked_s - o.created_s
        : scene.snapshot.time - o.created_s
      ).toFixed(1) + " s",
    ],
  ];
  return (
    <div id="order-inspector">
      <div className="panel-heading">
        <h2 id="order-title">Order #{o.id}</h2>
        <span id="order-state">{o.state}</span>
      </div>
      <dl id="order-facts" className="person-facts">
        {values.map(([k, v]) => (
          <div key={k}>
            <dt>{k}</dt>
            <dd>{v}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
