import type { IncomingMessage } from 'node:http';
import type { AddressInfo } from 'node:net';

import { afterEach, describe, expect, it, vi } from 'vitest';
import WebSocket from 'ws';
import type { RawData } from 'ws';

import { buildApp } from '../src/app.js';
import type { AppConfig } from '../src/config.js';
import type {
  LiveDelta,
  LiveHeartbeat,
  LiveSnapshot,
  LiveSubscription,
  LiveTelemetryClient,
  SubscribeInput
} from '../src/live/liveTelemetryClient.js';
import type {
  AdminLogEntry,
  AdminLogSource,
  AdminLogSubscribeInput,
  AdminLogSubscription
} from '../src/adminLogs/types.js';
import type { DeviceAuthorizer } from '../src/controlplane/deviceAuthorizer.js';
import { realtimeMetrics } from '../src/metrics.js';

type SubscriptionRecord = {
  input: SubscribeInput;
  closed: boolean;
};

class FakeLiveClient implements LiveTelemetryClient {
  readonly subscriptions = new Map<string, SubscriptionRecord>();
  readonly subscribeCalls: SubscribeInput[] = [];

  subscribe(input: SubscribeInput): LiveSubscription {
    this.subscribeCalls.push(input);
    const record: SubscriptionRecord = {
      input,
      closed: false
    };
    this.subscriptions.set(input.deviceId, record);
    return {
      close: () => {
        record.closed = true;
        this.subscriptions.delete(input.deviceId);
      }
    };
  }

  close(): void {
    this.subscriptions.clear();
  }

  emitSnapshot(deviceId: string, snapshot: Partial<LiveSnapshot> = {}): void {
    this.subscriptions.get(deviceId)?.input.onSnapshot({
      deviceId,
      cursor: { seq: 1, tsUnixMs: 1000 },
      metrics: { 'params.soc': 25 },
      ...snapshot
    });
  }

  emitDelta(deviceId: string, delta: Partial<LiveDelta> = {}): void {
    this.subscriptions.get(deviceId)?.input.onDelta({
      deviceId,
      cursor: { seq: 2, tsUnixMs: 2000 },
      changed: { 'params.wattsInSum': 120, 'params.pv1ChargeWatts': 30 },
      cleared: [],
      ...delta
    });
  }

  emitHeartbeat(deviceId: string, heartbeat: Partial<LiveHeartbeat> = {}): void {
    this.subscriptions.get(deviceId)?.input.onHeartbeat({
      deviceId,
      cursor: { seq: 3, tsUnixMs: 3000 },
      ...heartbeat
    });
  }

  emitClose(deviceId: string, error?: Error & { code?: number }): void {
    this.subscriptions.get(deviceId)?.input.onClose(error);
  }
}

type LogSubscriptionRecord = {
  input: AdminLogSubscribeInput;
  closed: boolean;
};

class FakeAdminLogSource implements AdminLogSource {
  readonly subscribeCalls: AdminLogSubscribeInput[] = [];
  readonly subscriptions = new Map<string, LogSubscriptionRecord>();
  replayEntries: AdminLogEntry[] = [];

  subscribe(input: AdminLogSubscribeInput): AdminLogSubscription {
    this.subscribeCalls.push(input);
    const record: LogSubscriptionRecord = { input, closed: false };
    this.subscriptions.set(input.subscriptionId, record);
    input.onStatus({ state: 'replay' });
    for (const entry of this.replayEntries) {
      input.onEntry(entry);
    }
    input.onReplayDone({ replayed: this.replayEntries.length });
    input.onStatus({ state: 'live' });
    return {
      close: () => {
        record.closed = true;
        this.subscriptions.delete(input.subscriptionId);
      }
    };
  }

  emit(subscriptionId: string, entry: AdminLogEntry): void {
    this.subscriptions.get(subscriptionId)?.input.onEntry(entry);
  }

  close(): void {
    this.subscriptions.clear();
  }
}

