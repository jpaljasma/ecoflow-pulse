import type { CompareRollupSeries, RollupPoint } from '@/features/history/api';

export const SOLAR_HISTORY_POINTS = 72;
const SOLAR_CHART_START_MINUTE = 6 * 60;
const SOLAR_CHART_END_MINUTE = 18 * 60;
const SOLAR_BUCKET_MINUTES = 10;
const SOLAR_HISTORY_REFRESH_BASE_MS = 60_000;
const SOLAR_HISTORY_REFRESH_JITTER_MS = 7_500;

export type SolarHistoryView = {
  todayWh: number;
  yesterdayWh: number;
  deltaPct: number | null;
  seriesWh: number[];
};

export type SolarHistoryOptions = {
  maxSolarWatts?: number;
};

export function buildTodayBounds(now = new Date()): { from: Date; to: Date } {
  const from = new Date(now);
  from.setHours(0, 0, 0, 0);
  return { from, to: now };
}

export function buildSolarHistoryView(
  points: RollupPoint[],
  options: SolarHistoryOptions = {}
): SolarHistoryView {
  return {
    todayWh: sumSolarWh(points, options),
    yesterdayWh: 0,
    deltaPct: null,
    seriesWh: buildSeriesWh(points, options)
  };
}

export function buildCompareSolarHistoryView(
  series: CompareRollupSeries,
  options: SolarHistoryOptions = {}
): SolarHistoryView {
  const todayWh = sumSolarWh(series.current.points, options);
  const yesterdayWh = sumSolarWh(series.previous.points, options);
  return {
    todayWh,
    yesterdayWh,
    deltaPct: computeDeltaPct(todayWh, yesterdayWh),
    seriesWh: buildSeriesWh(series.current.points, options)
  };
}

export function combineSolarHistoryViews(views: Array<SolarHistoryView | undefined>): SolarHistoryView {
  const seriesWh = Array.from({ length: SOLAR_HISTORY_POINTS }, () => 0);
  let todayWh = 0;
  let yesterdayWh = 0;

  for (const view of views) {
    if (!view) {
      continue;
    }
    todayWh += view.todayWh;
    yesterdayWh += view.yesterdayWh;
    for (let index = 0; index < seriesWh.length; index += 1) {
      seriesWh[index] = (seriesWh[index] ?? 0) + (view.seriesWh[index] ?? 0);
    }
  }

  return {
    todayWh,
    yesterdayWh,
    deltaPct: computeDeltaPct(todayWh, yesterdayWh),
    seriesWh
  };
}

export function historyRefreshIntervalMs(key: string): number {
  return SOLAR_HISTORY_REFRESH_BASE_MS + stableHash(key) % SOLAR_HISTORY_REFRESH_JITTER_MS;
}

function sumSolarWh(points: RollupPoint[], options: SolarHistoryOptions): number {
  let totalWh = 0;
  for (const point of points) {
    totalWh += solarWhForPoint(point, options);
  }
  return totalWh;
}

function buildSeriesWh(points: RollupPoint[], options: SolarHistoryOptions): number[] {
  const seriesWh = Array.from({ length: SOLAR_HISTORY_POINTS }, () => 0);

  for (const point of points) {
    const generatedWh = solarWhForPoint(point, options);
    const bucketMs = Number(point.bucketStartUnixMs);
    if (!Number.isFinite(bucketMs)) {
      continue;
    }

    const bucketDate = new Date(bucketMs);
    const minuteOfDay = bucketDate.getHours() * 60 + bucketDate.getMinutes();
    if (minuteOfDay < SOLAR_CHART_START_MINUTE || minuteOfDay >= SOLAR_CHART_END_MINUTE) {
      continue;
    }

    const bucketIndex = Math.floor((minuteOfDay - SOLAR_CHART_START_MINUTE) / SOLAR_BUCKET_MINUTES);
    if (bucketIndex >= 0 && bucketIndex < seriesWh.length) {
      seriesWh[bucketIndex] = (seriesWh[bucketIndex] ?? 0) + generatedWh;
    }
  }

  return seriesWh;
}

function computeDeltaPct(todayWh: number, yesterdayWh: number): number | null {
  if (!(yesterdayWh > 0)) {
    return null;
  }
  return ((todayWh - yesterdayWh) / yesterdayWh) * 100;
}

function solarWhForPoint(point: RollupPoint, options: SolarHistoryOptions): number {
  const explicitWh = Math.max(0, point.metrics.solarGeneratedWh ?? 0);
  if (explicitWh > 0) {
    return explicitWh;
  }

  const pvAvgW = clampSolarWatts(Math.max(0, point.metrics.pvAvgW ?? 0), options.maxSolarWatts);
  if (!(pvAvgW > 0)) {
    return 0;
  }

  const bucketStartMs = Number(point.bucketStartUnixMs);
  const bucketEndMs = Number(point.bucketEndUnixMs);
  if (!Number.isFinite(bucketStartMs) || !Number.isFinite(bucketEndMs) || bucketEndMs <= bucketStartMs) {
    return 0;
  }

  const durationHours = (bucketEndMs - bucketStartMs) / (60 * 60 * 1000);
  return pvAvgW * durationHours;
}

function clampSolarWatts(watts: number, maxSolarWatts?: number): number {
  if (!(watts > 0)) {
    return 0;
  }
  if (!(maxSolarWatts && maxSolarWatts > 0)) {
    return watts;
  }
  return Math.min(watts, maxSolarWatts);
}

function stableHash(input: string): number {
  let hash = 2166136261;
  for (let index = 0; index < input.length; index += 1) {
    hash ^= input.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return Math.abs(hash >>> 0);
}
