import type { RollupPoint } from '@/features/history/api';

export const SOLAR_HISTORY_POINTS = 72;
const SOLAR_CHART_START_MINUTE = 6 * 60;
const SOLAR_CHART_END_MINUTE = 18 * 60;
const SOLAR_BUCKET_MINUTES = 10;
const SOLAR_HISTORY_REFRESH_BASE_MS = 60_000;
const SOLAR_HISTORY_REFRESH_JITTER_MS = 7_500;

export type SolarHistoryView = {
  todayWh: number;
  seriesWh: number[];
};

export function buildTodayBounds(now = new Date()): { from: Date; to: Date } {
  const from = new Date(now);
  from.setHours(0, 0, 0, 0);
  return { from, to: now };
}

export function buildSolarHistoryView(points: RollupPoint[]): SolarHistoryView {
  const seriesWh = Array.from({ length: SOLAR_HISTORY_POINTS }, () => 0);
  let todayWh = 0;

  for (const point of points) {
    const generatedWh = Math.max(0, point.metrics.solarGeneratedWh ?? 0);
    todayWh += generatedWh;

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

  return {
    todayWh,
    seriesWh
  };
}

export function combineSolarHistoryViews(views: Array<SolarHistoryView | undefined>): SolarHistoryView {
  const seriesWh = Array.from({ length: SOLAR_HISTORY_POINTS }, () => 0);
  let todayWh = 0;

  for (const view of views) {
    if (!view) {
      continue;
    }
    todayWh += view.todayWh;
    for (let index = 0; index < seriesWh.length; index += 1) {
      seriesWh[index] = (seriesWh[index] ?? 0) + (view.seriesWh[index] ?? 0);
    }
  }

  return {
    todayWh,
    seriesWh
  };
}

export function historyRefreshIntervalMs(key: string): number {
  return SOLAR_HISTORY_REFRESH_BASE_MS + stableHash(key) % SOLAR_HISTORY_REFRESH_JITTER_MS;
}

function stableHash(input: string): number {
  let hash = 2166136261;
  for (let index = 0; index < input.length; index += 1) {
    hash ^= input.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return Math.abs(hash >>> 0);
}
