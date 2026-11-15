function inverse(v: number) {
  let a = 0,
    b = 1;
  for (let i = 0; i < 48; i++) {
    const m = (a + b) / 2;
    if (m * m * (3 - 2 * m) < v) a = m;
    else b = m;
  }
  return (a + b) / 2;
}
export function interval(
  a: number,
  b: number,
  range: [number, number] | null,
): [number, number] | null {
  if (!range) return [0, 1];
  const [lo, hi] = range;
  if (a === b) return a >= lo && a <= hi ? [0, 1] : null;
  const u = (lo - a) / (b - a),
    v = (hi - a) / (b - a),
    start = Math.max(0, Math.min(u, v)),
    end = Math.min(1, Math.max(u, v));
  return start <= end ? [inverse(start), inverse(end)] : null;
}
