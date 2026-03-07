import type { CompareRollupSeries, RollupPoint } from '../grpc/telemetryClient.js';

export const SOLAR_HISTORY_POINTS = 72;
const SOLAR_CHART_START_MS = 6 * 60 * 60 * 1000;
const SOLAR_CHART_END_MS = 18 * 60 * 60 * 1000;
const SOLAR_BUCKET_MS = 10 * 60 * 1000;

export type SolarHistoryView = {
  todayWh: number;
  yesterdayWh: number;
  deltaPct: number | null;
  seriesWh: number[];
};

export function buildCompareSolarHistoryView(series: CompareRollupSeries): SolarHistoryView {
  const todayWh = sumSolarWh(series.current.points);
  const yesterdayWh = sumSolarWh(series.previous.points);
  return {
    todayWh,
    yesterdayWh,
    deltaPct: computeDeltaPct(todayWh, yesterdayWh),
    seriesWh: buildSeriesWh(series.current.points, series.current.fromUnixMs)
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

export function emptySolarHistoryView(): SolarHistoryView {
  return {
    todayWh: 0,
    yesterdayWh: 0,
    deltaPct: null,
    seriesWh: Array.from({ length: SOLAR_HISTORY_POINTS }, () => 0)
  };
}

function sumSolarWh(points: RollupPoint[]): number {
  let totalWh = 0;
  for (const point of points) {
    totalWh += solarWhForPoint(point);
  }
  return totalWh;
}

function buildSeriesWh(points: RollupPoint[], fromUnixMs: string): number[] {
  const seriesWh = Array.from({ length: SOLAR_HISTORY_POINTS }, () => 0);
  const fromMs = Number(fromUnixMs);
  if (!Number.isFinite(fromMs)) {
    return seriesWh;
  }

  const chartStartMs = fromMs + SOLAR_CHART_START_MS;
  const chartEndMs = fromMs + SOLAR_CHART_END_MS;

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

function computeDeltaPct(todayWh: number, yesterdayWh: number): number | null {
  if (!(yesterdayWh > 0)) {
    return null;
  }
  return ((todayWh - yesterdayWh) / yesterdayWh) * 100;
}

function solarWhForPoint(point: RollupPoint): number {
  return Math.max(0, point.metrics.solarGeneratedWh ?? 0);
}
