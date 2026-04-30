export type ChartPoint = {
  x: number;
  y: number;
};

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
