import {
  Field,
  Action,
  Value,
  SettingsSlot,
  Connection,
  useSettings,
} from "./SettingsContext";
export function TrafficSettings() {
  const { scene } = useSettings();
  return (
    <section
      id="traffic-settings"
      className="glass load-overlay"
      hidden={!!scene.snapshot?.demand}
    >
      <div className="panel-heading">
        <div>
          <h2>{"Traffic demand"}</h2>
        </div>
        <span id="load-current">
          <Value id="load-current" fallback="—" />
        </span>
      </div>
      <label className="range-control">
        <span>
          {"Active people "}
          <b id="population-value">
            <Value id="population-value" fallback="320" />
          </b>
        </span>
        <Field
          as="input"
          id="population-control"
          type="number"
          step="20"
        ></Field>
      </label>
      <label className="range-control">
        <span>
          {"New people / min "}
          <b id="spawn-rate-value">
            <Value id="spawn-rate-value" fallback="0/min" />
          </b>
        </span>
        <Field
          as="input"
          id="spawn-rate-control"
          type="number"
          step="1"
        ></Field>
      </label>
      <label className="range-control">
        <span>
          {"People limit "}
          <b id="max-population-value">
            <Value id="max-population-value" fallback="1000" />
          </b>
        </span>
        <Field
          as="input"
          id="max-population-control"
          type="number"
          step="100"
        ></Field>
      </label>
      <label className="range-control">
        <span>
          {"Following clearance "}
          <b id="safe-gap-value">
            <Value id="safe-gap-value" fallback="2.5 m" />
          </b>
        </span>
        <Field
          as="input"
          id="safe-gap-control"
          type="number"
          step="0.5"
        ></Field>
      </label>
      <Action id="apply-load-button" type="button" className="apply-load">
        {"Apply traffic load"}
      </Action>
    </section>
  );
}
