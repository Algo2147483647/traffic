import {
  Field,
  Action,
  Value,
  SettingsSlot,
  Connection,
  useSettings,
} from "./SettingsContext";
export function DemandSettings() {
  const { scene } = useSettings();
  return (
    <section id="shared-demand-settings">
      <h3>{"Shared travel demand"}</h3>
      <label className="select-control">
        {"Trips per minute"}
        <Field as="input" id="shared-rate" type="number" step="0.5"></Field>
      </label>
      <label className="range-control">
        <span>
          {"Taxi share "}
          <b id="shared-share-value">
            <Value id="shared-share-value" fallback="50%" />
          </b>
        </span>
        <Field as="input" id="shared-share" type="number" step="5"></Field>
      </label>
      <label className="select-control">
        {"Metro share (%)"}
        <Field as="input" id="shared-metro" type="number" step="5"></Field>
      </label>
      <Action id="shared-apply">{"Apply demand mix"}</Action>
      <p id="shared-summary" role="status">
        <Value id="shared-summary" fallback="" />
      </p>
    </section>
  );
}
