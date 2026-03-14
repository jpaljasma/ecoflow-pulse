import timezoneCatalog from '@ecoflow-pulse/api-types/src/timezones.json';

const IANA_TIMEZONES = Array.from(new Set((timezoneCatalog as string[]).map((value) => value.trim()).filter(Boolean)));

function normalizedSearchValue(value: string): string {
  return value.trim().toLowerCase().replaceAll('_', ' ');
}

export function isValidIanaTimezone(value: string): boolean {
  const timezone = value.trim();
  if (!timezone) {
    return false;
  }
  if (IANA_TIMEZONES.includes(timezone)) {
    return true;
  }
  try {
    new Intl.DateTimeFormat(undefined, { timeZone: timezone });
    return true;
  } catch {
    return false;
  }
}

export function detectCurrentTimeZone(): string {
  try {
    const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (typeof timezone !== 'string') {
      return '';
    }
    const trimmed = timezone.trim();
    return isValidIanaTimezone(trimmed) ? trimmed : '';
  } catch {
    return '';
  }
}

export function resolveProfileTimezone(preferred: string, fallback: string): string {
  const candidates = [preferred, fallback, 'UTC'];
  for (const candidate of candidates) {
    const trimmed = candidate.trim();
    if (isValidIanaTimezone(trimmed)) {
      return trimmed;
    }
  }
  return 'UTC';
}

export function searchProfileTimezones(query: string, preferred: readonly string[] = []): string[] {
  const normalizedQuery = normalizedSearchValue(query);
  const pool = Array.from(new Set([...preferred.map((value) => value.trim()).filter(Boolean), ...IANA_TIMEZONES]));
  if (!normalizedQuery) {
    return pool;
  }
  const exact: string[] = [];
  const startsWith: string[] = [];
  const includes: string[] = [];

  for (const timezone of pool) {
    const normalizedTimezone = normalizedSearchValue(timezone);
    if (normalizedTimezone === normalizedQuery) {
      exact.push(timezone);
      continue;
    }
    if (normalizedTimezone.startsWith(normalizedQuery)) {
      startsWith.push(timezone);
      continue;
    }
    if (normalizedTimezone.includes(normalizedQuery)) {
      includes.push(timezone);
    }
  }

  return [...exact, ...startsWith, ...includes];
}
