# Active example presets

Map 1 is now the radial grid and central cloverleaf (formerly Map 3). Map 2 is unchanged. Only `example/Map1` and `example/Map2` are discoverable presets; `preset-1` always identifies the new Map 1.

Map 1 retains the 2 km inner ring, 4 km outer eight-lane ring, four axes, 500 m grid, corner entries, road tunnels, four metro lines and six real transfer stations. Its origin is the cloverleaf centre. `generation.json` and `materials/` contain generation inputs; running `scripts/generate_radial_grid_map.py` does not depend on a removed example folder. Historical signal/circuit geometry is a test-only fixture under `internal/testdata/signal-network`, not shipped as a map.

`preset.json` declares road, metro and optional communications materials. `road.json` references the matching files. Map 1 adds `communications.json`: 700 sites and 2,100 directional cells, on a 256 m honeycomb. See [communications implementation](communications-baseline.md).

The desktop packer replaces the unpacked output at the original path and includes only the active example folders. Distances are metres and X/Y share one physical scale.
