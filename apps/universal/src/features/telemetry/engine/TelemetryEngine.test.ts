import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/shared/config/env', () => ({
  env: {
    isWeb: false,
    apiUrl: 'http://192.168.50.62:18081',
    apiUrlExplicit: false,
    wsUrl: 'ws://192.168.50.62:8082/ws',
    wsUrlExplicit: false,
    nativeHostHints: []
  }
}));

import { TelemetryEngine } from '@/features/telemetry/engine/TelemetryEngine';
import * as clientWsMetrics from '@/features/telemetry/engine/clientWsMetrics';
import { env } from '@/shared/config/env';

type FakeSocketType = {
  url: string;
  readyState: number;
  onopen: ((event: Event) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent) => void) | null;
  sent: string[];
  closed: boolean;
  send: (data: string) => void;
  close: () => void;
  triggerOpen: () => void;
};

function createFakeSocket(url: string): FakeSocketType {
  return {
    url,
    readyState: 0,
    onopen: null,
    onclose: null,
    onerror: null,
    onmessage: null,
    sent: [],
    closed: false,
    send(data: string) {
      this.sent.push(data);
    },
    close() {
      this.closed = true;
      this.readyState = 3;
    },
    triggerOpen() {
      this.readyState = 1;
      this.onopen?.(new Event('open'));
    }
  };
}

