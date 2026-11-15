import { RadioSettings } from "./RadioSettings";
import {
  Field,
  Action,
  Value,
  SettingsSlot,
  Connection,
  useSettings,
} from "./SettingsContext";
import { FloatingWindow } from "./FloatingWindow";
import { SettingsProvider } from "./SettingsContext";
import { useAppState, ui, emit } from "../state/store";
import { MetroSettings } from "./MetroSettings";
import { DemandSettings } from "./DemandSettings";
import { TaxiSettings } from "./TaxiSettings";
import { TrafficSettings } from "./TrafficSettings";
import { AlgorithmSettings } from "./AlgorithmSettings";
export function SettingsWindow() {
  const { ui } = useAppState();
  return (
    <SettingsProvider>
      <FloatingWindow
        id="settings-panel"
        title="Settings"
        open={ui.settings}
        close={() => {
          ui.settings = false;
          emit();
        }}
        initial={{ x: 88, y: 68, width: 460, height: 650 }}
      >
        <div className="settings-map-grid">
          <label className="select-control preset-control">
            {"Map"}
            <Field as="select" id="map-preset" aria-label="Map preset">
              <option>{"Loading maps…"}</option>
            </Field>
          </label>
          <label className="select-control">
            {"Demand mix"}
            <Field as="select" id="simulation-mode">
              <option value="traffic">{"Traffic"}</option>
              <option value="taxi">{"Taxi"}</option>
              <option value="mixed">{"Road + Taxi + Metro"}</option>
            </Field>
          </label>
          <label className="select-control">
            {"Height view"}
            <Field as="select" id="layer-filter">
              <option value="all">{"All elevations"}</option>
              <option value="slice">{"Height slice"}</option>
            </Field>
          </label>
          <label className="select-control">
            {"From (m)"}
            <Field as="input" id="height-min" type="number" step="any"></Field>
          </label>
          <label className="select-control">
            {"To (m)"}
            <Field as="input" id="height-max" type="number" step="any"></Field>
          </label>
          <p id="height-feedback" role="status">
            <Value id="height-feedback" fallback="" />
          </p>
        </div>
        <div className="simulation-controls" aria-label="Simulation controls">
          <Action id="pause-button" type="button">
            {"Pause"}
          </Action>
          <Action id="step-button" type="button" className="quiet">
            {"Step"}
          </Action>
          <Action id="reset-button" type="button" className="quiet">
            {"Reset"}
          </Action>
          <label className="select-control">
            {"Rate"}
            <Field as="input" id="time-scale" type="number" step="any"></Field>
          </label>
        </div>
        <div className="settings-view-controls">
          <Action
            id="zoom-out-button"
            type="button"
            className="quiet icon-button"
            aria-label="Zoom out"
          >
            {"−"}
          </Action>
          <span id="view-scale" className="view-scale">
            <Value id="view-scale" fallback="100%" />
          </span>
          <Action
            id="zoom-in-button"
            type="button"
            className="quiet icon-button"
            aria-label="Zoom in"
          >
            {"+"}
          </Action>
          <Action id="fit-button" type="button" className="quiet">
            {"Fit map"}
          </Action>
          <Action id="fullscreen-button" type="button" className="quiet">
            {"Full screen"}
          </Action>
          <label className="coordinate-control">
            <Field as="input" id="show-coordinates" type="checkbox"></Field>
            {" Show coordinates and scale"}
          </label>
        </div>
        <label className="range-control">
          <span>
            {"Walking speed "}
            <b id="walk-speed-value">
              <Value id="walk-speed-value" fallback="1.4 m/s" />
            </b>
          </span>
          <Field
            as="input"
            id="walk-speed-control"
            type="number"
            step="0.1"
          ></Field>
        </label>
        <RadioSettings />
        <MetroSettings />
        <DemandSettings />
        <AlgorithmSettings />
        <TaxiSettings />
        <TrafficSettings />
        <p id="settings-feedback" role="status">
          <Value id="settings-feedback" fallback="" />
        </p>
        <div className="settings-status">
          <Connection />
          <div
            className="frequency-block"
            title="Measured simulation updates per second"
          >
            <strong id="compute-rate">
              <Value id="compute-rate" fallback="— Hz" />
            </strong>
          </div>
        </div>
      </FloatingWindow>
    </SettingsProvider>
  );
}
