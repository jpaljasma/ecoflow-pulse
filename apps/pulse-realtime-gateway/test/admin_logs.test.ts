import { describe, expect, it } from 'vitest';

import { adminLogEntryFromEnvelope, matchesAdminLogFilters } from '../src/adminLogs/logEntry.js';
import type { DecodedEnvelope } from '../src/live/envelopeCodec.js';

describe('admin realtime log entries', () => {
  it('redacts sensitive envelope fields from emitted detail', () => {
    const entry = adminLogEntryFromEnvelope(
      sampleEnvelope({
        payload: Buffer.from(
          JSON.stringify({
            status: 'ok',
            email: 'operator@example.test',
            params: {
              soc: 54,
              serialNumber: 'SENSITIVE-SERIAL',
              credentialId: 'credential-1'
            }
          })
        ),
        labels: {
          provider: 'ecoflow',
          providerDeviceId: 'SENSITIVE-PROVIDER-ID',
          userEmail: 'operator@example.test'
        }
      }),
      1772197190100
    );

    expect(JSON.stringify(entry)).not.toContain('operator@example.test');
    expect(JSON.stringify(entry)).not.toContain('SENSITIVE-SERIAL');
    expect(JSON.stringify(entry)).not.toContain('SENSITIVE-PROVIDER-ID');
    expect(JSON.stringify(entry)).not.toContain('credential-1');
    expect(entry.labels).toEqual({
      provider: 'ecoflow',
      providerDeviceId: '<redacted>',
      userEmail: '<redacted>'
    });
    expect(entry.status).toBe('ok');
  });

  it('applies device, status, source, and type filters server-side', () => {
    const entry = adminLogEntryFromEnvelope(sampleEnvelope(), 1772197190100);

    expect(
      matchesAdminLogFilters(entry, {
        deviceIds: ['dev-1'],
        statuses: ['ok'],
        sources: ['mqtt'],
        typeCodes: ['quota']
      })
    ).toBe(true);
    expect(
      matchesAdminLogFilters(entry, {
        deviceIds: ['dev-2'],
        statuses: [],
        sources: [],
        typeCodes: []
      })
    ).toBe(false);
    expect(
      matchesAdminLogFilters(entry, {
        deviceIds: [],
        statuses: ['error'],
        sources: [],
        typeCodes: []
      })
    ).toBe(false);
  });
});

function sampleEnvelope(overrides: Partial<DecodedEnvelope> = {}): DecodedEnvelope {
  return {
    deviceId: 'dev-1',
    ecoflowSn: 'SENSITIVE-SERIAL',
    messageId: 'message-1',
    ingestedTimeUnixMs: 1772197190100,
    observedTimeUnixMs: 1772197190000,
    deviceTimeUnixMs: 1772197189000,
    payload: Buffer.from(JSON.stringify({ status: 'ok', params: { soc: 54 } })),
    payloadEncoding: 'PAYLOAD_ENCODING_JSON_UTF8',
    sourceKind: 'SOURCE_KIND_MQTT_QUOTA',
    source: 'mqtt',
    typeCode: 'quota',
    payloadType: 'provider.telemetry',
    payloadVersion: 1,
    labels: { provider: 'ecoflow' },
    ...overrides
  };
}