describe('TelemetryEngine', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('opens the websocket transport by default', () => {
    const createSocket = vi.fn((url: string) => createFakeSocket(url));

    const engine = new TelemetryEngine({
      createSocket
    });

    engine.connect();

    expect(createSocket).toHaveBeenCalledTimes(1);
    expect(engine.getStatus()).toBe('connecting');
  });

  it('stays auth_required without opening a socket when auth is required and no token is present', () => {
    const reportSpy = vi.spyOn(clientWsMetrics, 'reportClientWsMetric').mockResolvedValue();
    const createSocket = vi.fn((url: string) => createFakeSocket(url));
    const engine = new TelemetryEngine({ createSocket });

    engine.connect(undefined, { authRequired: true });

    expect(createSocket).not.toHaveBeenCalled();
    expect(engine.getStatus()).toBe('auth_required');
    expect(reportSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        eventType: 'connection',
        outcome: 'auth_required'
      })
    );
  });

  it('reopens the socket when the access token changes', () => {
    const sockets: FakeSocketType[] = [];
    const createSocket = vi.fn((url: string) => {
      const socket = createFakeSocket(url);
      sockets.push(socket);
      return socket;
    });
    const engine = new TelemetryEngine({ createSocket });

    engine.connect('token-a', { authRequired: true });
    expect(createSocket).toHaveBeenCalledTimes(1);
    expect(sockets[0]?.url).toContain('token=token-a');

    sockets[0]?.triggerOpen();
    expect(engine.getStatus()).toBe('connected');

    engine.connect('token-b', { authRequired: true });

    expect(sockets[0]?.closed).toBe(true);
    expect(createSocket).toHaveBeenCalledTimes(2);
    expect(sockets[1]?.url).toContain('token=token-b');
    expect(engine.getStatus()).toBe('connecting');
  });

  it('redacts websocket auth tokens from connection logs', () => {
    const infoSpy = vi.spyOn(console, 'info').mockImplementation(() => {});
    const createSocket = vi.fn((url: string) => createFakeSocket(url));
    const engine = new TelemetryEngine({ createSocket });

    engine.connect('mock-access-token', { authRequired: true });

    expect(createSocket).toHaveBeenCalledWith(expect.stringContaining('token=mock-access-token'));
    expect(infoSpy).toHaveBeenCalledWith(
      '[TelemetryEngine] opening websocket',
      expect.objectContaining({
        url: expect.stringContaining('token=redacted')
      })
    );
    expect(JSON.stringify(infoSpy.mock.calls)).not.toContain('mock-access-token');

    engine.disconnect();
  });

  it('falls back to localhost websocket endpoint after reconnect', () => {
    vi.useFakeTimers();
    const randomSpy = vi.spyOn(Math, 'random').mockReturnValue(0);
    const sockets: FakeSocketType[] = [];
    const createSocket = vi.fn((url: string) => {
      const socket = createFakeSocket(url);
      sockets.push(socket);
      return socket;
    });
    const engine = new TelemetryEngine({ createSocket });

    engine.connect();

    expect(createSocket).toHaveBeenCalledTimes(1);
    expect(sockets[0]?.url).toContain('192.168.50.62');

    sockets[0]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(500);

    expect(createSocket).toHaveBeenCalledTimes(2);
    expect(sockets[1]?.url).toContain('127.0.0.1');

    engine.disconnect();
    randomSpy.mockRestore();
    vi.useRealTimers();
  });

  it('retries the current websocket origin only on web', () => {
    vi.useFakeTimers();
    const randomSpy = vi.spyOn(Math, 'random').mockReturnValue(0);
    const originalIsWeb = env.isWeb;
    env.isWeb = true;

    const sockets: FakeSocketType[] = [];
    const createSocket = vi.fn((url: string) => {
      const socket = createFakeSocket(url);
      sockets.push(socket);
      return socket;
    });
    const engine = new TelemetryEngine({ createSocket });

    engine.connect();
    expect(sockets[0]?.url).toBe('ws://192.168.50.62:8082/ws');

    sockets[0]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(500);

    expect(createSocket).toHaveBeenCalledTimes(2);
    expect(sockets[1]?.url).toBe('ws://192.168.50.62:8082/ws');

    engine.disconnect();
    env.isWeb = originalIsWeb;
    randomSpy.mockRestore();
    vi.useRealTimers();
  });

  it('tries api-proxy websocket first and then standalone gateway fallback', () => {
    vi.useFakeTimers();
    const randomSpy = vi.spyOn(Math, 'random').mockReturnValue(0);
    const originalWsUrl = env.wsUrl;
    const originalWsUrlExplicit = env.wsUrlExplicit;
    const originalApiUrl = env.apiUrl;

    env.wsUrl = 'ws://192.168.50.62:18081/ws';
    env.wsUrlExplicit = false;
    env.apiUrl = 'http://192.168.50.62:18081';

    const sockets: FakeSocketType[] = [];
    const createSocket = vi.fn((url: string) => {
      const socket = createFakeSocket(url);
      sockets.push(socket);
      return socket;
    });

    const engine = new TelemetryEngine({ createSocket });
    engine.connect();

    expect(sockets[0]?.url).toContain('192.168.50.62:18081/ws');

    sockets[0]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(500);
    expect(sockets[1]?.url).toContain('127.0.0.1:18081/ws');

    sockets[1]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(1_000);
    expect(sockets[2]?.url).toContain('localhost:18081/ws');

    sockets[2]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(2_000);
    expect(sockets[3]?.url).toContain('192.168.50.62/ws');

    sockets[3]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(4_000);
    expect(sockets[4]?.url).toContain('127.0.0.1/ws');

    sockets[4]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(8_000);
    expect(sockets[5]?.url).toContain('localhost/ws');

    sockets[5]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(16_000);
    expect(sockets[6]?.url).toContain('192.168.50.62:8082/ws');

    engine.disconnect();
    env.wsUrl = originalWsUrl;
    env.wsUrlExplicit = originalWsUrlExplicit;
    env.apiUrl = originalApiUrl;
    randomSpy.mockRestore();
    vi.useRealTimers();
  });

  it('reconnects with jitter and resubscribes after an unexpected close', () => {
    vi.useFakeTimers();
    const reportSpy = vi.spyOn(clientWsMetrics, 'reportClientWsMetric').mockResolvedValue();
    const randomSpy = vi.spyOn(Math, 'random').mockReturnValue(0.5);
    const sockets: FakeSocketType[] = [];
    const createSocket = vi.fn((url: string) => {
      const socket = createFakeSocket(url);
      sockets.push(socket);
      return socket;
    });
    const engine = new TelemetryEngine({
      createSocket,
      reconnectBaseMs: 1_000,
      reconnectMaxMs: 30_000
    });

    engine.connect();
    engine.subscribe(['device-1', 'device-2']);
    sockets[0]?.triggerOpen();

    expect(sockets[0]?.sent).toContain(
      JSON.stringify({ type: 'subscribe', deviceIds: ['device-1', 'device-2'] })
    );
    expect(engine.getStatus()).toBe('connected');

    sockets[0]?.onclose?.({ code: 1006 } as CloseEvent);
    expect(engine.getStatus()).toBe('reconnecting');

    vi.advanceTimersByTime(749);
    expect(createSocket).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(1);
    expect(createSocket).toHaveBeenCalledTimes(2);

    sockets[1]?.triggerOpen();
    expect(engine.getStatus()).toBe('connected');
    expect(sockets[1]?.sent).toContain(
      JSON.stringify({ type: 'subscribe', deviceIds: ['device-1', 'device-2'] })
    );
    expect(reportSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        eventType: 'connection',
        phase: 'initial',
        outcome: 'connected'
      })
    );
    expect(reportSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        eventType: 'disconnect',
        reason: 'unexpected_close'
      })
    );
    expect(reportSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        eventType: 'connection',
        phase: 'reconnect',
        outcome: 'connected'
      })
    );

    engine.disconnect();
    randomSpy.mockRestore();
    vi.useRealTimers();
  });

  it('keeps the last snapshot and marks it stale while reconnecting', () => {
    vi.useFakeTimers();
    const reportSpy = vi.spyOn(clientWsMetrics, 'reportClientWsMetric').mockResolvedValue();
    const randomSpy = vi.spyOn(Math, 'random').mockReturnValue(0);
    const sockets: FakeSocketType[] = [];
    const createSocket = vi.fn((url: string) => {
      const socket = createFakeSocket(url);
      sockets.push(socket);
      return socket;
    });
    const engine = new TelemetryEngine({
      createSocket,
      snapshotIntervalMs: 20,
      staleAfterMs: 50,
      reconnectBaseMs: 1_000
    });
    let latestPayload:
      | {
          status: string;
          snapshots: Record<string, { stale: boolean; metrics: { pvW: number } | null }>;
        }
      | undefined;

    engine.onSnapshot((payload) => {
      latestPayload = payload as typeof latestPayload;
    });

    engine.connect();
    engine.subscribe(['device-1']);
    sockets[0]?.triggerOpen();
    sockets[0]?.onmessage?.({
      data: JSON.stringify({
        type: 'telemetry',
        deviceId: 'device-1',
        ts: 1,
        metrics: { soc: 50, pvW: 200, loadW: 90, batteryW: 25, tempC: 21, acW: 30, dcW: 10 }
      })
    } as MessageEvent);

    vi.advanceTimersByTime(20);
    expect(latestPayload?.snapshots['device-1']?.metrics?.pvW).toBe(200);
    expect(latestPayload?.snapshots['device-1']?.stale).toBe(false);

    sockets[0]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(60);

    expect(latestPayload?.status).toBe('reconnecting');
    expect(latestPayload?.snapshots['device-1']?.metrics?.pvW).toBe(200);
    expect(latestPayload?.snapshots['device-1']?.stale).toBe(true);
    expect(reportSpy).toHaveBeenCalledWith({
      eventType: 'freshness_transition',
      state: 'stale'
    });

    engine.disconnect();
    randomSpy.mockRestore();
    vi.useRealTimers();
  });

  it('keeps sparse streams active until the configured inactivity window elapses', () => {
    vi.useFakeTimers();
    const sockets: FakeSocketType[] = [];
    const createSocket = vi.fn((url: string) => {
      const socket = createFakeSocket(url);
      sockets.push(socket);
      return socket;
    });
    const engine = new TelemetryEngine({
      createSocket,
      snapshotIntervalMs: 20,
      staleAfterMs: 50,
      inactiveAfterMs: 500
    });
    let latestPayload:
      | {
          snapshots: Record<string, { stale: boolean; inactive: boolean; metrics: { pvW: number } | null }>;
        }
      | undefined;

    engine.onSnapshot((payload) => {
      latestPayload = payload as typeof latestPayload;
    });

    engine.connect();
    engine.subscribe(['device-1']);
    sockets[0]?.triggerOpen();
    sockets[0]?.onmessage?.({
      data: JSON.stringify({
        type: 'telemetry',
        deviceId: 'device-1',
        ts: 1,
        metrics: { soc: 50, pvW: 200, loadW: 90, batteryW: 25, tempC: 21, acW: 30, dcW: 10 }
      })
    } as MessageEvent);

    vi.advanceTimersByTime(100);
    expect(latestPayload?.snapshots['device-1']?.stale).toBe(true);
    expect(latestPayload?.snapshots['device-1']?.inactive).toBe(false);

    vi.advanceTimersByTime(450);
    expect(latestPayload?.snapshots['device-1']?.inactive).toBe(true);

    engine.disconnect();
    vi.useRealTimers();
  });

  it('reports stale recovery after fresh data returns', () => {
    vi.useFakeTimers();
    const reportSpy = vi.spyOn(clientWsMetrics, 'reportClientWsMetric').mockResolvedValue();
    const sockets: FakeSocketType[] = [];
    const createSocket = vi.fn((url: string) => {
      const socket = createFakeSocket(url);
      sockets.push(socket);
      return socket;
    });
    const engine = new TelemetryEngine({
      createSocket,
      snapshotIntervalMs: 20,
      staleAfterMs: 50
    });

    engine.onSnapshot(() => undefined);
    engine.connect();
    engine.subscribe(['device-1']);
    sockets[0]?.triggerOpen();
    sockets[0]?.onmessage?.({
      data: JSON.stringify({
        type: 'telemetry',
        deviceId: 'device-1',
        ts: 1,
        metrics: { soc: 50, pvW: 200, loadW: 90, batteryW: 25, tempC: 21, acW: 30, dcW: 10 }
      })
    } as MessageEvent);

    vi.advanceTimersByTime(20);
    vi.advanceTimersByTime(60);
    sockets[0]?.onmessage?.({
      data: JSON.stringify({
        type: 'telemetry',
        deviceId: 'device-1',
        ts: 2,
        metrics: { soc: 51, pvW: 220, loadW: 95, batteryW: 20, tempC: 22, acW: 35, dcW: 10 }
      })
    } as MessageEvent);
    vi.advanceTimersByTime(20);

    expect(reportSpy).toHaveBeenCalledWith({
      eventType: 'freshness_transition',
      state: 'fresh'
    });
    expect(reportSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        eventType: 'stale_recovery',
        durationMs: expect.any(Number)
      })
    );

    engine.disconnect();
    vi.useRealTimers();
  });

  it('does not let older websocket telemetry replace the latest device metrics', () => {
    vi.useFakeTimers();
    const sockets: FakeSocketType[] = [];
    const createSocket = vi.fn((url: string) => {
      const socket = createFakeSocket(url);
      sockets.push(socket);
      return socket;
    });
    const engine = new TelemetryEngine({
      createSocket,
      snapshotIntervalMs: 20
    });
    let latestPayload:
      | {
          snapshots: Record<string, { metrics: { ts: number; soc: number; pvW: number } | null }>;
        }
      | undefined;

    engine.onSnapshot((payload) => {
      latestPayload = payload as typeof latestPayload;
    });
    engine.connect();
    engine.subscribe(['device-1']);
    sockets[0]?.triggerOpen();

    sockets[0]?.onmessage?.({
      data: JSON.stringify({
        type: 'telemetry',
        deviceId: 'device-1',
        ts: 2,
        metrics: { soc: 71, pvW: 500, loadW: 90, batteryW: 25, tempC: 21, acW: 30, dcW: 10 }
      })
    } as MessageEvent);
    sockets[0]?.onmessage?.({
      data: JSON.stringify({
        type: 'telemetry',
        deviceId: 'device-1',
        ts: 1,
        metrics: { soc: 98.8, pvW: 200, loadW: 80, batteryW: 20, tempC: 20, acW: 20, dcW: 5 }
      })
    } as MessageEvent);

    vi.advanceTimersByTime(20);

    expect(latestPayload?.snapshots['device-1']?.metrics).toMatchObject({
      ts: 2,
      soc: 71,
      pvW: 500
    });

    engine.disconnect();
    vi.useRealTimers();
  });

  it('tracks how many fleet trend buckets contain real live data', () => {
    vi.useFakeTimers();
    const sockets: FakeSocketType[] = [];
    const createSocket = vi.fn((url: string) => {
      const socket = createFakeSocket(url);
      sockets.push(socket);
      return socket;
    });
    const engine = new TelemetryEngine({
      createSocket,
      snapshotIntervalMs: 20,
      fleetTrendBucketMs: 100,
      fleetTrendPoints: 5
    });
    let latestPayload:
      | {
          fleetTrend: {
            load: number[];
            pv: number[];
            ac: number[];
            dc: number[];
            filledPoints: number;
          };
        }
      | undefined;

    engine.onSnapshot((payload) => {
      latestPayload = payload;
    });
    engine.connect();
    engine.subscribe(['device-1']);
    sockets[0]?.triggerOpen();

    sockets[0]?.onmessage?.({
      data: JSON.stringify({
        type: 'telemetry',
        deviceId: 'device-1',
        ts: 1,
        metrics: { soc: 50, pvW: 200, loadW: 90, batteryW: 25, tempC: 21, acW: 30, dcW: 10 }
      })
    } as MessageEvent);

    vi.advanceTimersByTime(150);

    expect(latestPayload?.fleetTrend.filledPoints).toBeGreaterThan(0);
    expect(latestPayload?.fleetTrend.pv.at(-1)).toBeGreaterThan(0);

    engine.disconnect();
    vi.useRealTimers();
  });

  it('retains live detail signals and solar ports from websocket telemetry', () => {
    vi.useFakeTimers();
    const sockets: FakeSocketType[] = [];
    const createSocket = vi.fn((url: string) => {
      const socket = createFakeSocket(url);
      sockets.push(socket);
      return socket;
    });
    const engine = new TelemetryEngine({
      createSocket,
      snapshotIntervalMs: 20
    });
    let latestPayload:
      | {
          snapshots: Record<
            string,
            {
              liveDetail?: {
                signals?: { batteryHeatingOn?: boolean; solarChargingOn?: boolean };
                solarPorts?: Array<{ id: string; state?: string; watts?: number }>;
              };
            }
          >;
        }
      | undefined;

    engine.onSnapshot((payload) => {
      latestPayload = payload;
    });
    engine.connect();
    engine.subscribe(['device-1']);
    sockets[0]?.triggerOpen();

    sockets[0]?.onmessage?.({
      data: JSON.stringify({
        type: 'telemetry',
        deviceId: 'device-1',
        ts: 1,
        metrics: { soc: 50, pvW: 120, loadW: 90, batteryW: 25, tempC: 21, acW: 0, dcW: 10 },
        detail: {
          signals: { batteryHeatingOn: false, solarChargingOn: true },
          solarPorts: [{ id: 'pv-low', name: 'PV Low', state: 'charging', watts: 120 }]
        }
      })
    } as MessageEvent);

    vi.advanceTimersByTime(40);

    expect(latestPayload?.snapshots['device-1']?.liveDetail).toEqual({
      signals: { batteryHeatingOn: false, solarChargingOn: true },
      solarPorts: [{ id: 'pv-low', name: 'PV Low', state: 'charging', watts: 120 }]
    });

    engine.disconnect();
    vi.useRealTimers();
  });
});
