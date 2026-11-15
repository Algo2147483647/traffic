import type { SceneState } from "../state/store";

// Presentation only: all association and power values are supplied by Go.
export function drawRadio(
  ctx: CanvasRenderingContext2D,
  state: SceneState,
  screen: (p: { x: number; y: number }) => { x: number; y: number },
  visible: (z: number) => boolean,
  scale: number,
  exclusions: DOMRect[],
) {
  const layout = state.city?.communications,
    view = state.snapshot?.communications;
  if (state.mapView === "traffic" || !state.showRadio || !layout) return;
  ctx.save();
  for (const r of exclusions) {
    ctx.beginPath();
    ctx.rect(-1000000, -1000000, 2000000, 2000000);
    ctx.rect(r.left, r.top, r.width, r.height);
    ctx.clip("evenodd");
  }
  ctx.font = "12px Times New Roman";
  const counts = new Map<string, number>();
  for (const cell of view?.cells || [])
    counts.set(cell.site_id, (counts.get(cell.site_id) || 0) + cell.associated);
  const peak = Math.max(1, ...counts.values());
  for (const site of layout.sites) {
    const n = counts.get(site.id) || 0;
    if (!n || !visible(site.position_m.z + site.mast_height_m)) continue;
    const p = screen(site.position_m),
      r = Math.max(2, site.coverage_radius_m * scale),
      fraction = n / peak;
    const gradient = ctx.createRadialGradient(p.x, p.y, 0, p.x, p.y, r);
    const hue = 210 - 180 * fraction;
    gradient.addColorStop(0, `hsla(${hue},90%,58%,.5)`);
    gradient.addColorStop(0.5, `hsla(${hue},90%,58%,.22)`);
    gradient.addColorStop(1, `hsla(${hue},90%,58%,0)`);
    ctx.fillStyle = gradient;
    ctx.fillRect(p.x - r, p.y - r, r * 2, r * 2);
  }
  const sites = new Map(layout.sites.map((s) => [s.id, s]));
  for (const u of view?.terminals || []) {
    if (!visible(u.position_m.z)) continue;
    const p = screen(u.position_m);
    const link = u.neighbours.find((l) => l.cell_id === u.serving_cell),
      site = link && sites.get(link.site_id);
    if (
      u.status === "connected" &&
      site &&
      visible(site.position_m.z + site.mast_height_m)
    ) {
      const target = screen(site.position_m);
      ctx.strokeStyle = "rgba(125,204,231,.5)";
      ctx.lineWidth = 0.8;
      ctx.beginPath();
      ctx.moveTo(p.x, p.y);
      ctx.lineTo(target.x, target.y);
      ctx.stroke();
    }
    ctx.fillStyle =
      u.status === "connected"
        ? "#a6dbe8"
        : u.status === "outage"
          ? "#efc85b"
          : "#777";
    ctx.beginPath();
    ctx.arc(p.x, p.y, 1.8, 0, Math.PI * 2);
    ctx.fill();
  }
  for (const site of layout.sites) {
    if (!visible(site.position_m.z + site.mast_height_m)) continue;
    const p = screen(site.position_m),
      selected =
        state.selected?.type === "radio-site" && state.selected.id === site.id;
    ctx.strokeStyle = selected ? "#f0cb60" : "#7bc9e3";
    ctx.fillStyle = selected ? "#f0cb60" : "#7bc9e3";
    ctx.lineWidth = selected ? 2 : 1.25;
    const size = selected ? 8 : Math.max(1.5, Math.min(7, 22 * scale));
    if (size < 4) {
      ctx.beginPath();
      ctx.arc(p.x, p.y, size, 0, Math.PI * 2);
      ctx.fill();
    } else {
      ctx.beginPath();
      ctx.moveTo(p.x, p.y - size);
      ctx.lineTo(p.x - size * 0.7, p.y + size * 0.7);
      ctx.lineTo(p.x + size * 0.7, p.y + size * 0.7);
      ctx.closePath();
      ctx.stroke();
    }
    if (selected || scale > 0.2)
      ctx.fillText(site.id, p.x + size + 3, p.y - size);
  }
  const terminal = view?.terminals.find((u) => u.id === state.radioTerminal);
  if (terminal && visible(terminal.position_m.z)) {
    const p = screen(terminal.position_m);
    for (const link of terminal.neighbours) {
      const site = layout.sites.find((s) => s.id === link.site_id);
      if (!site || !visible(site.position_m.z + site.mast_height_m)) continue;
      const target = screen(site.position_m),
        serving = link.cell_id === terminal.serving_cell;
      ctx.strokeStyle = serving ? "#c8f26b" : "#7097ab";
      ctx.lineWidth = serving ? 2 : 1;
      ctx.setLineDash(serving ? [] : [5, 5]);
      ctx.beginPath();
      ctx.moveTo(p.x, p.y);
      ctx.lineTo(target.x, target.y);
      ctx.stroke();
    }
    ctx.setLineDash([]);
    ctx.strokeStyle = terminal.status === "connected" ? "#c8f26b" : "#efc85b";
    ctx.beginPath();
    ctx.arc(p.x, p.y, 7, 0, Math.PI * 2);
    ctx.stroke();
    ctx.fillStyle = "#d7e9ed";
    ctx.fillText(terminal.id, p.x + 10, p.y + 17);
  }
  ctx.restore();
}
