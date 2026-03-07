export const SOLAR_HISTORY_POINTS = 72;
const SOLAR_HISTORY_REFRESH_BASE_MS = 60_000;
const SOLAR_HISTORY_REFRESH_JITTER_MS = 7_500;

export function buildTodayBounds(now = new Date()): { from: Date; to: Date } {
  const from = new Date(now);
  from.setHours(0, 0, 0, 0);
  return { from, to: now };
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
