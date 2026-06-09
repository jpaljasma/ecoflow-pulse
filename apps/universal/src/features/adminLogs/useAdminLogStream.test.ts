import { describe, expect, it, vi } from 'vitest';

vi.mock('@/shared/config/env', () => ({
  env: { wsUrl: 'wss://pulse.home.arpa/ws' }
}));

import { armAdminLogStartupTimeout } from './useAdminLogStream';

describe('admin log websocket connect timeout', () => {
  it('closes a websocket that remains in CONNECTING past the timeout', () => {
    vi.useFakeTimers();
    const close = vi.fn();
    const ws = {
      readyState: WebSocket.CONNECTING,
      close
    } as unknown as WebSocket;

    armAdminLogStartupTimeout(ws, { timeoutMs: 250 });
    vi.advanceTimersByTime(249);
    expect(close).not.toHaveBeenCalled();

    vi.advanceTimersByTime(1);
    expect(close).toHaveBeenCalledTimes(1);
    vi.useRealTimers();
  });

  it('closes an open websocket when no startup frame arrives before the timeout', () => {
    vi.useFakeTimers();
    const close = vi.fn();
    const ws = {
      readyState: WebSocket.OPEN,
      close
    } as unknown as WebSocket;

    armAdminLogStartupTimeout(ws, { timeoutMs: 250 });
    vi.advanceTimersByTime(250);

    expect(close).toHaveBeenCalledTimes(1);
    vi.useRealTimers();
  });
});
