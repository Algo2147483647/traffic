# Experimental dispatch extension

This directory is loaded at backend startup. In the unpacked desktop it lives at
`resources/experiments/dispatch` beside `resources/configs`. Edit a local manifest
and restart the application to discover a new algorithm in Settings > Taxi.

`wait-balanced.json` runs `wait_balanced.py` with the Python interpreter on PATH.
Use an absolute interpreter path in the command array for a dedicated environment.
No Python dependency is needed for the six native policies or Traffic mode.
Worker programs execute with your local user permissions, not in a security sandbox.

Protocol v1 is deliberately one-shot: read one JSON line from stdin, write exactly
one proposal JSON object to stdout and exit. Use stderr for diagnostics. Each
decision runs in a fresh process. The example uses only the Python standard library.
There is no automatic installation of experimental dependencies.

Input contains contract_version, epoch, request_id, sim_time_s, seed, parameters,
vehicles, orders and candidates. Candidate ETA is a free-flow prediction in seconds;
actual road movement includes signals, queues and lane restrictions. Candidate
pickup_distance_m is meters. Only reachable pairs are included. The oldest 64
waiting orders are considered per decision, against all eligible idle taxis.

Reply with matching contract_version/epoch/request_id and assignments containing
vehicle_id/order_id pairs. Each vehicle and order can occur once; absent orders
remain waiting. Unknown IDs, forbidden pairs, duplicate matches, extra output or
wrong identities reject the entire proposal. The host owns routes and order state.
An empty assignment list is valid. Diagnostics must not be written to stdout.

A decision freezes simulation time while UI commands stay responsive. Response
limits are 1 MiB stdout, 16 KiB stderr and the manifest wall timeout. Three failures
pause the scenario; no baseline silently replaces a failed experiment. Reset or
mode/map changes cancel pending decisions. Descendant cleanup uses taskkill /T on
Windows and a process group on macOS/Linux; this is not containment of a deliberately
escaping process. Workers must not detach background processes.

Stateful experimental algorithms should encode their state outside this v1 worker
protocol only when intentionally testing non-reproducible external state. For clean
replay use a pure decision function seeded from (seed, request_id). Persistent-worker
handshakes and checkpoint restore are future extensions, not current capabilities.

For a compiled Go policy implement dispatch.Policy and register a factory in init:

```go
func init() {
    err := dispatch.Register(dispatch.Metadata{ID: "research.my_policy", Name: "My policy", Kind: "native"},
        func() dispatch.Policy { return &MyPolicy{} })
    if err != nil { panic(err) }
}
```

Import that package in the backend and taxi-lab bootstrap. Decide accepts detached
values and returns dispatch.Proposal. Use dispatch.Empty(input) for correct identity
fields. Native code is trusted/cooperative; use a process worker for hard deadlines.

Reproducible paired runs from the project root:

```powershell
go run ./cmd/taxi-lab -algorithms nearest,hungarian,experimental.wait_balanced -seeds 71429,71430,71431 -duration 600 -output desktop/runtime/taxi-comparison
```

This launches a separate experiment program and leaves the interactive scenario
alone. JSON contains map/engine hashes, configuration, order timestamps, fleet
statistics and failure status; CSV includes open orders at the horizon. No warm-up
is excluded. Mean wait covers picked-up orders and must be read together with
cancelled/open counts. These are simulator results, not calibrated service forecasts.
