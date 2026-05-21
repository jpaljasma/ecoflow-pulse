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
const TREND_METRIC_KEYS: TrendMetricKey[] = ['solar', 'ac', 'dc', 'load'];

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
  return repairPowerTrendDropouts({
    solar: buildTrendValues(series, 'solar', now),
    ac: buildTrendValues(series, 'ac', now),
    dc: buildTrendValues(series, 'dc', now),
    load: buildTrendValues(series, 'load', now)
  });
}

export function sumPowerTrendViews(views: PowerTrendView[]): PowerTrendView {
  if (views.length === 0) {
    return emptyPowerTrendView();
  }
  const out = emptyPowerTrendView();
  for (const view of views) {
    for (const key of TREND_METRIC_KEYS) {
      for (let i = 0; i < POWER_TREND_POINTS; i += 1) {
        out[key][i] = (out[key][i] ?? 0) + (view[key][i] ?? 0);
      }
    }
  }
  return repairPowerTrendDropouts(out);
}

export function repairPowerTrendDropouts(
  view: PowerTrendView,
  maxRunLength = 2
): PowerTrendView {
  const out: PowerTrendView = {
    solar: [...view.solar],
    ac: [...view.ac],
    dc: [...view.dc],
    load: [...view.load]
  };
  const length = Math.min(out.solar.length, out.ac.length, out.dc.length, out.load.length);

  let idx = 1;
  while (idx < length - 1) {
    if (!isZeroPowerSlot(out, idx)) {
      idx += 1;
      continue;
    }

    const start = idx;
    while (idx < length - 1 && isZeroPowerSlot(out, idx)) {
      idx += 1;
    }
    const end = idx - 1;
    const runLength = end - start + 1;
    const before = start - 1;
    const after = idx;
    if (
      runLength <= maxRunLength &&
      hasPowerSignal(out, before) &&
      hasPowerSignal(out, after)
    ) {
      fillDropoutRun(out, start, end, before, after);
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

export function mergePowerTrendPrefill(
  prefill: PowerTrendView,
  live: PowerTrendView,
  liveCoveragePoints: number
): PowerTrendView {
  return repairPowerTrendDropouts({
    solar: mergeTrendPrefill(prefill.solar, live.solar, liveCoveragePoints),
    ac: mergeTrendPrefill(prefill.ac, live.ac, liveCoveragePoints),
    dc: mergeTrendPrefill(prefill.dc, live.dc, liveCoveragePoints),
    load: mergeTrendPrefill(prefill.load, live.load, liveCoveragePoints)
  });
}

export function mergeTrendPrefillWithLivePoints(
  prefill: number[],
  livePoints: TimePoint[] | undefined,
  now = new Date()
): number[] {
  const normalizedPrefill = normalizeSeries(prefill);
  if (!livePoints?.length) {
    return normalizedPrefill;
  }

  const bounds = buildPowerTrendBounds(now);
  const relevantPoints = livePoints
    .filter((point) => point.ts >= bounds.slotFromMs && point.ts <= bounds.slotToMs)
    .sort((left, right) => left.ts - right.ts);
  if (!relevantPoints.length) {
    return normalizedPrefill;
  }

  const merged = [...normalizedPrefill];
  let pointIdx = 0;
  let lastLiveValue: number | null = null;
  let firstLiveSlot = POWER_TREND_POINTS;

  for (let i = 0; i < POWER_TREND_POINTS; i += 1) {
    const slotStartMs = bounds.slotFromMs + i * POWER_TREND_BUCKET_MS;
    const slotEndMs = slotStartMs + POWER_TREND_BUCKET_MS;

    while (pointIdx < relevantPoints.length && relevantPoints[pointIdx]!.ts < slotEndMs) {
      lastLiveValue = relevantPoints[pointIdx]!.value;
      firstLiveSlot = Math.min(firstLiveSlot, i);
      pointIdx += 1;
    }

    if (i >= firstLiveSlot && lastLiveValue !== null) {
      merged[i] = lastLiveValue;
    }
  }

  return merged;
}

export function mergePowerTrendPrefillWithLivePoints(
  prefill: PowerTrendView,
  livePoints: {
    solar?: TimePoint[];
    ac?: TimePoint[];
    dc?: TimePoint[];
    load?: TimePoint[];
  },
  now = new Date()
): PowerTrendView {
  return repairPowerTrendDropouts({
    solar: mergeTrendPrefillWithLivePoints(prefill.solar, livePoints.solar, now),
    ac: mergeTrendPrefillWithLivePoints(prefill.ac, livePoints.ac, now),
    dc: mergeTrendPrefillWithLivePoints(prefill.dc, livePoints.dc, now),
    load: mergeTrendPrefillWithLivePoints(prefill.load, livePoints.load, now)
  });
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

function fillDropoutRun(
  view: PowerTrendView,
  start: number,
  end: number,
  before: number,
  after: number
) {
  const runLength = end - start + 1;
  TREND_METRIC_KEYS.forEach((key) => {
    const from = view[key][before] ?? 0;
    const to = view[key][after] ?? from;
    for (let idx = start; idx <= end; idx += 1) {
      const step = (idx - start + 1) / (runLength + 1);
      view[key][idx] = from + (to - from) * step;
    }
  });
}

function hasPowerSignal(view: PowerTrendView, idx: number): boolean {
  return TREND_METRIC_KEYS.some((key) => positiveFinite(view[key][idx]));
}

function isZeroPowerSlot(view: PowerTrendView, idx: number): boolean {
  return TREND_METRIC_KEYS.every((key) => !positiveFinite(view[key][idx]));
}

function positiveFinite(value: number | undefined): boolean {
  return typeof value === 'number' && Number.isFinite(value) && value > 0;
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
