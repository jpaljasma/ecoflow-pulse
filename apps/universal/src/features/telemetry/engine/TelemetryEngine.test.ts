import { describe, expect, it, vi } from 'vitest';

vi.mock('@/shared/config/env', () => ({
  env: {
    apiUrl: 'http://127.0.0.1:8081',
    wsUrl: 'ws://127.0.0.1:8080/ws',
    wsUrlExplicit: true
  }
}));

vi.mock('@/shared/api/mockLogDevices', () => ({
  getMockDevices: vi.fn(async () => [])
}));

import { TelemetryEngine } from '@/features/telemetry/engine/TelemetryEngine';

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
  it('stays auth_required without opening a socket when auth is required and no token is present', () => {
    const createSocket = vi.fn((url: string) => createFakeSocket(url));
    const engine = new TelemetryEngine({
      wsEnabled: true,
      createSocket
    });

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
    const engine = new TelemetryEngine({
      wsEnabled: true,
      createSocket
    });

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
});
