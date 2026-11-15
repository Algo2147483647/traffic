import { useAppState, ui, emit } from "../state/store";
import { MetroInspector } from "./MetroInspector";
import { PersonInspector, OrderInspector } from "./PersonInspector";
import { SignalInspector, InterchangeInspector } from "./JunctionInspector";
export function Inspector() {
  const { scene } = useAppState(),
    type = scene.selected?.type;
  return (
    <aside className={"right-stack " + (ui.collapsed ? "collapsed" : "")}>
      <button
        id="inspector-toggle"
        className="quiet"
        aria-label={ui.collapsed ? "Expand details" : "Collapse details"}
        aria-expanded={!ui.collapsed}
        title={ui.collapsed ? "Expand details" : "Collapse details"}
        onClick={() => {
          ui.collapsed = !ui.collapsed;
          emit();
        }}
      >
        {ui.collapsed ? "‹" : "›"}
      </button>
      <section className="glass inspector-overlay">
        {!type && (
          <div id="empty-inspector" className="empty-state">
            Select a vehicle or junction
          </div>
        )}
        {type === "metro-station" && <MetroInspector />}
        {type === "person" && <PersonInspector />}
        {type === "order" && <OrderInspector />}
        {type === "intersection" && <SignalInspector />}
        {type === "interchange" && <InterchangeInspector />}
      </section>
    </aside>
  );
}
