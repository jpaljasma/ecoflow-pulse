export const DEFAULT_TELEMETRY_SUBJECT_PREFIX = 'pulse.telemetry';

export function ingestWildcardSubject(prefix: string): string {
  const clean = prefix.trim() || DEFAULT_TELEMETRY_SUBJECT_PREFIX;
  return `${clean}.ingest.*`;
}
