import { describe, expect, it } from 'vitest';
import type { NatsConnection } from 'nats';

import { adminLogEntryFromEnvelope, matchesAdminLogFilters } from '../src/adminLogs/logEntry.js';
import { NatsAdminLogSource, type NatsConnectionFactory } from '../src/adminLogs/natsAdminLogSource.js';
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

  it('applies device, status, provider, source, and type filters server-side', () => {
    const entry = adminLogEntryFromEnvelope(sampleEnvelope(), 1772197190100);

    expect(
      matchesAdminLogFilters(entry, {
        deviceIds: ['dev-1'],
        statuses: ['ok'],
        providers: ['ecoflow'],
        sources: ['mqtt'],
        typeCodes: ['quota'],
        typeCodeSuffixes: []
      })
    ).toBe(true);
    expect(
      matchesAdminLogFilters(entry, {
        deviceIds: ['dev-2'],
        statuses: [],
        providers: [],
        sources: [],
        typeCodes: [],
        typeCodeSuffixes: []
      })
    ).toBe(false);
    expect(
      matchesAdminLogFilters(entry, {
        deviceIds: [],
        statuses: ['error'],
        providers: [],
        sources: [],
        typeCodes: [],
        typeCodeSuffixes: []
      })
    ).toBe(false);
    expect(
      matchesAdminLogFilters(entry, {
        deviceIds: [],
        statuses: [],
        providers: ['pecron'],
        sources: [],
        typeCodes: [],
        typeCodeSuffixes: []
      })
    ).toBe(false);
  });

  it('matches type code suffix filters without enumerating every status payload', () => {
    const entry = adminLogEntryFromEnvelope(sampleEnvelope({ typeCode: 'mpptStatus' }), 1772197190100);

    expect(
      matchesAdminLogFilters(entry, {
        deviceIds: [],
        statuses: [],
        providers: [],
        sources: [],
        typeCodes: [],
        typeCodeSuffixes: ['Status']
      })
    ).toBe(true);
    expect(
      matchesAdminLogFilters(entry, {
        deviceIds: [],
        statuses: [],
        providers: [],
        sources: [],
        typeCodes: [],
        typeCodeSuffixes: ['Info']
      })
    ).toBe(false);
  });

  it('uses explicit ack policy for JetStream replay pull consumers', async () => {
    let ackPolicy = '';
    const connect: NatsConnectionFactory = async () => ({
      jetstream: () => ({
        pullSubscribe: async (_subject: string, opts: { getOpts: () => { config: { ack_policy?: string } } }) => {
          ackPolicy = opts.getOpts().config.ack_policy ?? '';
          return emptyReplaySubscription();
        }
      }),
      subscribe: () => emptyReplaySubscription(),
      drain: async () => undefined
    }) as unknown as NatsConnection;
    const source = new NatsAdminLogSource(
      {
        urls: ['nats://127.0.0.1:4222'],
        subjectPrefix: 'pulse.telemetry',
        streamName: 'PULSE_TELEMETRY_INGEST'
      },
      connect
    );

    const terminal = new Promise<void>((resolve, reject) => {
      source.subscribe({
        subscriptionId: 'logs-1',
        filters: { deviceIds: [], statuses: [], providers: [], sources: [], typeCodes: [], typeCodeSuffixes: [] },
        replayLimit: 10,
        replaySinceUnixMs: Date.now() - 60_000,
        requestId: 'test-request',
        onEntry: () => undefined,
        onReplayDone: () => undefined,
        onStatus: ({ state, message }) => {
          if (state === 'live') {
            resolve();
          }
          if (state === 'error') {
            reject(new Error(message ?? 'unexpected stream error'));
          }
        }
      });
    });

    await terminal;
    await source.close();
    expect(ackPolicy).toBe('explicit');
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

function emptyReplaySubscription() {
  return {
    pull: () => undefined,
    destroy: async () => undefined,
    unsubscribe: () => undefined,
    async *[Symbol.asyncIterator]() {}
  };
}
