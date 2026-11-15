import {
  Field,
  Action,
  Value,
  SettingsSlot,
  Connection,
  useSettings,
} from "./SettingsContext";
export function AlgorithmSettings() {
  const { scene } = useSettings();
  return (
    <section id="algorithm-settings">
      <h3>{"Experimental algorithms"}</h3>
      <label className="select-control">
        {"Demand timing and destinations"}
        <Field as="select" id="demand-algorithm">
          <option value="native">{"Native"}</option>
        </Field>
      </label>
      <label className="select-control">
        {"Empty taxi supply"}
        <Field as="select" id="supply-algorithm">
          <option value="native">{"Native"}</option>
        </Field>
      </label>
      <label className="select-control">
        {"Road motion"}
        <Field as="select" id="motion-algorithm">
          <option value="native">{"Native"}</option>
        </Field>
      </label>
      <label className="select-control">
        {"Metro operation"}
        <Field as="select" id="metro-algorithm">
          <option value="native">{"Native"}</option>
        </Field>
      </label>
      <label className="select-control">
        {"Route planner"}
        <Field as="select" id="route-algorithm">
          <option value="native">{"Native"}</option>
        </Field>
      </label>
      <label className="select-control">
        {"Signal scheduler"}
        <Field as="select" id="signal-algorithm">
          <option value="native">{"Native"}</option>
        </Field>
      </label>
      <label className="select-control">
        {"Parameters (JSON)"}
        <Field as="textarea" id="road-algorithm-parameters"></Field>
      </label>
      <Action id="road-algorithm-apply">{"Apply algorithms"}</Action>
      <p id="road-algorithm-status" role="status">
        <Value id="road-algorithm-status" fallback="" />
      </p>
    </section>
  );
}
