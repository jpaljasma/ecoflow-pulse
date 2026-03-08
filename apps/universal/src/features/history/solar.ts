export const SOLAR_HISTORY_START_HOUR = 6;
export const SOLAR_HISTORY_END_HOUR = 20;
export const SOLAR_HISTORY_BUCKET_MINUTES = 10;
export const SOLAR_HISTORY_TICK_HOURS = [6, 9, 12, 15, 18, 20] as const;
export const SOLAR_HISTORY_POINTS =
  ((SOLAR_HISTORY_END_HOUR - SOLAR_HISTORY_START_HOUR) * 60) / SOLAR_HISTORY_BUCKET_MINUTES;
export const SOLAR_HISTORY_CHART_TITLE = '☼ Solar Generated (6am-8pm, 10m buckets)';
const SOLAR_HISTORY_REFRESH_BASE_MS = 60_000;
const SOLAR_HISTORY_REFRESH_JITTER_MS = 7_500;
const NEXT_DAY_REFRESH_BUFFER_MS = 1_000;

export type SolarHistoryBounds = {
  from: Date;
  to: Date;
  compareFrom: Date;
  compareTo: Date;
};

export function buildSolarHistoryBounds(now = new Date()): SolarHistoryBounds {
  const from = new Date(now);
  from.setHours(0, 0, 0, 0);

  const compareTo = new Date(from);
  const compareFrom = new Date(compareTo);
  compareFrom.setDate(compareFrom.getDate() - 1);

  return {
    from,
    to: now,
    compareFrom,
    compareTo
  };
}

export function buildTodayBounds(now = new Date()): { from: Date; to: Date } {
  const { from, to } = buildSolarHistoryBounds(now);
  return { from, to };
}

export function historyRefreshIntervalMs(key: string): number {
  return SOLAR_HISTORY_REFRESH_BASE_MS + stableHash(key) % SOLAR_HISTORY_REFRESH_JITTER_MS;
}

export function msUntilNextLocalDay(now = new Date()): number {
  const next = new Date(now);
  next.setHours(24, 0, 0, 0);
  return Math.max(NEXT_DAY_REFRESH_BUFFER_MS, next.getTime() - now.getTime() + NEXT_DAY_REFRESH_BUFFER_MS);
}

function stableHash(input: string): number {
  let hash = 2166136261;
  for (let index = 0; index < input.length; index += 1) {
    hash ^= input.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return Math.abs(hash >>> 0);
}
