# React, TypeScript and Canvas frontend

The active desktop frontend is `frontend/src`, built by Vite. It replaces the old monolithic `frontend/app.js`; there is no legacy script loaded alongside React and no raw HTML injection wrapper.

## Responsibilities

```text
frontend/
  index.html                         React mount point only
  assets.go                          Embeds dist into the Go desktop backend
  styles.css                         Preserved application appearance
  vite.config.ts                     Static production build and API development proxy
  tsconfig.json                      Frontend TypeScript checking
  src/
    main.tsx                         React root
    App.tsx                          Application composition and connection lifetime
    api/
      models.ts                      Generated Go JSON response types
      client.ts                      HTTP controls, serialized requests, SSE/reconnect, export
    state/
      store.ts                       Backend observations, selection, camera and UI state
      defaults.ts                    Initial form values
      settingsActions.ts             Validated form intent -> backend command / view action
    interaction/
      mapGestures.ts                 Pan, wheel/pinch zoom, hit testing, selection, cleanup
    rendering/
      renderer.ts                    Canvas geometry and drawing; no HTTP or DOM queries
      controller.ts                  Canvas command reference and visible route-fit extent
      heightSlice.ts                 Physical ramp clipping
      metroRoute.ts                  Directional track itinerary geometry and route status
    components/
      MapCanvas.tsx                  Canvas lifecycle, resize observer, frame coalescing
      FloatingWindow.tsx             Drag, resize, maximize, keyboard and saved window bounds
      SettingsButton.tsx             Settings icon
      SettingsWindow.tsx             Settings composition
      SettingsContext.tsx            Draft form ownership and derived status values
      MetroSettings.tsx              Train physical/operational settings
      DemandSettings.tsx             Shared three-mode demand controls
      AlgorithmSettings.tsx          Experimental algorithms and JSON parameters
      TaxiSettings.tsx               Live fleet, order rate and dispatch settings
      TrafficSettings.tsx            Population, arrivals and following clearance
      RailServiceWindow.tsx          Planned fleet and actual service trips
      Statistics.tsx                 Population/time, traffic and taxi metrics
      Inspector.tsx                  Selection-driven inspector composition
      MetroInspector.tsx             Direction queues, ETAs, transfers and passenger itinerary
      PersonInspector.tsx            Vehicle/person and taxi-order details
      JunctionInspector.tsx          Signal phase copies and interchange movement previews
```

React owns the DOM and forms. Canvas draws roads, physical vehicles/trains, signals, height slices, coordinates and selected paths. Canvas previews use temporary presentation state and restore it; they never replace the live application snapshot. The renderer receives overlay dimensions from its component rather than querying page controls.

Backend observations remain distinct from editable drafts: incoming snapshots do not replace an unsaved form merely because time advanced. Config changes synchronize their corresponding draft group. Numeric view/control fields commit on blur or Enter; grouped forms commit with Apply. Invalid JSON and rejected commands remain visible in status messages.

`api/client.ts` is the only HTTP/SSE transport. Controls run in request order, errors propagate to React, and EventSource closes on unmount. The store rejects older same-run/world snapshots. Reset/map change clears stale selections; a selected metro passenger remains inspectable after boarding and completion.

Every subscription, canvas gesture listener, resize observer and scheduled animation frame has teardown. Map drawing is coalesced to animation frames. React does not create one DOM element for each physical vehicle.

## Build and development

- `npm run frontend:types`: regenerate response interfaces from actual Go JSON structs via `scripts/api-types`.
- `npm run frontend:check`: check frontend TypeScript.
- `npm run frontend:build`: generate types, check and build static assets.
- `npm run frontend:dev`: Vite development server; proxy `/api` to the backend on port 8080.
- Desktop build/pack automatically builds this frontend before compiling Go. `frontend/dist` is generated and ignored by Git.
- On a clean checkout, build the frontend before invoking Go tests/build directly, because `go:embed` requires the compiled assets.

No runtime CDN, browser-side simulation or separate production web server is required. The Windows EXE path is unchanged. macOS builds use the same compiled frontend.

## Regression coverage

The desktop React smoke scenario uses actual DOM events, not store internals. It checks window movement/resize/maximize, configuration submission, map changes, height slices, mixed demand, live fleet controls, invalid JSON, timetable planning, interchange route previews, all-movement phase copies, station queues and passenger route inspection. Helper tests execute the TypeScript geometry sources directly. The default `npm run smoke` executes this React scenario; the obsolete DOM-mutation smoke implementation has been removed.

## Validation record (2026-11-15)

- TypeScript check and Vite production build passed; production Go embeds the generated assets.
- Go tests passed across the backend packages; static endpoint tests now discover and request the hashed JavaScript/CSS from the production HTML. `go vet ./...` passed.
- All 7 desktop tests passed, including gesture pan/zoom/selection/teardown and the TypeScript height-slice and passenger-track helpers.
- Development and packaged Electron React scenarios passed: three presets, editable settings, floating windows, timetable changes, live taxi fleet, mixed demand, interchange and phase previews, station inspector, passenger itinerary and conserved population. The inspected transfer scenario reported three transferring passengers and a four-step itinerary, with no captured page errors.
- The fixed Windows executable was rebuilt in `desktop/dist/win-unpacked/Meridian Traffic.exe`. The production backend hash is checked against `desktop/runtime/win-x64/traffic.exe`.
- Windows runtime was verified. macOS uses the same frontend build; no macOS desktop runtime was available for this validation.

## Radio overlay

Map 1 now loads the former Map 3 geometry. `rendering/radio.ts` draws the JSON-defined 256 m honeycomb; `RadioSettings` and `RadioInspector` consume backend link-budget observations. The frontend performs no radio propagation calculation. Site and terminal display uses the existing physical-height slices.

## Map presentation modes

`MapViewButtons` switches the presentation state among All, Traffic and Communications. `RadioCharts` renders backend observations as received-power/SNR ECDFs, a site-user-count histogram and a bounded simulation-time history. `rendering/radio.ts` draws all serving links and smooth site-load heat, with no hexagonal boundary paths. Visibility does not send backend commands or reset tasks.

Validation: TypeScript/build, server assets and all 10 desktop tests passed. Packaged UI regression and screenshot review confirmed the view buttons, communications charts/heat/links, existing route inspectors and passenger conservation with no captured page errors.
