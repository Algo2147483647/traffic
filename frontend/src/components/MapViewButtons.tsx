import { useAppState, setMapView } from "../state/store";
export function MapViewButtons() {
  const { scene } = useAppState();
  return (
    <nav className="map-view-buttons" aria-label="Map view">
      {(["all", "traffic", "communications"] as const).map((view, i) => (
        <button
          key={view}
          id={"view-" + view}
          title={["All", "Traffic", "Communications"][i]}
          aria-label={["All", "Traffic", "Communications"][i]}
          aria-pressed={scene.mapView === view}
          onClick={() => setMapView(view)}
        >
          <svg
            viewBox="0 0 24 24"
            aria-hidden="true"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
          >
            {view === "all" ? (
              <>
                <path d="m3 8 9-5 9 5-9 5Z" />
                <path d="m3 12 9 5 9-5M3 16l9 5 9-5" />
              </>
            ) : view === "traffic" ? (
              <>
                <path d="M8 3 4 21M16 3l4 18M12 3v4m0 3v4m0 3v4" />
              </>
            ) : (
              <>
                <circle cx="12" cy="9" r="2" />
                <path d="M12 11v10M8 21h8M7 5a6 6 0 0 0 0 8M17 5a6 6 0 0 1 0 8M4 2a10 10 0 0 0 0 14M20 2a10 10 0 0 1 0 14" />
              </>
            )}
          </svg>
        </button>
      ))}
    </nav>
  );
}