class StallingAdminLogSource implements AdminLogSource {
  readonly subscribeCalls: AdminLogSubscribeInput[] = [];

  subscribe(input: AdminLogSubscribeInput): AdminLogSubscription {
    this.subscribeCalls.push(input);
    return {
      close: () => {}
    };
  }

  close(): void {}
}

class FakeDeviceAuthorizer implements DeviceAuthorizer {
  constructor(private readonly deviceIds: string[]) {}

  async authorize(input: { deviceId: string }): Promise<{ canonicalDeviceId: string }> {
    if (this.deviceIds.includes(input.deviceId)) {
      return { canonicalDeviceId: input.deviceId };
    }
    throw Object.assign(new Error('device access denied'), { code: 7 });
  }

  async listAuthorizedDevices(): Promise<{ deviceIds: string[] }> {
    return { deviceIds: this.deviceIds };
  }

  close(): void {}
}

function baseConfig(): AppConfig {
  return {
    host: '127.0.0.1',
    port: 0,
    grpcApiAddr: '127.0.0.1:9090',
    grpcDeadlineMs: 2500,
    reconnectBackoff: {
      baseMs: 20,
      maxMs: 40
    },
    natsUrls: ['nats://127.0.0.1:4222'],
    valkey: {
      addrs: ['127.0.0.1:6379'],
      keyPrefix: 'pulse:projection'
    },
    telemetrySubjectPrefix: 'pulse.telemetry',
    logs: {
      streamName: 'PULSE_TELEMETRY_INGEST',
      replayLimit: 200,
      replayWindowMs: 300000,
      devAdminEnabled: false
    },
    delivery: {
      fastIntervalMs: 250,
      steadyIntervalMs: 500,
      slowIntervalMs: 1000,
      highWatermark: 8,
      bufferedAmountHighWaterBytes: 262144,
      quietTicksToRecover: 4
    },
    auth: { mode: 'noop', allowMissingJwt: true }
  };
}

afterEach(() => {
  vi.restoreAllMocks();
  realtimeMetrics.reset();
});

type MessageCollector = {
  queue: unknown[];
  waiters: Array<(message: unknown) => void>;
};

const messageCollectors = new WeakMap<WebSocket, MessageCollector>();

