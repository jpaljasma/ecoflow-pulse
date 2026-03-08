export const SOLAR_HISTORY_POINTS = 72;
const SOLAR_HISTORY_REFRESH_BASE_MS = 60_000;
const SOLAR_HISTORY_REFRESH_JITTER_MS = 7_500;
const NEXT_DAY_REFRESH_BUFFER_MS = 1_000;

export function buildTodayBounds(now = new Date()): { from: Date; to: Date } {
  const from = new Date(now);
  from.setHours(0, 0, 0, 0);
  return { from, to: now };
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
