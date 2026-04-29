export const SOLAR_HISTORY_START_HOUR = 6;
export const SOLAR_HISTORY_END_HOUR = 20;
export const SOLAR_HISTORY_BUCKET_MINUTES = 10;
const SOLAR_HISTORY_WINDOW_ROUND_MINUTES = 30;
export const SOLAR_HISTORY_TICK_HOURS = [6, 9, 12, 15, 18, 20] as const;
export const SOLAR_HISTORY_POINTS =
  ((SOLAR_HISTORY_END_HOUR - SOLAR_HISTORY_START_HOUR) * 60) / SOLAR_HISTORY_BUCKET_MINUTES;
export const SOLAR_HISTORY_CHART_TITLE = 'Solar Generated (6am-8pm, 10m buckets)';
const SOLAR_HISTORY_REFRESH_BASE_MS = 60_000;
const SOLAR_HISTORY_REFRESH_JITTER_MS = 7_500;
const NEXT_DAY_REFRESH_BUFFER_MS = 1_000;

export type SolarHistoryBounds = {
  from: Date;
  to: Date;
  compareFrom: Date;
  compareTo: Date;
};

export type SolarHistoryWindow = {
  startMinutes: number;
  endMinutes: number;
  points: number;
  title: string;
};

export type SolarHistoryForecastLike = {
  timezone?: string | null;
  daily?: Array<{
    dateIso: string;
    sunriseIso?: string | null;
    sunsetIso?: string | null;
  }>;
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

export function defaultSolarHistoryWindow(): SolarHistoryWindow {
  return buildSolarHistoryWindow(SOLAR_HISTORY_START_HOUR * 60, SOLAR_HISTORY_END_HOUR * 60);
}

export function resolveSolarHistoryWindow(
  forecast: SolarHistoryForecastLike | undefined,
  now = new Date()
): SolarHistoryWindow {
  const timezone = forecast?.timezone?.trim();
  if (!timezone) {
    return defaultSolarHistoryWindow();
  }
  const todayIso = getDateIsoInTimezone(now, timezone);
  const today = forecast?.daily?.find((day) => day.dateIso === todayIso);
  const sunriseMinutes = getMinutesInTimezone(today?.sunriseIso, timezone);
  const sunsetMinutes = getMinutesInTimezone(today?.sunsetIso, timezone);
  if (sunriseMinutes === undefined || sunsetMinutes === undefined) {
    return defaultSolarHistoryWindow();
  }

  const startMinutes = floorToWindow(sunriseMinutes, SOLAR_HISTORY_WINDOW_ROUND_MINUTES);
  const endMinutes = ceilToWindow(sunsetMinutes, SOLAR_HISTORY_WINDOW_ROUND_MINUTES);
  if (endMinutes <= startMinutes) {
    return defaultSolarHistoryWindow();
  }
  return buildSolarHistoryWindow(startMinutes, endMinutes);
}

function stableHash(input: string): number {
  let hash = 2166136261;
  for (let index = 0; index < input.length; index += 1) {
    hash ^= input.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return Math.abs(hash >>> 0);
}

function buildSolarHistoryWindow(startMinutes: number, endMinutes: number): SolarHistoryWindow {
  const clampedStart = clampMinutes(startMinutes);
  const clampedEnd = clampMinutes(endMinutes);
  const safeEnd = Math.max(clampedStart + SOLAR_HISTORY_BUCKET_MINUTES, clampedEnd);
  const points = Math.max(1, (safeEnd - clampedStart) / SOLAR_HISTORY_BUCKET_MINUTES);
  return {
    startMinutes: clampedStart,
    endMinutes: safeEnd,
    points,
    title: `Solar Generated (${formatTitleTime(clampedStart)}-${formatTitleTime(safeEnd)}, 10m buckets)`
  };
}

function floorToWindow(value: number, windowMinutes: number): number {
  return clampMinutes(Math.floor(value / windowMinutes) * windowMinutes);
}

function ceilToWindow(value: number, windowMinutes: number): number {
  return clampMinutes(Math.ceil(value / windowMinutes) * windowMinutes);
}

function clampMinutes(value: number): number {
  return Math.max(0, Math.min(24 * 60, value));
}

function getMinutesInTimezone(timestampIso: string | null | undefined, timezone: string): number | undefined {
  if (!timestampIso) {
    return undefined;
  }
  const date = new Date(timestampIso);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  try {
    const parts = new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      hour: '2-digit',
      minute: '2-digit',
      hourCycle: 'h23'
    }).formatToParts(date);
    const hour = Number(parts.find((part) => part.type === 'hour')?.value);
    const minute = Number(parts.find((part) => part.type === 'minute')?.value);
    if (!Number.isFinite(hour) || !Number.isFinite(minute)) {
      return undefined;
    }
    return hour * 60 + minute;
  } catch {
    return undefined;
  }
}

function getDateIsoInTimezone(date: Date, timezone: string): string {
  try {
    const parts = new Intl.DateTimeFormat('en-CA', {
      timeZone: timezone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit'
    }).formatToParts(date);
    const year = parts.find((part) => part.type === 'year')?.value;
    const month = parts.find((part) => part.type === 'month')?.value;
    const day = parts.find((part) => part.type === 'day')?.value;
    if (year && month && day) {
      return `${year}-${month}-${day}`;
    }
  } catch {
    // Fall through to UTC fallback below.
  }
  return date.toISOString().slice(0, 10);
}

function formatTitleTime(totalMinutes: number): string {
  const minutesInDay = ((totalMinutes % (24 * 60)) + 24 * 60) % (24 * 60);
  const hour24 = Math.floor(minutesInDay / 60);
  const minute = minutesInDay % 60;
  const period = hour24 >= 12 ? 'pm' : 'am';
  const hour12 = hour24 % 12 || 12;
  return minute === 0
    ? `${hour12}${period}`
    : `${hour12}:${String(minute).padStart(2, '0')}${period}`;
}
