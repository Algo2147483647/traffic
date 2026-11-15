import {
  Field,
  Action,
  Value,
  SettingsSlot,
  Connection,
  useSettings,
} from "./SettingsContext";
export function MetroSettings() {
  const { scene } = useSettings();
  return (
    <section id="metro-settings">
      <h3>{"Metro operation"}</h3>
      <label>
        <Field as="input" id="metro-enabled" type="checkbox"></Field>
        {" Run metro"}
      </label>
      <Field as="input" id="metro-headway" type="hidden"></Field>
      <label className="select-control">
        {"Passengers per minute"}
        <Field as="input" id="metro-demand" type="number"></Field>
      </label>
      <label className="select-control">
        {"Cars per train"}
        <Field as="input" id="metro-cars" type="number"></Field>
      </label>
      <label className="select-control">
        {"Maximum speed (km/h)"}
        <Field as="input" id="metro-speed" type="number"></Field>
      </label>
      <p>
        {"One dot = one passenger. Car counts change at an empty terminal."}
      </p>
      <label className="select-control">
        {"Train capacity"}
        <Field as="input" id="metro-capacity" type="number"></Field>
      </label>
      <label className="select-control">
        {"Minimum dwell (s)"}
        <Field as="input" id="metro-dwell" type="number"></Field>
      </label>
      <SettingsSlot id="metro-levels" />
      <Action id="metro-apply">{"Apply metro settings"}</Action>
      <p id="metro-summary">
        <Value id="metro-summary" fallback="" />
      </p>
      <SettingsSlot id="metro-trains" />
    </section>
  );
}