describe('pulse-realtime-gateway', () => {
  it('sends snapshot-first telemetry messages for subscribed devices', async () => {
    const client = new FakeLiveClient();
    const app = buildApp(baseConfig(), client);
    const ws = await openWebSocket(app, '/ws');

    ws.send(JSON.stringify({ type: 'subscribe', deviceIds: ['dev-1'] }));
    await waitFor(() => client.subscriptions.has('dev-1'));
    client.emitSnapshot('dev-1', {
      metrics: {
        'params.f32ShowSoc': 54.8,
        'params.inLvMpptPwr': 22.63,
        'params.wattsInSum': 29.39,
        'params.batAmp': -1.6,
        'params.batVol': 54.7,
        'params.cellTemp.0': 19,
        'params.cellTemp.1': 21,
        'params.cellTemp.2': 20
      }
    });

    expect(await nextMessage(ws)).toEqual({
      type: 'device_status',
      deviceId: 'dev-1',
      ts: 1000,
      online: true
    });
    expect(await nextMessage(ws)).toEqual({
      type: 'telemetry',
      deviceId: 'dev-1',
      ts: 1000,
      metrics: {
        soc: 54.8,
        pvW: 22.63,
        loadW: 0,
        batteryW: 29.39,
        tempC: 20,
        acW: 6.760000000000002,
        dcW: 0
      },
      detail: {
        signals: {
          solarChargingOn: true
        },
        solarPorts: [{ id: 'pv-low', name: 'PV Low', state: 'charging', watts: 22.63 }]
      }
    });

    await closeWebSocket(ws);
    await app.close();
  });

  it('merges deltas and handles unsubscribe', async () => {
    const client = new FakeLiveClient();
    const app = buildApp(baseConfig(), client);
    const ws = await openWebSocket(app, '/ws');

    ws.send(JSON.stringify({ type: 'subscribe', deviceIds: ['dev-1'] }));
    await waitFor(() => client.subscriptions.has('dev-1'));
    client.emitSnapshot('dev-1', {
      metrics: {
        'params.f32ShowSoc': 25,
        'params.wattsOutSum': 10
      }
    });
    await nextMessage(ws);
    await nextMessage(ws);

    client.emitDelta('dev-1', {
      changed: {
        'params.wattsInSum': 120,
        'params.pv1ChargeWatts': 30,
        'params.outUsb1Pwr': 8
      },
      cleared: ['params.wattsOutSum']
    });

    expect(await nextMessage(ws)).toEqual({
      type: 'telemetry',
      deviceId: 'dev-1',
      ts: 2000,
      metrics: {
        soc: 25,
        pvW: 30,
        loadW: 8,
        batteryW: 112,
        tempC: 0,
        acW: 90,
        dcW: 8
      },
      detail: {
        signals: {
          dcOn: true,
          usbOn: true,
          solarChargingOn: true
        }
      }
    });

    ws.send(JSON.stringify({ type: 'unsubscribe', deviceIds: ['dev-1'] }));
    expect(await nextMessage(ws)).toEqual({
      type: 'device_status',
      deviceId: 'dev-1',
      ts: expect.any(Number),
      online: false
    });
    expect(client.subscriptions.has('dev-1')).toBe(false);

    await closeWebSocket(ws);
    await app.close();
  });

  it('rejects websocket upgrade when auth prevalidation fails', async () => {
    const client = new FakeLiveClient();
    const app = buildApp(baseConfig(), client, {
      wsPreValidation: async (_request, reply) => {
        void reply.code(401).send({ error: 'missing_bearer_token' });
      }
    });

    await expect(openWebSocket(app, '/ws')).rejects.toThrow(/401/);
    await app.close();
  });

  it('exposes websocket auth metrics for scraping', async () => {
    const client = new FakeLiveClient();
    const app = buildApp(baseConfig(), client, {
      wsPreValidation: async (_request, reply) => {
        realtimeMetrics.recordAuthOutcome('missing_bearer_token');
        void reply.code(401).send({ error: 'missing_bearer_token' });
      }
    });

    await expect(openWebSocket(app, '/ws')).rejects.toThrow(/401/);

    const metrics = await app.inject({
      method: 'GET',
      url: '/metrics'
    });

    expect(metrics.statusCode).toBe(200);
    expect(metrics.headers['content-type']).toContain('text/plain');
    expect(metrics.body).toContain('pulse_realtime_ws_auth_total');
    expect(metrics.body).toContain('outcome="missing_bearer_token"');

    await app.close();
  });

  it('exposes admin log subscription lifecycle metrics for scraping', async () => {
    const client = new FakeLiveClient();
    const logs = new FakeAdminLogSource();
    logs.replayEntries = [sampleLogEntry({ id: 'replay-1', ts: 1000 })];
    const app = buildApp(baseConfig(), client, {
      logSource: logs,
      wsPreValidation: async (request) => {
        request.auth = {
          subject: 'admin-user',
          email: '',
          roles: ['viewer', 'admin'],
          rawJwt: 'admin-token'
        };
        request.wsAuthHeader = 'Bearer admin-token';
      }
    });
    const ws = await openWebSocket(app, '/ws');

    ws.send(JSON.stringify({ type: 'logs_subscribe', subscriptionId: 'logs-1' }));

    expect(await nextMessage(ws)).toMatchObject({
      type: 'logs_status',
      subscriptionId: 'logs-1',
      state: 'replay'
    });
    expect(await nextMessage(ws)).toMatchObject({
      type: 'log_entry',
      subscriptionId: 'logs-1'
    });
    expect(await nextMessage(ws)).toMatchObject({
      type: 'logs_replay_done',
      subscriptionId: 'logs-1',
      replayed: 1
    });
    expect(await nextMessage(ws)).toMatchObject({
      type: 'logs_status',
      subscriptionId: 'logs-1',
      state: 'live'
    });

    const metrics = await app.inject({
      method: 'GET',
      url: '/metrics'
    });

    expect(metrics.statusCode).toBe(200);
    expect(metrics.body).toContain('pulse_realtime_ws_log_subscriptions_total');
    expect(metrics.body).toContain('outcome="requested"');
    expect(metrics.body).toContain('outcome="status_replay"');
    expect(metrics.body).toContain('outcome="authorized"');
    expect(metrics.body).toContain('outcome="subscribed"');
    expect(metrics.body).toContain('outcome="replay_done"');
    expect(metrics.body).toContain('outcome="status_live"');

    await closeWebSocket(ws);
    await app.close();
  });

  it('responds to client heartbeat pings so idle connections stay fresh', async () => {
    const client = new FakeLiveClient();
    const app = buildApp(baseConfig(), client);
    const ws = await openWebSocket(app, '/ws');

    ws.send(JSON.stringify({ type: 'ping', ts: 123 }));

    expect(await nextMessage(ws)).toEqual({
      type: 'pong',
      ts: expect.any(Number)
    });

    await closeWebSocket(ws);
    await app.close();
  });

  it('rejects log subscriptions for non-admin users without device access', async () => {
    const client = new FakeLiveClient();
    const logs = new FakeAdminLogSource();
    const app = buildApp(baseConfig(), client, {
      logSource: logs,
      wsPreValidation: async (request) => {
        request.auth = {
          subject: 'viewer-user',
          email: '',
          roles: ['viewer'],
          rawJwt: 'viewer-token'
        };
      }
    });
    const ws = await openWebSocket(app, '/ws');

    ws.send(JSON.stringify({ type: 'logs_subscribe', subscriptionId: 'logs-1' }));

    expect(await nextMessage(ws)).toEqual({
      type: 'logs_status',
      subscriptionId: 'logs-1',
      ts: expect.any(Number),
      state: 'forbidden',
      message: 'device log access required'
    });
    expect(logs.subscribeCalls).toHaveLength(0);

    await closeWebSocket(ws);
    await app.close();
  });

  it('acknowledges owner log subscriptions before device authorization finishes', async () => {
    const client = new FakeLiveClient();
    const logs = new FakeAdminLogSource();
    let resolveAuthorizedDevices: ((value: { deviceIds: string[] }) => void) | undefined;
    const deviceAuthorizer: DeviceAuthorizer = {
      async authorize(input: { deviceId: string }) {
        return { canonicalDeviceId: input.deviceId };
      },
      async listAuthorizedDevices() {
        return await new Promise<{ deviceIds: string[] }>((resolve) => {
          resolveAuthorizedDevices = resolve;
        });
      },
      close() {}
    };
    const app = buildApp(baseConfig(), client, {
      logSource: logs,
      deviceAuthorizer,
      wsPreValidation: async (request) => {
        request.auth = {
          subject: 'owner-user',
          email: '',
          roles: ['viewer'],
          rawJwt: 'owner-token'
        };
        request.wsAuthHeader = 'Bearer owner-token';
      }
    });
    const ws = await openWebSocket(app, '/ws');

    try {
      ws.send(JSON.stringify({ type: 'logs_subscribe', subscriptionId: 'logs-1' }));

      expect(await nextMessageWithin(ws, 100)).toMatchObject({
        type: 'logs_status',
        subscriptionId: 'logs-1',
        state: 'replay'
      });
      expect(logs.subscribeCalls).toHaveLength(0);

      resolveAuthorizedDevices?.({ deviceIds: ['dev-owned'] });
      await waitFor(() => logs.subscribeCalls.length === 1);
    } finally {
      resolveAuthorizedDevices?.({ deviceIds: ['dev-owned'] });
      await closeWebSocket(ws);
      await app.close();
    }
  });

  it('acknowledges admin log subscriptions before the log source emits status', async () => {
    const client = new FakeLiveClient();
    const logs = new StallingAdminLogSource();
    const app = buildApp(baseConfig(), client, {
      logSource: logs,
      wsPreValidation: async (request) => {
        request.auth = {
          subject: 'admin-user',
          email: '',
          roles: ['viewer', 'admin'],
          rawJwt: 'admin-token'
        };
        request.wsAuthHeader = 'Bearer admin-token';
      }
    });
    const ws = await openWebSocket(app, '/ws');

    try {
      ws.send(JSON.stringify({ type: 'logs_subscribe', subscriptionId: 'logs-1' }));

      expect(await nextMessageWithin(ws, 100)).toMatchObject({
        type: 'logs_status',
        subscriptionId: 'logs-1',
        state: 'replay'
      });
      expect(logs.subscribeCalls).toHaveLength(1);
    } finally {
      await closeWebSocket(ws);
      await app.close();
    }
  });

  it('scopes owner log subscriptions to authorized devices', async () => {
    const client = new FakeLiveClient();
    const logs = new FakeAdminLogSource();
    const app = buildApp(baseConfig(), client, {
      logSource: logs,
      deviceAuthorizer: new FakeDeviceAuthorizer(['dev-owned']),
      wsPreValidation: async (request) => {
        request.auth = {
          subject: 'owner-user',
          email: '',
          roles: ['viewer'],
          rawJwt: 'owner-token'
        };
        request.wsAuthHeader = 'Bearer owner-token';
      }
    });
    const ws = await openWebSocket(app, '/ws');

    ws.send(
      JSON.stringify({
        type: 'logs_subscribe',
        subscriptionId: 'logs-1',
        filters: {
          deviceIds: ['dev-owned', 'dev-other'],
          statuses: ['ok'],
          providers: ['ecoflow'],
          sources: ['mqtt'],
          typeCodes: [],
          typeCodeSuffixes: ['Status']
        }
      })
    );

    expect(await nextMessage(ws)).toMatchObject({
      type: 'logs_status',
      subscriptionId: 'logs-1',
      state: 'replay'
    });
    expect(logs.subscribeCalls[0]).toMatchObject({
      subscriptionId: 'logs-1',
      authHeader: 'Bearer owner-token',
      filters: {
        deviceIds: ['dev-owned'],
        statuses: ['ok'],
        providers: ['ecoflow'],
        sources: ['mqtt'],
        typeCodes: [],
        typeCodeSuffixes: ['Status']
      }
    });

    await closeWebSocket(ws);
    await app.close();
  });

  it('rejects owner log subscriptions scoped only to unauthorized devices', async () => {
    const client = new FakeLiveClient();
    const logs = new FakeAdminLogSource();
    const app = buildApp(baseConfig(), client, {
      logSource: logs,
      deviceAuthorizer: new FakeDeviceAuthorizer(['dev-owned']),
      wsPreValidation: async (request) => {
        request.auth = {
          subject: 'owner-user',
          email: '',
          roles: ['viewer'],
          rawJwt: 'owner-token'
        };
        request.wsAuthHeader = 'Bearer owner-token';
      }
    });
    const ws = await openWebSocket(app, '/ws');

    ws.send(
      JSON.stringify({
        type: 'logs_subscribe',
        subscriptionId: 'logs-1',
        filters: {
          deviceIds: ['dev-other'],
          statuses: [],
          providers: [],
          sources: [],
          typeCodes: []
        }
      })
    );

    expect(await nextMessage(ws)).toMatchObject({
      type: 'logs_status',
      subscriptionId: 'logs-1',
      state: 'replay'
    });
    expect(await nextMessage(ws)).toEqual({
      type: 'logs_status',
      subscriptionId: 'logs-1',
      ts: expect.any(Number),
      state: 'forbidden',
      message: 'device log access required'
    });
    expect(logs.subscribeCalls).toHaveLength(0);

    await closeWebSocket(ws);
    await app.close();
  });

  it('limits the admin log dev override to noop auth mode', async () => {
    const keycloakLogs = new FakeAdminLogSource();
    const keycloakConfig = baseConfig();
    keycloakConfig.auth = {
      mode: 'keycloak',
      issuerUrl: 'https://issuer.example/realms/pulse',
      audience: 'pulse',
      jwksUrl: '',
      allowMissingJwt: false
    };
    keycloakConfig.logs.devAdminEnabled = true;
    const keycloakApp = buildApp(keycloakConfig, new FakeLiveClient(), {
      logSource: keycloakLogs,
      wsPreValidation: async (request) => {
        request.auth = {
          subject: 'viewer-user',
          email: '',
          roles: ['viewer'],
          rawJwt: 'viewer-token'
        };
      }
    });
    const keycloakWs = await openWebSocket(keycloakApp, '/ws');

    keycloakWs.send(JSON.stringify({ type: 'logs_subscribe', subscriptionId: 'logs-1' }));

    expect(await nextMessage(keycloakWs)).toMatchObject({
      type: 'logs_status',
      subscriptionId: 'logs-1',
      state: 'forbidden'
    });
    expect(keycloakLogs.subscribeCalls).toHaveLength(0);
    await closeWebSocket(keycloakWs);
    await keycloakApp.close();

    const noopLogs = new FakeAdminLogSource();
    const noopConfig = baseConfig();
    noopConfig.logs.devAdminEnabled = true;
    const noopApp = buildApp(noopConfig, new FakeLiveClient(), { logSource: noopLogs });
    const noopWs = await openWebSocket(noopApp, '/ws');

    noopWs.send(JSON.stringify({ type: 'logs_subscribe', subscriptionId: 'logs-1' }));

    expect(await nextMessage(noopWs)).toMatchObject({
      type: 'logs_status',
      subscriptionId: 'logs-1',
      state: 'replay'
    });
    expect(noopLogs.subscribeCalls).toHaveLength(1);
    await closeWebSocket(noopWs);
    await noopApp.close();
  });

  it('streams replayed admin logs before live entries and releases on unsubscribe', async () => {
    const client = new FakeLiveClient();
    const logs = new FakeAdminLogSource();
    const replayEntry = sampleLogEntry({ id: 'replay-1', ts: 1000 });
    const liveEntry = sampleLogEntry({ id: 'live-1', ts: 2000 });
    logs.replayEntries = [replayEntry];
    const app = buildApp(baseConfig(), client, {
      logSource: logs,
      wsPreValidation: async (request) => {
        request.auth = {
          subject: 'admin-user',
          email: '',
          roles: ['viewer', 'admin'],
          rawJwt: 'admin-token'
        };
        request.wsAuthHeader = 'Bearer admin-token';
      }
    });
    const ws = await openWebSocket(app, '/ws');

    ws.send(
      JSON.stringify({
        type: 'logs_subscribe',
        subscriptionId: 'logs-1',
        replayLimit: 50,
        replaySinceUnixMs: 500,
        filters: {
          deviceIds: ['dev-1'],
          statuses: ['ok'],
          providers: ['ecoflow'],
          sources: ['mqtt'],
          typeCodes: ['quota']
        }
      })
    );

    expect(await nextMessage(ws)).toEqual({
      type: 'logs_status',
      subscriptionId: 'logs-1',
      ts: expect.any(Number),
      state: 'replay'
    });
    expect(await nextMessage(ws)).toEqual({
      type: 'log_entry',
      subscriptionId: 'logs-1',
      entry: replayEntry
    });
    expect(await nextMessage(ws)).toEqual({
      type: 'logs_replay_done',
      subscriptionId: 'logs-1',
      ts: expect.any(Number),
      replayed: 1
    });
    expect(await nextMessage(ws)).toEqual({
      type: 'logs_status',
      subscriptionId: 'logs-1',
      ts: expect.any(Number),
      state: 'live'
    });
    expect(logs.subscribeCalls[0]).toMatchObject({
      subscriptionId: 'logs-1',
      replayLimit: 50,
      replaySinceUnixMs: 500,
      authHeader: 'Bearer admin-token',
      filters: {
        deviceIds: ['dev-1'],
        statuses: ['ok'],
        providers: ['ecoflow'],
        sources: ['mqtt'],
        typeCodes: ['quota']
      }
    });

    logs.emit('logs-1', liveEntry);
    expect(await nextMessage(ws)).toEqual({
      type: 'log_entry',
      subscriptionId: 'logs-1',
      entry: liveEntry
    });

    ws.send(JSON.stringify({ type: 'logs_unsubscribe', subscriptionId: 'logs-1' }));
    expect(await nextMessage(ws)).toEqual({
      type: 'logs_status',
      subscriptionId: 'logs-1',
      ts: expect.any(Number),
      state: 'closed'
    });
    expect(logs.subscriptions.has('logs-1')).toBe(false);

    await closeWebSocket(ws);
    await app.close();
  });

  it('releases admin log subscriptions when websocket sessions close', async () => {
    const client = new FakeLiveClient();
    const logs = new FakeAdminLogSource();
    const app = buildApp(baseConfig(), client, {
      logSource: logs,
      wsPreValidation: async (request) => {
        request.auth = {
          subject: 'admin-user',
          email: '',
          roles: ['admin'],
          rawJwt: 'admin-token'
        };
      }
    });
    const ws = await openWebSocket(app, '/ws');

    ws.send(JSON.stringify({ type: 'logs_subscribe', subscriptionId: 'logs-1' }));
    await waitFor(() => logs.subscriptions.has('logs-1'));

    await closeWebSocket(ws);
    await waitFor(() => !logs.subscriptions.has('logs-1'));
    expect(logs.subscriptions.has('logs-1')).toBe(false);

    await app.close();
  });

  it('reconnects device streams after retryable errors', async () => {
    const client = new FakeLiveClient();
    const app = buildApp(baseConfig(), client);
    const ws = await openWebSocket(app, '/ws');

    ws.send(JSON.stringify({ type: 'subscribe', deviceIds: ['dev-1'] }));
    await waitFor(() => client.subscribeCalls.length === 1);
    expect(client.subscribeCalls).toHaveLength(1);

    client.emitClose('dev-1', Object.assign(new Error('unavailable'), { code: 14 }));
    expect(await nextMessage(ws)).toEqual({
      type: 'device_status',
      deviceId: 'dev-1',
      ts: expect.any(Number),
      online: false
    });

    await waitFor(() => client.subscribeCalls.length === 2);
    expect(client.subscribeCalls).toHaveLength(2);

    await closeWebSocket(ws);
    await app.close();
  });

  it('does not reconnect after terminal authz errors', async () => {
    const client = new FakeLiveClient();
    const app = buildApp(baseConfig(), client);
    const ws = await openWebSocket(app, '/ws');

    ws.send(JSON.stringify({ type: 'subscribe', deviceIds: ['dev-1'] }));
    await waitFor(() => client.subscribeCalls.length === 1);
    expect(client.subscribeCalls).toHaveLength(1);

    client.emitClose('dev-1', Object.assign(new Error('forbidden'), { code: 7 }));
    expect(await nextMessage(ws)).toEqual({
      type: 'device_status',
      deviceId: 'dev-1',
      ts: expect.any(Number),
      online: false
    });

    await sleep(80);
    expect(client.subscribeCalls).toHaveLength(1);

    await closeWebSocket(ws);
    await app.close();
  });

  it('emits online status heartbeats without extra telemetry', async () => {
    const client = new FakeLiveClient();
    const app = buildApp(baseConfig(), client);
    const ws = await openWebSocket(app, '/ws');

    ws.send(JSON.stringify({ type: 'subscribe', deviceIds: ['dev-1'] }));
    await waitFor(() => client.subscriptions.has('dev-1'));
    client.emitSnapshot('dev-1');
    await nextMessage(ws);
    await nextMessage(ws);

    client.emitHeartbeat('dev-1');
    expect(await nextMessage(ws)).toEqual({
      type: 'device_status',
      deviceId: 'dev-1',
      ts: 3000,
      online: true
    });

    await closeWebSocket(ws);
    await app.close();
  });

  it('closes websocket sessions and releases subscriptions during app shutdown', async () => {
    const client = new FakeLiveClient();
    const app = buildApp(baseConfig(), client);
    const ws = await openWebSocket(app, '/ws');

    ws.send(JSON.stringify({ type: 'subscribe', deviceIds: ['dev-1'] }));
    await waitFor(() => client.subscriptions.has('dev-1'));

    const closed = new Promise<void>((resolve) => {
      ws.once('close', () => resolve());
    });

    await app.close();
    await closed;

    expect(client.subscriptions.has('dev-1')).toBe(false);
  });
});

