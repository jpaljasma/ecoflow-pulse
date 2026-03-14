import timezoneCatalog from '@ecoflow-pulse/api-types/src/timezones.json' with { type: 'json' };

const IANA_TIMEZONES = new Set((timezoneCatalog as string[]).map((value) => value.trim()).filter(Boolean));

export function isValidIanaTimezone(value: string): boolean {
  const timezone = value.trim();
  if (!timezone) {
    return false;
  }
  if (IANA_TIMEZONES.has(timezone)) {
    return true;
  }
  try {
    new Intl.DateTimeFormat(undefined, { timeZone: timezone });
    return true;
  } catch {
    return false;
  }
}
