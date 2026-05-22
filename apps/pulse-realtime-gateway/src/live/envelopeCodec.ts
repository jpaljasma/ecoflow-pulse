import path from 'node:path';
import { fileURLToPath } from 'node:url';

import protobuf from 'protobufjs';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, '../../../../');
const envelopeProtoPath = path.join(projectRoot, 'proto/pulse/envelope/v1/envelope.proto');

type RawEnvelopeMessage = {
  deviceId?: string;
  ecoflowSn?: string;
  messageId?: string;
  ingestedTimeUnixMs?: number | string;
  observedTimeUnixMs?: number | string;
  deviceTimeUnixMs?: number | string;
  payload?: Uint8Array | Buffer | string;
  payloadEncoding?: string | number;
  sourceKind?: string | number;
  source?: string;
  typeCode?: string;
  payloadType?: string;
  payloadVersion?: number | string;
  labels?: Record<string, string>;
};

export type DecodedEnvelope = {
  deviceId: string;
  ecoflowSn: string;
  messageId: string;
  ingestedTimeUnixMs: number;
  observedTimeUnixMs: number;
  deviceTimeUnixMs: number;
  payload: Uint8Array;
  payloadEncoding: string;
  sourceKind: string;
  source: string;
  typeCode: string;
  payloadType: string;
  payloadVersion: number;
  labels: Record<string, string>;
};

const root = protobuf.loadSync(envelopeProtoPath);
const telemetryEnvelopeType = root.lookupType('pulse.envelope.v1.TelemetryEnvelope');

export function decodeEnvelope(data: Uint8Array): DecodedEnvelope | null {
  try {
    const decoded = telemetryEnvelopeType.decode(data);
    const object = telemetryEnvelopeType.toObject(decoded, {
      longs: Number,
      enums: String,
      bytes: Buffer,
      defaults: false
    }) as RawEnvelopeMessage;

    const payload = object.payload ? Buffer.from(object.payload) : Buffer.alloc(0);
    return {
      deviceId: normalizeString(object.deviceId),
      ecoflowSn: normalizeString(object.ecoflowSn),
      messageId: normalizeString(object.messageId),
      ingestedTimeUnixMs: normalizeInt(object.ingestedTimeUnixMs),
      observedTimeUnixMs: normalizeInt(object.observedTimeUnixMs),
      deviceTimeUnixMs: normalizeInt(object.deviceTimeUnixMs),
      payload,
      payloadEncoding: normalizeString(object.payloadEncoding),
      sourceKind: normalizeString(object.sourceKind),
      source: normalizeString(object.source),
      typeCode: normalizeString(object.typeCode),
      payloadType: normalizeString(object.payloadType),
      payloadVersion: normalizeInt(object.payloadVersion),
      labels: normalizeLabels(object.labels)
    };
  } catch {
    return null;
  }
}

function normalizeString(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

function normalizeInt(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.trunc(value);
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return Math.trunc(parsed);
    }
  }
  return 0;
}

function normalizeLabels(value: unknown): Record<string, string> {
  if (!value || typeof value !== 'object') {
    return {};
  }
  const out: Record<string, string> = {};
  for (const [key, raw] of Object.entries(value)) {
    const cleanKey = key.trim();
    if (!cleanKey || typeof raw !== 'string') {
      continue;
    }
    out[cleanKey] = raw.trim();
  }
  return out;
}