async function openWebSocket(app: ReturnType<typeof buildApp>, route: string): Promise<WebSocket> {
  await app.listen({ host: '127.0.0.1', port: 0 });
  const address = app.server.address() as AddressInfo;
  const url = `ws://127.0.0.1:${address.port}${route}`;
  return await new Promise<WebSocket>((resolve, reject) => {
    const ws = new WebSocket(url);
    ws.once('open', () => {
      const collector: MessageCollector = { queue: [], waiters: [] };
      messageCollectors.set(ws, collector);
      ws.on('message', (data: RawData) => {
        let message: unknown;
        try {
          message = JSON.parse(data.toString());
        } catch {
          return;
        }
        const waiter = collector.waiters.shift();
        if (waiter) {
          waiter(message);
          return;
        }
        collector.queue.push(message);
      });
      resolve(ws);
    });
    ws.once('unexpected-response', (_request: IncomingMessage, response: IncomingMessage) => {
      reject(new Error(`unexpected response ${response.statusCode ?? 'unknown'}`));
    });
    ws.once('error', reject);
  });
}

async function nextMessage(ws: WebSocket): Promise<unknown> {
  const collector = messageCollectors.get(ws);
  if (!collector) {
    throw new Error('message collector not initialized');
  }
  if (collector.queue.length > 0) {
    return collector.queue.shift();
  }
  return await new Promise((resolve, reject) => {
    collector.waiters.push(resolve);
    ws.once('error', reject);
  });
}

