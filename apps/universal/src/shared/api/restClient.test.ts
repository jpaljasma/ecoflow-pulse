import { afterEach, describe, expect, it, vi } from 'vitest';

const {
  recoverSessionForUnauthorizedRequest,
  triggerSessionExpiredRedirect
} = vi.hoisted(() => ({
  recoverSessionForUnauthorizedRequest: vi.fn(),
  triggerSessionExpiredRedirect: vi.fn()
}));
const { reportClientRestMetric } = vi.hoisted(() => ({
  reportClientRestMetric: vi.fn()
}));

type MutableMockEnv = {
  activeConnectionProfile: { dataPlane: 'local' | 'cloud' };
  activeDataPlane: 'local' | 'cloud';
};

vi.mock('@/features/auth/sessionRecoveryCoordinator', () => ({
  recoverSessionForUnauthorizedRequest,
  triggerSessionExpiredRedirect
}));

vi.mock('@/shared/api/clientRestMetrics', async () => {
  const actual = await vi.importActual<typeof import('@/shared/api/clientRestMetrics')>(
    '@/shared/api/clientRestMetrics'
  );
  return {
    ...actual,
    reportClientRestMetric
  };
});

vi.mock('@/shared/config/env', () => ({
  env: {
    isWeb: false,
    apiUrl: 'http://192.168.50.62:18081',
    apiUrlExplicit: false,
    wsUrl: 'ws://192.168.50.62:8082/ws',
    wsUrlExplicit: false,
    nativeHostHints: [],
    connectionProfileId: 'local',
    activeDataPlane: 'local',
    activeConnectionProfile: {
      dataPlane: 'local'
    }
  }
}));

import { env } from '@/shared/config/env';
import { ApiError, requestJson } from '@/shared/api/restClient';

describe('requestJson', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    recoverSessionForUnauthorizedRequest.mockReset();
    triggerSessionExpiredRedirect.mockReset();
    reportClientRestMetric.mockReset();
    env.apiUrlExplicit = false;
    env.apiUrl = 'http://192.168.50.62:18081';
    env.connectionProfileId = 'local';
    (env as unknown as MutableMockEnv).activeDataPlane = 'local';
    (env as unknown as MutableMockEnv).activeConnectionProfile = { dataPlane: 'local' };
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
    expect(reportClientRestMetric).toHaveBeenCalledWith(
      expect.objectContaining({
        route: '/api/devices',
        outcome: 'success',
        statusClass: '2xx',
        errorKind: 'none'
      })
    );
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
    expect(reportClientRestMetric).toHaveBeenCalledWith(
      expect.objectContaining({
        route: '/api/devices',
        outcome: 'network_error',
        statusClass: 'none',
        errorKind: 'network_failure'
      })
    );
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
    expect(reportClientRestMetric).toHaveBeenCalledTimes(1);
  });

  it('sends active connection profile and data-plane headers on REST requests', async () => {
    env.connectionProfileId = 'cloud';
    (env as unknown as MutableMockEnv).activeDataPlane = 'cloud';
    (env as unknown as MutableMockEnv).activeConnectionProfile = { dataPlane: 'cloud' };
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'content-type': 'application/json' }
      })
    );

    await requestJson('/api/v1/weather/forecast', {
      token: 'token-123'
    });

    expect(fetchSpy).toHaveBeenCalledOnce();
    expect((fetchSpy.mock.calls[0]?.[1] as RequestInit | undefined)?.headers).toMatchObject({
      Authorization: 'Bearer token-123',
      'X-Pulse-Connection-Profile': 'cloud',
      'X-Pulse-Data-Plane': 'cloud'
    });
  });

  it('retries once with a recovered session token after a 401 response', async () => {
    recoverSessionForUnauthorizedRequest.mockResolvedValueOnce('fresh-token');
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'invalid_bearer_token' }), {
          status: 401,
          headers: { 'content-type': 'application/json' }
        })
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ devices: [{ id: 'ok' }] }), {
          status: 200,
          headers: { 'content-type': 'application/json' }
        })
      );

    const payload = await requestJson<{ devices: Array<{ id: string }> }>('/api/devices', {
      token: 'stale-token'
    });

    expect(payload).toEqual({ devices: [{ id: 'ok' }] });
    expect(recoverSessionForUnauthorizedRequest).toHaveBeenCalledWith('stale-token');
    expect(triggerSessionExpiredRedirect).not.toHaveBeenCalled();
    expect(fetchSpy).toHaveBeenCalledTimes(2);
    expect((fetchSpy.mock.calls[1]?.[1] as RequestInit | undefined)?.headers).toMatchObject({
      Authorization: 'Bearer fresh-token'
    });
    expect(reportClientRestMetric).toHaveBeenCalledWith(
      expect.objectContaining({
        outcome: 'success'
      })
    );
  });

  it('requests reauthentication when recovery cannot repair a 401 response', async () => {
    recoverSessionForUnauthorizedRequest.mockResolvedValueOnce(null);
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ error: 'invalid_bearer_token' }), {
        status: 401,
        headers: { 'content-type': 'application/json' }
      })
    );

    await expect(
      requestJson('/api/devices', {
        token: 'stale-token'
      })
    ).rejects.toMatchObject({
      status: 401
    });

    expect(triggerSessionExpiredRedirect).toHaveBeenCalledTimes(1);
    expect(reportClientRestMetric).toHaveBeenCalledWith(
      expect.objectContaining({
        outcome: 'http_error',
        statusClass: '4xx',
        errorKind: 'status_401'
      })
    );
  });
});
