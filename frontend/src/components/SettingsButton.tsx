import { ui, emit, useAppState } from "../state/store";
export function SettingsButton() {
  useAppState();
  return (
    <button
      aria-controls="settings-panel"
      aria-expanded={ui.settings}
      onClick={() => {
        ui.settings = !ui.settings;
        emit();
      }}
      aria-label="Settings"
      className="settings-toggle quiet"
      id="settings-toggle"
      title="Settings"
    >
      <svg aria-hidden="true" height="24" viewBox="0 0 24 24" width="24">
        <polygon
          fill="none"
          points="19.00,12.00 20.83,13.76 20.31,15.44 17.82,15.89 16.95,16.95 17.00,19.48 15.44,20.31 13.37,18.87 12.00,19.00 10.24,20.83 8.56,20.31 8.11,17.82 7.05,16.95 4.52,17.00 3.69,15.44 5.13,13.37 5.00,12.00 3.17,10.24 3.69,8.56 6.18,8.11 7.05,7.05 7.00,4.52 8.56,3.69 10.63,5.13 12.00,5.00 13.76,3.17 15.44,3.69 15.89,6.18 16.95,7.05 19.48,7.00 20.31,8.56 18.87,10.63"
          stroke="currentColor"
          strokeLinejoin="round"
          strokeWidth="1.5"
        ></polygon>
        <circle
          cx="12"
          cy="12"
          fill="none"
          r="3"
          stroke="currentColor"
          strokeWidth="2"
        ></circle>
      </svg>
    </button>
  );
}
