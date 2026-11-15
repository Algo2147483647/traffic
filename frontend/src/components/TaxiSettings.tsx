import {
  Field,
  Action,
  Value,
  SettingsSlot,
  Connection,
  useSettings,
} from "./SettingsContext";
export function TaxiSettings() {
  const { scene } = useSettings();
  return (
    <section id="taxi-settings" hidden={!scene.snapshot?.taxi}>
      <div className="settings-section-heading">
        <h3>{"Taxi service"}</h3>
        <span id="taxi-fleet-status">
          <Value id="taxi-fleet-status" fallback="" />
        </span>
      </div>
      <div className="taxi-live-fields">
        <label htmlFor="taxi-fleet">{"Drivers"}</label>
        <Field
          as="input"
          id="taxi-fleet"
          type="number"
          step="1"
          aria-label="Driver count"
        ></Field>
        <Field
          as="input"
          hidden=""
          id="taxi-fleet-range"
          type="number"
          step="1"
          aria-label="Driver count slider"
        ></Field>
        <label htmlFor="taxi-demand">{"Shared trips per minute"}</label>
        <Field
          as="input"
          id="taxi-demand"
          type="number"
          step="0.5"
          aria-label="Order frequency"
        ></Field>
        <Field
          as="input"
          hidden=""
          id="taxi-demand-range"
          type="number"
          step="0.5"
          aria-label="Order frequency slider"
        ></Field>
      </div>
      <Action id="taxi-load-apply" className="apply-load">
        {"Apply live"}
      </Action>
      <p id="taxi-load-feedback" role="status">
        <Value id="taxi-load-feedback" fallback="" />
      </p>
      <section className="taxi-advanced">
        <h3>{"Dispatch & experiment"}</h3>
        <label className="select-control">
          {"Algorithm"}
          <Field as="select" id="taxi-algorithm">
            <option value="nearest">{"Nearest ETA"}</option>
          </Field>
        </label>
        <div className="taxi-fields">
          <label>
            {"Seed"}
            <Field as="input" id="taxi-seed" type="number"></Field>
          </label>
          <label>
            {"Dispatch interval (s)"}
            <Field as="input" id="taxi-interval" type="number"></Field>
          </label>
        </div>
        <label className="range-control">
          {"Algorithm parameters (JSON)"}
          <Field as="textarea" id="taxi-parameters" rows="3"></Field>
        </label>
        <div className="settings-action-row">
          <Action id="taxi-apply">{"Apply taxi settings"}</Action>
          <Action id="taxi-export" className="quiet">
            {"Export run"}
          </Action>
        </div>
      </section>
      <p id="taxi-error" role="status">
        <Value id="taxi-error" fallback="" />
      </p>
    </section>
  );
}
