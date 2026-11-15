import { useAppState, scene, emit, select } from "../state/store";
import { exportCommunications, report } from "../api/client";
export function RadioSettings() {
  const { scene } = useAppState(),
    layout = scene.city?.communications,
    v = scene.snapshot?.communications;
  if (!layout) return null;
  return (
    <section id="radio-settings">
      <h3>Communications</h3>
      <label>
        <input
          id="show-radio"
          type="checkbox"
          checked={scene.showRadio}
          onChange={(e) => {
            scene.showRadio = e.target.checked;
            emit();
          }}
        />{" "}
        Base stations
      </label>
      <p className="radio-note">
        256 m nominal range. Heat shows associated users per site, not resource
        utilization.
      </p>
      <label className="select-control">
        Inspect site
        <select
          id="radio-site-select"
          value={
            scene.selected?.type === "radio-site"
              ? String(scene.selected.id)
              : ""
          }
          onChange={(e) => {
            if (e.target.value)
              select({ type: "radio-site", id: e.target.value });
          }}
        >
          <option value="">Choose a site</option>
          {layout.sites.map((s) => (
            <option key={s.id} value={s.id}>
              {s.id} · {s.name}
            </option>
          ))}
        </select>
      </label>
      <p id="radio-summary">
        {layout.sites.length} sites · {v?.cells.length || 0} cells ·{" "}
        {v?.connected || 0} connected · {v?.outage || 0} below threshold ·{" "}
        {v?.unsupported || 0} outside model
      </p>
      <button
        id="radio-export"
        className="quiet"
        onClick={() => void exportCommunications().catch(report)}
      >
        Export mobility and links
      </button>
    </section>
  );
}
