import { describe, expect, it, vi } from 'vitest';

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
    const createSocket = vi.fn((url: string) => createFakeSocket(url));
    const engine = new TelemetryEngine({ createSocket });

    engine.connect(undefined, { authRequired: true });

    expect(createSocket).not.toHaveBeenCalled();
    expect(engine.getStatus()).toBe('auth_required');
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
    vi.advanceTimersByTime(1);

    expect(createSocket).toHaveBeenCalledTimes(2);
    expect(sockets[1]?.url).toContain('127.0.0.1');

    engine.disconnect();
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
    vi.advanceTimersByTime(1);
    expect(sockets[1]?.url).toContain('127.0.0.1:18081/ws');

    sockets[1]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(1);
    expect(sockets[2]?.url).toContain('localhost:18081/ws');

    sockets[2]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(1);
    expect(sockets[3]?.url).toContain('192.168.50.62/ws');

    sockets[3]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(1);
    expect(sockets[4]?.url).toContain('127.0.0.1/ws');

    sockets[4]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(1);
    expect(sockets[5]?.url).toContain('localhost/ws');

    sockets[5]?.onclose?.({ code: 1006 } as CloseEvent);
    vi.advanceTimersByTime(1);
    expect(sockets[6]?.url).toContain('192.168.50.62:8082/ws');

    engine.disconnect();
    env.wsUrl = originalWsUrl;
    env.wsUrlExplicit = originalWsUrlExplicit;
    env.apiUrl = originalApiUrl;
    randomSpy.mockRestore();
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
});
