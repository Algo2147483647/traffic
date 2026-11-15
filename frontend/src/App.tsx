import { useEffect } from "react";
import { connect } from "./api/client";
import { emit } from "./state/store";
import { MapCanvas } from "./components/MapCanvas";
import { SettingsButton } from "./components/SettingsButton";
import { SettingsWindow } from "./components/SettingsWindow";
import { RailServiceWindow } from "./components/RailServiceWindow";
import { Statistics } from "./components/Statistics";
import { Inspector } from "./components/Inspector";
export function App() {
  useEffect(connect, []);
  useEffect(() => {
    document.addEventListener("fullscreenchange", emit);
    return () => document.removeEventListener("fullscreenchange", emit);
  }, []);
  return (
    <main className="dashboard">
      <MapCanvas />
      <SettingsButton />
      <SettingsWindow />
      <RailServiceWindow />
      <Statistics />
      <Inspector />
    </main>
  );
}
