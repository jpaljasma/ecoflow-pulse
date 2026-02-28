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
} from '../src/grpc/liveTelemetryClient.js';

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

function baseConfig(): AppConfig {
  return {
    host: '127.0.0.1',
    port: 0,
    grpcApiAddr: '127.0.0.1:9090',
    grpcDeadlineMs: 2500,
    subscribeUpdateHz: 4,
    reconnectBackoff: {
      baseMs: 20,
      maxMs: 40
    },
    auth: { mode: 'noop', allowMissingJwt: true }
  };
}

afterEach(() => {
  vi.restoreAllMocks();
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
        batteryW: -87.52000000000001,
        tempC: 20,
        acW: 6.760000000000002,
        dcW: 0
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
