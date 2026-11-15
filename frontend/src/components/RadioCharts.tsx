import { useAppState, radioHistory } from "../state/store";
type Series = {
  name: string;
  color: string;
  points: { x: number; y: number }[];
};
function Plot({
  title,
  unit,
  series,
  kind = "line",
}: {
  title: string;
  unit: string;
  series: Series[];
  kind?: "line" | "cdf" | "bar";
}) {
  const points = series
    .flatMap((s) => s.points)
    .filter((p) => Number.isFinite(p.x) && Number.isFinite(p.y));
  if (!points.length)
    return (
      <figure className="radio-plot">
        <figcaption>{title}</figcaption>
        <p>No eligible samples</p>
      </figure>
    );
  let x0 = Math.min(...points.map((p) => p.x)),
    x1 = Math.max(...points.map((p) => p.x)),
    y0 = Math.min(0, ...points.map((p) => p.y)),
    y1 = Math.max(...points.map((p) => p.y));
  if (x1 === x0) x1 = x0 + 1;
  if (y1 === y0) y1 = y0 + 1;
  const x = (v: number) => 42 + ((v - x0) / (x1 - x0)) * 260,
    y = (v: number) => 101 - ((v - y0) / (y1 - y0)) * 80;
  return (
    <figure className="radio-plot">
      <figcaption>{title}</figcaption>
      <svg viewBox="0 0 320 127" role="img" aria-label={title}>
        <path d="M42 16V101H302" fill="none" stroke="#666" />
        <text x="38" y="25" textAnchor="end">
          {y1.toFixed(0)}
        </text>
        <text x="38" y="101" textAnchor="end">
          {y0.toFixed(0)}
        </text>
        <text x="42" y="118">
          {x0.toFixed(1)}
        </text>
        <text x="302" y="118" textAnchor="end">
          {x1.toFixed(1)} {unit}
        </text>
        {series.map((s) => (
          <g key={s.name}>
            {kind === "bar" ? (
              s.points.map((p, i) => (
                <rect
                  key={i}
                  x={x(p.x) - 5}
                  y={y(p.y)}
                  width="10"
                  height={101 - y(p.y)}
                  fill={s.color}
                />
              ))
            ) : kind === "cdf" ? (
              <path
                fill="none"
                stroke={s.color}
                strokeWidth="1.6"
                d={
                  "M42,101" +
                  s.points.map((p) => `H${x(p.x)}V${y(p.y)}`).join("")
                }
              />
            ) : (
              <polyline
                fill="none"
                stroke={s.color}
                strokeWidth="1.6"
                points={s.points
                  .filter((p) => Number.isFinite(p.y))
                  .map((p) => `${x(p.x)},${y(p.y)}`)
                  .join(" ")}
              />
            )}
            {s.points.length === 1 && (
              <circle
                cx={x(s.points[0].x)}
                cy={y(s.points[0].y)}
                r="2"
                fill={s.color}
              />
            )}
          </g>
        ))}
      </svg>
      <div className="plot-legend">
        {series.map((s) => (
          <span key={s.name} style={{ color: s.color }}>
            {s.name}
          </span>
        ))}
      </div>
    </figure>
  );
}
const cdf = (values: number[]) =>
  values
    .sort((a, b) => a - b)
    .map((x, i) => ({ x, y: (100 * (i + 1)) / values.length }));
export function RadioCharts() {
  const { scene } = useAppState(),
    v = scene.snapshot?.communications;
  if (!v)
    return (
      <div id="radio-charts">
        <h2>Communications</h2>
        <p>No radio layout in this preset.</p>
      </div>
    );
  const links = v.terminals
      .filter((u) => u.status === "connected")
      .map((u) => u.neighbours.find((l) => l.cell_id === u.serving_cell))
      .filter(Boolean),
    loads = new Map<string, number>();
  v.cells.forEach((c) =>
    loads.set(c.site_id, (loads.get(c.site_id) || 0) + c.associated),
  );
  const peak = Math.max(0, ...loads.values());
  const bins = new Map<number, number>();
  loads.forEach((n) => bins.set(n, (bins.get(n) || 0) + 1));
  return (
    <section id="radio-charts">
      <h2>Communications</h2>
      <p className="radio-note">
        Network totals · all heights · {(v.time_us / 1e6).toFixed(1)} s<br />
        Simplified outdoor model · thermal-noise SNR
      </p>
      <p>
        {v.connected} connected · {v.outage} outage · {v.unsupported} outside
        model
      </p>
      <div className="heat-legend">
        <i />0 — {peak} users / site
      </div>
      <p className="radio-note">
        Heat measures associated-user count, not bandwidth utilization. Lines
        connect each visible admitted user to its serving site.
      </p>
      <Plot
        kind="cdf"
        title="Received power · empirical CDF (%)"
        unit="dBm"
        series={[
          {
            name: `Serving links · n=${links.length}`,
            color: "#82cce5",
            points: cdf(
              links.map((l) => 10 * Math.log10(l.received_power_w) + 30),
            ),
          },
        ]}
      />
      <Plot
        kind="cdf"
        title="SNR · empirical CDF (%)"
        unit="dB"
        series={[
          {
            name: "Serving links · no interference",
            color: "#c8f26b",
            points: cdf(links.map((l) => l.snr_db)),
          },
        ]}
      />
      <Plot
        kind="bar"
        title="Site load distribution (sites)"
        unit="users/site"
        series={[
          {
            name: `All ${loads.size} sites, including idle`,
            color: "#efc85b",
            points: [...bins]
              .sort((a, b) => a[0] - b[0])
              .map(([x, y]) => ({ x, y })),
          },
        ]}
      />
      <Plot
        title="Association history (users)"
        unit="s"
        series={[
          {
            name: "Connected",
            color: "#82cce5",
            points: radioHistory.map((p) => ({ x: p.time, y: p.connected })),
          },
          {
            name: "Outage",
            color: "#efc85b",
            points: radioHistory.map((p) => ({ x: p.time, y: p.outage })),
          },
        ]}
      />
      <p className="radio-note">
        Recent simulation samples; resets and map changes clear history. RSRP,
        SINR, throughput, delay and resource utilization are not produced by the
        current model.
      </p>
    </section>
  );
}
