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

import { env } from '@/shared/config/env';
import { ApiError, requestJson } from '@/shared/api/restClient';

describe('requestJson', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    env.apiUrlExplicit = false;
    env.apiUrl = 'http://192.168.50.62:18081';
  });

  it('retries with localhost fallback on network failure for native defaults', async () => {
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockRejectedValueOnce(new TypeError('Network request failed'))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ devices: [] }), {
          status: 200,
          headers: { 'content-type': 'application/json' }
        })
      );

    const payload = await requestJson<{ devices: unknown[] }>('/api/devices');

    expect(payload).toEqual({ devices: [] });
    expect(fetchSpy).toHaveBeenCalledTimes(2);
    expect(fetchSpy.mock.calls[0]?.[0]).toBe('http://192.168.50.62:18081/api/devices');
    expect(fetchSpy.mock.calls[1]?.[0]).toBe('http://127.0.0.1:18081/api/devices');
  });

  it('does not retry fallback when API URL is explicit', async () => {
    env.apiUrlExplicit = true;
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockRejectedValueOnce(new TypeError('Network request failed'));

    let thrown: unknown;
    try {
      await requestJson('/api/devices');
    } catch (error) {
      thrown = error;
    }

    expect(thrown).toBeInstanceOf(ApiError);
    expect((thrown as Error).message).toContain('Network request failed for GET /api/devices');
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(fetchSpy.mock.calls[0]?.[0]).toBe('http://192.168.50.62:18081/api/devices');
  });

  it('falls back to public-edge host when standalone port is unavailable', async () => {
    env.apiUrl = 'http://127.0.0.1:18081';
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockRejectedValueOnce(new TypeError('Network request failed'))
      .mockRejectedValueOnce(new TypeError('Network request failed'))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ devices: [{ id: 'ok' }] }), {
          status: 200,
          headers: { 'content-type': 'application/json' }
        })
      );

    const payload = await requestJson<{ devices: Array<{ id: string }> }>('/api/devices');

    expect(payload).toEqual({ devices: [{ id: 'ok' }] });
    expect(fetchSpy).toHaveBeenCalledTimes(3);
    expect(fetchSpy.mock.calls[0]?.[0]).toBe('http://127.0.0.1:18081/api/devices');
    expect(fetchSpy.mock.calls[1]?.[0]).toBe('http://localhost:18081/api/devices');
    expect(fetchSpy.mock.calls[2]?.[0]).toBe('http://127.0.0.1/api/devices');
  });
});
