import type { RollupPoint, RollupSeries } from '@/features/history/api';
import type { TimePoint } from '@/features/telemetry/engine/ringBuffer';

export const POWER_TREND_POINTS = 60;
export const POWER_TREND_BUCKET_SECONDS = 5;

const POWER_TREND_BUCKET_MS = POWER_TREND_BUCKET_SECONDS * 1000;

export type PowerTrendView = {
  solar: number[];
  ac: number[];
  dc: number[];
  load: number[];
};

type TrendMetricKey = keyof PowerTrendView;

const METRIC_KEY_BY_SERIES: Record<TrendMetricKey, keyof RollupPoint['metrics']> = {
  solar: 'pvAvgW',
  ac: 'acInAvgW',
  dc: 'dcAvgW',
  load: 'loadAvgW'
};

export function emptyPowerTrendView(): PowerTrendView {
  return {
    solar: Array.from({ length: POWER_TREND_POINTS }, () => 0),
    ac: Array.from({ length: POWER_TREND_POINTS }, () => 0),
    dc: Array.from({ length: POWER_TREND_POINTS }, () => 0),
    load: Array.from({ length: POWER_TREND_POINTS }, () => 0)
  };
}

export function buildPowerTrendBounds(now = new Date()) {
  const toMs = Math.ceil(now.getTime() / POWER_TREND_BUCKET_MS) * POWER_TREND_BUCKET_MS;
  const fromMs = toMs - (POWER_TREND_POINTS - 1) * POWER_TREND_BUCKET_MS;
  const queryFromMs = Math.floor(fromMs / 60_000) * 60_000;
  const queryToMs = Math.ceil((toMs + 1) / 60_000) * 60_000;

  return {
    queryFrom: new Date(queryFromMs),
    queryTo: new Date(queryToMs),
    slotFromMs: fromMs,
    slotToMs: toMs
  };
}

export function buildPowerTrendView(series: RollupSeries, now = new Date()): PowerTrendView {
  return {
    solar: buildTrendValues(series, 'solar', now),
    ac: buildTrendValues(series, 'ac', now),
    dc: buildTrendValues(series, 'dc', now),
    load: buildTrendValues(series, 'load', now)
  };
}

export function sumPowerTrendViews(views: PowerTrendView[]): PowerTrendView {
  if (views.length === 0) {
    return emptyPowerTrendView();
  }
  const out = emptyPowerTrendView();
  for (const view of views) {
    for (let i = 0; i < POWER_TREND_POINTS; i += 1) {
      out.solar[i] = (out.solar[i] ?? 0) + (view.solar[i] ?? 0);
      out.ac[i] = (out.ac[i] ?? 0) + (view.ac[i] ?? 0);
      out.dc[i] = (out.dc[i] ?? 0) + (view.dc[i] ?? 0);
      out.load[i] = (out.load[i] ?? 0) + (view.load[i] ?? 0);
    }
  }
  return out;
}

export function mergeTrendPrefill(prefill: number[], live: number[], liveCoveragePoints: number): number[] {
  const normalizedPrefill = normalizeSeries(prefill);
  const clampedCoverage = clampInt(liveCoveragePoints, 0, POWER_TREND_POINTS);
  if (clampedCoverage === 0) {
    return normalizedPrefill;
  }
  const liveTail = live.slice(-clampedCoverage);
  if (liveTail.length === 0) {
    return normalizedPrefill;
  }
  return [
    ...normalizedPrefill.slice(0, POWER_TREND_POINTS - liveTail.length),
    ...liveTail
  ];
}

export function sparklineCoveragePoints(points: TimePoint[] | undefined): number {
  if (!points?.length) {
    return 0;
  }
  const first = points[0]?.ts ?? 0;
  const last = points[points.length - 1]?.ts ?? first;
  if (last <= first) {
    return 1;
  }
  const covered = Math.ceil((last - first) / POWER_TREND_BUCKET_MS) + 1;
  return clampInt(covered, 1, POWER_TREND_POINTS);
}

function buildTrendValues(series: RollupSeries, trendKey: TrendMetricKey, now: Date): number[] {
  const bounds = buildPowerTrendBounds(now);
  const metricKey = METRIC_KEY_BY_SERIES[trendKey];
  const points = [...series.points].sort(
    (left, right) => Number(left.bucketStartUnixMs) - Number(right.bucketStartUnixMs)
  );

  const values: number[] = [];
  let pointIdx = 0;
  for (let i = 0; i < POWER_TREND_POINTS; i += 1) {
    const slotMidMs = bounds.slotFromMs + i * POWER_TREND_BUCKET_MS + POWER_TREND_BUCKET_MS / 2;

    while (
      pointIdx < points.length &&
      pointBucketEndMs(points[pointIdx] as RollupPoint) <= slotMidMs
    ) {
      pointIdx += 1;
    }

    const point = points[pointIdx];
    if (!point) {
      values.push(0);
      continue;
    }
    const bucketStartMs = Number(point.bucketStartUnixMs);
    const bucketEndMs = pointBucketEndMs(point);
    if (slotMidMs >= bucketStartMs && slotMidMs < bucketEndMs) {
      values.push(point.metrics[metricKey] ?? 0);
    } else {
      values.push(0);
    }
  }

  return normalizeSeries(values);
}

function pointBucketEndMs(point: RollupPoint): number {
  const explicit = Number(point.bucketEndUnixMs);
  if (Number.isFinite(explicit) && explicit > 0) {
    return explicit;
  }
  return Number(point.bucketStartUnixMs) + 60_000;
}

function normalizeSeries(values: number[]): number[] {
  if (values.length === POWER_TREND_POINTS) {
    return values;
  }
  if (values.length > POWER_TREND_POINTS) {
    return values.slice(-POWER_TREND_POINTS);
  }
  return [
    ...Array.from({ length: POWER_TREND_POINTS - values.length }, () => 0),
    ...values
  ];
}

function clampInt(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, Math.trunc(value)));
}
