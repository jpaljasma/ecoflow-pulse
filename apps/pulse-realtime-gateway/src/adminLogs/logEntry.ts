import type { DecodedEnvelope } from '../live/envelopeCodec.js';
import type { AdminLogEntry, AdminLogFilters, AdminLogStatus } from './types.js';

const redacted = '<redacted>';
const sensitiveKeyPattern =
  /(access.?key|secret|token|jwt|password|credential|provider.?device.?id|serial|sn|ecoflow.?sn|email)/i;

export function adminLogEntryFromEnvelope(envelope: DecodedEnvelope, fallbackTs: number): AdminLogEntry {
  const receivedTs =
    envelope.ingestedTimeUnixMs || envelope.observedTimeUnixMs || envelope.deviceTimeUnixMs || fallbackTs;
  const payload = parsePayload(envelope.payload, envelope.payloadEncoding);
  const status = payload.ok ? deriveStatus(payload.value, envelope) : 'warning';
  const typeCode = envelope.typeCode || envelope.payloadType || 'telemetry';
  return {
    id: envelope.messageId || `${envelope.deviceId}:${receivedTs}:${typeCode}`,
    ts: envelope.observedTimeUnixMs || envelope.deviceTimeUnixMs || receivedTs,
    receivedTs,
    deviceId: envelope.deviceId,
    status,
    source: envelope.source || sourceFromKind(envelope.sourceKind),
    sourceKind: envelope.sourceKind || 'SOURCE_KIND_UNSPECIFIED',
    typeCode,
    summary: `${typeCode} ${status} for ${shortDevice(envelope.deviceId)}`,
    labels: redactLabels(envelope.labels),
    detail: redactObject({
      deviceId: envelope.deviceId,
      messageId: envelope.messageId,
      source: envelope.source,
      sourceKind: envelope.sourceKind,
      typeCode: envelope.typeCode,
      payloadType: envelope.payloadType,
      payloadVersion: envelope.payloadVersion,
      payloadEncoding: envelope.payloadEncoding,
      timestamps: {
        deviceTimeUnixMs: envelope.deviceTimeUnixMs,
        observedTimeUnixMs: envelope.observedTimeUnixMs,
        ingestedTimeUnixMs: envelope.ingestedTimeUnixMs
      },
      labels: envelope.labels,
      payload: payload.ok ? payload.value : { parseStatus: 'unavailable' }
    })
  };
}

export function matchesAdminLogFilters(entry: AdminLogEntry, filters: AdminLogFilters): boolean {
  if (filters.deviceIds.length > 0 && !filters.deviceIds.includes(entry.deviceId)) {
    return false;
  }
  if (filters.statuses.length > 0 && !filters.statuses.includes(entry.status)) {
    return false;
  }
  if (filters.providers.length > 0 && !matchesStringFilter(providerFromLabels(entry.labels), filters.providers)) {
    return false;
  }
  if (filters.sources.length > 0 && !filters.sources.includes(entry.source)) {
    return false;
  }
  if (!matchesTypeCode(entry.typeCode, filters)) {
    return false;
  }
  return true;
}

function matchesTypeCode(typeCode: string, filters: AdminLogFilters): boolean {
  if (filters.typeCodes.length === 0 && filters.typeCodeSuffixes.length === 0) {
    return true;
  }
  if (filters.typeCodes.includes(typeCode)) {
    return true;
  }
  const normalized = typeCode.trim().toLowerCase();
  return filters.typeCodeSuffixes.some((suffix) => normalized.endsWith(suffix.trim().toLowerCase()));
}

function providerFromLabels(labels: Record<string, string>): string {
  return labels.provider ?? labels.Provider ?? '';
}

function matchesStringFilter(value: string, filters: readonly string[]): boolean {
  const normalized = value.trim().toLowerCase();
  if (!normalized) {
    return false;
  }
  return filters.some((filter) => filter.trim().toLowerCase() === normalized);
}

export function redactObject(value: unknown): Record<string, unknown> {
  const sanitized = redactValue(value);
  return sanitized && typeof sanitized === 'object' && !Array.isArray(sanitized)
    ? (sanitized as Record<string, unknown>)
    : {};
}

function redactValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => redactValue(item));
  }
  if (!value || typeof value !== 'object') {
    return value;
  }
  const out: Record<string, unknown> = {};
  for (const [key, raw] of Object.entries(value)) {
    out[key] = sensitiveKeyPattern.test(key) ? redacted : redactValue(raw);
  }
  return out;
}

function redactLabels(labels: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [key, value] of Object.entries(labels)) {
    out[key] = sensitiveKeyPattern.test(key) ? redacted : value;
  }
  return out;
}

function parsePayload(payload: Uint8Array, encoding: string): { ok: true; value: unknown } | { ok: false } {
  if (!payload || payload.length === 0) {
    return { ok: false };
  }
  if (encoding && encoding !== 'PAYLOAD_ENCODING_JSON_UTF8') {
    return { ok: true, value: { encoding, bytes: payload.length } };
  }
  try {
    return { ok: true, value: JSON.parse(Buffer.from(payload).toString('utf8')) };
  } catch {
    return { ok: false };
  }
}

function deriveStatus(payload: unknown, envelope: DecodedEnvelope): AdminLogStatus {
  const rawStatus = lookupStatus(payload);
  if (rawStatus === 'error' || rawStatus === 'failed' || rawStatus === 'failure') {
    return 'error';
  }
  if (rawStatus === 'warn' || rawStatus === 'warning' || envelope.sourceKind === 'SOURCE_KIND_REPLAY') {
    return 'warning';
  }
  return 'ok';
}

function lookupStatus(payload: unknown): string {
  if (!payload || typeof payload !== 'object') {
    return '';
  }
  const record = payload as Record<string, unknown>;
  const direct = record.status ?? record.state ?? record.result;
  return typeof direct === 'string' ? direct.trim().toLowerCase() : '';
}

function sourceFromKind(kind: string): string {
  switch (kind) {
    case 'SOURCE_KIND_MQTT_STATUS':
      return 'mqtt-status';
    case 'SOURCE_KIND_MQTT_QUOTA':
      return 'mqtt';
    case 'SOURCE_KIND_REPLAY':
      return 'replay';
    default:
      return 'unknown';
  }
}

function shortDevice(deviceId: string): string {
  const clean = deviceId.trim();
  return clean.length <= 8 ? clean : clean.slice(0, 8);
}
