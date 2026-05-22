import { Buffer } from 'node:buffer';
import { bench, describe } from 'vitest';

import { adminLogEntryFromEnvelope, matchesAdminLogFilters } from '../src/adminLogs/logEntry.js';
import type { DecodedEnvelope } from '../src/live/envelopeCodec.js';

const payload = Buffer.from(JSON.stringify({
  status: 'ok',
  params: {
    soc: 67,
    wattsInSum: 420,
    nested: {
      token: 'redacted-by-benchmark'
    }
  }
}));

const envelope: DecodedEnvelope = {
  messageId: 'msg-1',
  deviceId: 'device-1',
  ecoflowSn: 'redacted-by-benchmark',
  sourceKind: 'SOURCE_KIND_MQTT_QUOTA',
  source: 'mqtt',
  typeCode: 'quota',
  payloadType: 'quota',
  payloadVersion: 1,
  payloadEncoding: 'PAYLOAD_ENCODING_JSON_UTF8',
  payload,
  deviceTimeUnixMs: 1772197190000,
  observedTimeUnixMs: 1772197190010,
  ingestedTimeUnixMs: 1772197190020,
  labels: {
    provider: 'ecoflow',
    providerDeviceId: 'sensitive'
  }
};

describe('admin log gateway performance', () => {
  bench('adminLogEntryFromEnvelope redacts normalized envelope', () => {
    adminLogEntryFromEnvelope(envelope, 1772197190100);
  });

  bench('matchesAdminLogFilters checks populated filters', () => {
    const entry = adminLogEntryFromEnvelope(envelope, 1772197190100);
    matchesAdminLogFilters(entry, {
      deviceIds: ['device-1'],
      statuses: ['ok'],
      providers: ['ecoflow'],
      sources: ['mqtt'],
      typeCodes: ['quota']
    });
  });
});
