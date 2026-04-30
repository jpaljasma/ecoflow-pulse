export type ChartPoint = {
  x: number;
  y: number;
};

const EPSILON = 1e-6;

function looksCumulative(values: number[]): boolean {
  if (values.length < 8) return false;
  let nonDecreasing = 0;
  for (let i = 1; i < values.length; i += 1) {
    if ((values[i] ?? 0) + EPSILON >= (values[i - 1] ?? 0)) nonDecreasing += 1;
  }
  const monotonicRatio = nonDecreasing / Math.max(1, values.length - 1);
  const first = values[0] ?? 0;
  const last = values[values.length - 1] ?? 0;
  return monotonicRatio >= 0.92 && last > first + 1;
}

function toBucketSeries(values: number[]): number[] {
  if (!looksCumulative(values)) return values;
  const buckets: number[] = [];
  let prev = Math.max(0, values[0] ?? 0);
  for (let i = 0; i < values.length; i += 1) {
    const curr = Math.max(0, values[i] ?? 0);
    const delta = i === 0 ? curr : Math.max(0, curr - prev);
    buckets.push(delta);
    prev = curr;
  }
  return buckets;
}

export function normalizeSolarBucketSeries(values: number[] | undefined, points: number): number[] {
  const safePoints = Math.max(1, points);
  const buckets = toBucketSeries(
    (values ?? []).map((value) => (Number.isFinite(value) ? Math.max(0, value) : 0))
  );
  if (buckets.length >= safePoints) {
    return buckets.slice(0, safePoints);
  }
  return [...buckets, ...Array.from({ length: safePoints - buckets.length }, () => 0)];
}

export function buildStepPolylinePoints(points: ChartPoint[]): ChartPoint[] {
  if (points.length < 2) {
    return points;
  }

  const stepped: ChartPoint[] = [points[0]!];
  for (let index = 1; index < points.length; index += 1) {
    const previous = points[index - 1]!;
    const next = points[index]!;
    stepped.push({ x: next.x, y: previous.y }, next);
  }
  return stepped;
}

export function buildSvgStepPath(points: ChartPoint[]): string {
  if (points.length < 2) {
    return '';
  }

  const first = points[0]!;
  let d = `M ${first.x.toFixed(2)} ${first.y.toFixed(2)}`;
  for (let index = 1; index < points.length; index += 1) {
    const next = points[index]!;
    d += ` H ${next.x.toFixed(2)} V ${next.y.toFixed(2)}`;
  }
  return d;
}