async function nextMessageWithin(ws: WebSocket, timeoutMs: number): Promise<unknown> {
  return await Promise.race([
    nextMessage(ws),
    sleep(timeoutMs).then(() => {
      throw new Error('timed out waiting for next websocket message');
    })
  ]);
}

async function closeWebSocket(ws: WebSocket): Promise<void> {
  await new Promise<void>((resolve) => {
    ws.once('close', () => resolve());
    ws.close();
  });
}

async function waitFor(predicate: () => boolean, timeoutMs = 500): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (!predicate()) {
    if (Date.now() >= deadline) {
      throw new Error('timed out waiting for condition');
    }
    await sleep(10);
  }
}

async function sleep(ms: number): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, ms));
}

function sampleLogEntry(overrides: Partial<AdminLogEntry> = {}): AdminLogEntry {
  return {
    id: 'entry-1',
    ts: 1772197190000,
    receivedTs: 1772197190100,
    deviceId: 'dev-1',
    status: 'ok',
    source: 'mqtt',
    sourceKind: 'SOURCE_KIND_MQTT_QUOTA',
    typeCode: 'quota',
    summary: 'MQTT quota update for dev-1',
    labels: { provider: 'ecoflow' },
    detail: {
      deviceId: 'dev-1',
      provider: 'ecoflow',
      payload: { params: { soc: 54 } }
    },
    ...overrides
  };
}
