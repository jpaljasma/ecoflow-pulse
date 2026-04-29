import type { CompareRollupSeries, RollupPoint, RollupSeries } from '../grpc/telemetryClient.js';

const SOLAR_CHART_START_MS = 6 * 60 * 60 * 1000;
const SOLAR_CHART_END_MS = 20 * 60 * 60 * 1000;
const SOLAR_BUCKET_MS = 10 * 60 * 1000;
export const SOLAR_HISTORY_POINTS = (SOLAR_CHART_END_MS - SOLAR_CHART_START_MS) / SOLAR_BUCKET_MS;

type SolarHistoryWindowOptions = {
  windowStartMinutes?: number;
  windowEndMinutes?: number;
};

type SolarHistoryWindow = {
  chartStartMs: number;
  chartEndMs: number;
  points: number;
};

export type SolarHistoryView = {
  todayWh: number;
  yesterdayWh: number;
  yesterdayRunningWh: number;
  deltaPct: number | null;
  seriesWh: number[];
  yesterdaySeriesWh: number[];
};

export function buildCompareSolarHistoryView(
  series: CompareRollupSeries,
  options: SolarHistoryWindowOptions = {}
): SolarHistoryView {
  const window = resolveSolarHistoryWindow(options);
  const todayWh = sumSolarWh(series.current.points);
  const yesterdayWh = sumSolarWh(series.previous.points);
  const yesterdayRunningWh = sumSolarWhUntil(
    series.previous.points,
    runningCompareCutoffMs(series.current, series.previous)
  );
  return {
    todayWh,
    yesterdayWh,
    yesterdayRunningWh,
    deltaPct: computeDeltaPct(todayWh, yesterdayRunningWh),
    seriesWh: buildSeriesWh(series.current.points, series.current.fromUnixMs, window),
    yesterdaySeriesWh: buildSeriesWh(series.previous.points, series.previous.fromUnixMs, window)
  };
}

export function combineSolarHistoryViews(views: Array<SolarHistoryView | undefined>): SolarHistoryView {
  const points = views.find((view) => view?.seriesWh.length)?.seriesWh.length ?? SOLAR_HISTORY_POINTS;
  const seriesWh = Array.from({ length: points }, () => 0);
  const yesterdaySeriesWh = Array.from({ length: points }, () => 0);
  let todayWh = 0;
  let yesterdayWh = 0;
  let yesterdayRunningWh = 0;

  for (const view of views) {
    if (!view) {
      continue;
    }
    todayWh += view.todayWh;
    yesterdayWh += view.yesterdayWh;
    yesterdayRunningWh += view.yesterdayRunningWh;
    for (let index = 0; index < seriesWh.length; index += 1) {
      seriesWh[index] = (seriesWh[index] ?? 0) + (view.seriesWh[index] ?? 0);
      yesterdaySeriesWh[index] =
        (yesterdaySeriesWh[index] ?? 0) + (view.yesterdaySeriesWh[index] ?? 0);
    }
  }

  return {
    todayWh,
    yesterdayWh,
    yesterdayRunningWh,
    deltaPct: computeDeltaPct(todayWh, yesterdayRunningWh),
    seriesWh,
    yesterdaySeriesWh
  };
}

export function emptySolarHistoryView(): SolarHistoryView {
  return {
    todayWh: 0,
    yesterdayWh: 0,
    yesterdayRunningWh: 0,
    deltaPct: null,
    seriesWh: Array.from({ length: SOLAR_HISTORY_POINTS }, () => 0),
    yesterdaySeriesWh: Array.from({ length: SOLAR_HISTORY_POINTS }, () => 0)
  };
}

function sumSolarWh(points: RollupPoint[]): number {
  let totalWh = 0;
  for (const point of points) {
    totalWh += solarWhForPoint(point);
  }
  return totalWh;
}

