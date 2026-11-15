import { useEffect } from "react";
import { useAppState, scene, emit } from "../state/store";
const dbm = (w: number) => (w > 0 ? (10 * Math.log10(w) + 30).toFixed(1) : "—");
export function RadioInspector() {
  useAppState();
  const layout = scene.city?.communications,
    v = scene.snapshot?.communications,
    id = scene.selected?.id;
  const site = layout?.sites.find((s) => s.id === id),
    ue = v?.terminals.find((u) => u.id === scene.radioTerminal);
  useEffect(() => {
    if (!v || !site) return;
    const first =
      v.terminals.find((u) => u.neighbours[0]?.site_id === site.id) ||
      v.terminals[0];
    scene.radioTerminal = first?.id || "";
    emit();
  }, [id]);
  if (!site || !v) return null;
  return (
    <div id="radio-inspector">
      <h2>
        {site.id} / {site.name}
      </h2>
      <p className="radio-note">
        {v.model} v{v.model_version} · simplified baseline
        <br />
        Sample {(v.time_us / 1e6).toFixed(2)} s · run {v.stamp.run}
      </p>
      <dl className="person-facts">
        <div>
          <dt>Ground / mast / antenna</dt>
          <dd>
            {site.position_m.z} / {site.mast_height_m} /{" "}
            {site.position_m.z + site.mast_height_m} m
          </dd>
        </div>
        <div>
          <dt>Coverage radius</dt>
          <dd>{site.coverage_radius_m} m</dd>
        </div>
        <div>
          <dt>Position</dt>
          <dd>
            {site.position_m.x.toFixed(1)}, {site.position_m.y.toFixed(1)} m
          </dd>
        </div>
      </dl>
      <div className="radio-table">
        <table>
          <thead>
            <tr>
              <th>Cell</th>
              <th>GHz</th>
              <th>MHz</th>
              <th>dBm</th>
              <th>UEs</th>
            </tr>
          </thead>
          <tbody>
            {site.sectors.map((c) => (
              <tr key={c.id}>
                <td>{c.id}</td>
                <td>{c.frequency_hz / 1e9}</td>
                <td>{c.bandwidth_hz / 1e6}</td>
                <td>{dbm(c.transmit_power_w)}</td>
                <td>{v.cells.find((x) => x.id === c.id)?.associated || 0}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <label className="select-control">
        Mobile terminal
        <select
          id="radio-terminal"
          value={scene.radioTerminal}
          onChange={(e) => {
            scene.radioTerminal = e.target.value;
            emit();
          }}
        >
          <option value="">Choose a terminal</option>
          {v.terminals.map((u) => (
            <option key={u.id} value={u.id}>
              {u.id} · {u.kind}
              {u.serving_cell ? " · " + u.serving_cell : ""}
            </option>
          ))}
        </select>
      </label>
      {ue ? (
        <>
          <p id="radio-terminal-status">
            {ue.status} · {ue.kind}
            <br />
            Antenna {ue.position_m.x.toFixed(1)}, {ue.position_m.y.toFixed(1)},{" "}
            {ue.position_m.z.toFixed(1)} m<br />
            {ue.speed_mps.toFixed(1)} m/s
            {ue.carrier && (
              <>
                <br />
                Carrier {ue.carrier}
              </>
            )}
          </p>
          <p>
            Serving: {ue.serving_cell || "None"} · threshold{" "}
            {dbm(layout.profile.receiver_sensitivity_w)} dBm
          </p>
          <div className="radio-table">
            <table id="radio-links">
              <thead>
                <tr>
                  <th>Cell</th>
                  <th>m</th>
                  <th>Rx dBm</th>
                  <th>SNR dB</th>
                </tr>
              </thead>
              <tbody>
                {ue.neighbours.map((l) => (
                  <tr
                    key={l.cell_id}
                    className={
                      l.cell_id === ue.serving_cell ? "radio-serving" : ""
                    }
                  >
                    <td>{l.cell_id}</td>
                    <td>{l.distance_m.toFixed(0)}</td>
                    <td>{dbm(l.received_power_w)}</td>
                    <td>{l.snr_db.toFixed(1)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      ) : (
        <p>No active terminal selected.</p>
      )}
      <p className="radio-note">
        SNR includes thermal noise only. No interference, throughput, protocol
        handover or underground propagation is simulated. Selection uses
        strongest received power above threshold.
      </p>
    </div>
  );
}