function sumSolarWhUntil(points: RollupPoint[], cutoffUnixMs: number | undefined): number {
  if (cutoffUnixMs === undefined || !Number.isFinite(cutoffUnixMs)) {
    return sumSolarWh(points);
  }

  let totalWh = 0;
  for (const point of points) {
    const bucketStartMs = Number(point.bucketStartUnixMs);
    if (!Number.isFinite(bucketStartMs) || bucketStartMs >= cutoffUnixMs) {
      continue;
    }
    totalWh += solarWhForPoint(point);
  }
  return totalWh;
}

function buildSeriesWh(points: RollupPoint[], fromUnixMs: string, window: SolarHistoryWindow): number[] {
  const seriesWh = Array.from({ length: window.points }, () => 0);
  const fromMs = Number(fromUnixMs);
  if (!Number.isFinite(fromMs)) {
    return seriesWh;
  }

  const chartStartMs = fromMs + window.chartStartMs;
  const chartEndMs = fromMs + window.chartEndMs;

  for (const point of points) {
    const generatedWh = solarWhForPoint(point);
    const bucketStartMs = Number(point.bucketStartUnixMs);
    if (!Number.isFinite(bucketStartMs) || bucketStartMs < chartStartMs || bucketStartMs >= chartEndMs) {
      continue;
    }

    const bucketIndex = Math.floor((bucketStartMs - chartStartMs) / SOLAR_BUCKET_MS);
    if (bucketIndex >= 0 && bucketIndex < seriesWh.length) {
      seriesWh[bucketIndex] = (seriesWh[bucketIndex] ?? 0) + generatedWh;
    }
  }

  return seriesWh;
}

function resolveSolarHistoryWindow(options: SolarHistoryWindowOptions): SolarHistoryWindow {
  const chartStartMs = Number.isFinite(options.windowStartMinutes)
    ? Number(options.windowStartMinutes) * 60 * 1000
    : SOLAR_CHART_START_MS;
  const chartEndMs = Number.isFinite(options.windowEndMinutes)
    ? Number(options.windowEndMinutes) * 60 * 1000
    : SOLAR_CHART_END_MS;
  const spanMs = Math.max(SOLAR_BUCKET_MS, chartEndMs - chartStartMs);
  return {
    chartStartMs,
    chartEndMs: chartStartMs + Math.ceil(spanMs / SOLAR_BUCKET_MS) * SOLAR_BUCKET_MS,
    points: Math.max(1, Math.ceil(spanMs / SOLAR_BUCKET_MS))
  };
}

function runningCompareCutoffMs(current: RollupSeries, previous: RollupSeries): number | undefined {
  const currentFromMs = Number(current.fromUnixMs);
  const currentToMs = Number(current.toUnixMs);
  const previousFromMs = Number(previous.fromUnixMs);
  const previousToMs = Number(previous.toUnixMs);
  if (
    !Number.isFinite(currentFromMs) ||
    !Number.isFinite(currentToMs) ||
    !Number.isFinite(previousFromMs)
  ) {
    return undefined;
  }
  const cutoffMs = previousFromMs + Math.max(0, currentToMs - currentFromMs);
  return Number.isFinite(previousToMs) ? Math.min(cutoffMs, previousToMs) : cutoffMs;
}

function computeDeltaPct(todayWh: number, yesterdayWh: number): number | null {
  if (!(yesterdayWh > 0)) {
    return null;
  }
  return ((todayWh - yesterdayWh) / yesterdayWh) * 100;
}

function solarWhForPoint(point: RollupPoint): number {
  const explicitWh = Math.max(0, point.metrics.solarGeneratedWh ?? 0);
  const derivedWh = deriveBucketSolarWh(point);
  return Math.max(explicitWh, derivedWh);
}

function deriveBucketSolarWh(point: RollupPoint): number {
  const startMs = Number(point.bucketStartUnixMs);
  const endMs = Number(point.bucketEndUnixMs);
  if (!Number.isFinite(startMs) || !Number.isFinite(endMs) || endMs <= startMs) {
    return 0;
  }
  const durationHours = (endMs - startMs) / (60 * 60 * 1000);
  if (!(durationHours > 0) || !(point.metrics.pvAvgW > 0)) {
    return 0;
  }
  return point.metrics.pvAvgW * durationHours;
}
