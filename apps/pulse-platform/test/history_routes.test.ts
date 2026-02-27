import type { ServiceError } from '@grpc/grpc-js';
import { status as grpcStatus } from '@grpc/grpc-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { buildApp } from '../src/app.js';
import type { AppConfig } from '../src/config.js';
import type { CompareRollupSeries, RollupSeries, TelemetryHistoryClient } from '../src/grpc/telemetryClient.js';

function baseConfig(): AppConfig {
  return {
    host: '127.0.0.1',
    port: 8081,
    grpcApiAddr: '127.0.0.1:9090',
    grpcDeadlineMs: 2500,
    auth: { mode: 'noop', allowMissingJwt: true }
  };
}

function makeSeries(deviceId = '019c9f0e-4521-775d-873e-e80039f16d75'): RollupSeries {
  return {
    deviceId,
    resolution: 'hour',
    fromUnixMs: '1772193600000',
    toUnixMs: '1772197200000',
    points: [
      {
        bucketStartUnixMs: '1772193600000',
        bucketEndUnixMs: '1772197200000',
        sampleCount: 3,
        firstTsUnixMs: '1772193610000',
        lastTsUnixMs: '1772197190000',
        metrics: {
          socAvgPct: 50,
          socMinPct: 45,
          socMaxPct: 55,
          acInAvgW: 10,
          acInMaxW: 12,
          pvAvgW: 20,
          pvMaxW: 24,
          dcAvgW: 0,
          dcMaxW: 0,
          loadAvgW: 30,
          loadMaxW: 36,
          netAvgW: -20,
          netMinW: -25,
          netMaxW: -18,
          batteryAvgW: 40,
          batteryMinW: 38,
          batteryMaxW: 44,
          tempAvgC: 25,
          tempMinC: 22,
          tempMaxC: 28,
          solarGeneratedWh: 12
        }
      }
    ]
  };
}

function makeClient(overrides: Partial<TelemetryHistoryClient> = {}): TelemetryHistoryClient {
  return {
      queryRollupRange: vi.fn(async () => makeSeries()),
      compareRollupRange: vi.fn(async () => ({
        current: makeSeries(),
        previous: { ...makeSeries(), points: [] }
      } satisfies CompareRollupSeries)),
    close: vi.fn(),
    ...overrides
  };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('pulse-platform history routes', () => {
  it('returns range history via grpc client', async () => {
    const client = makeClient();
    const app = buildApp(baseConfig(), client);
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/019c9f0e-4521-775d-873e-e80039f16d75/history?resolution=hour&from=1772193600000&to=1772197200000',
      headers: {
        authorization: 'Bearer test-token'
      }
    });

    expect(response.statusCode).toBe(200);
    expect(client.queryRollupRange).toHaveBeenCalledWith(
      expect.objectContaining({
        deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
        resolution: 'hour',
        fromUnixMs: '1772193600000',
        toUnixMs: '1772197200000',
        authHeader: 'Bearer test-token',
        deadlineMs: 2500
      })
    );

    await app.close();
  });

  it('parses ISO timestamps and compare requests', async () => {
    const client = makeClient();
    const app = buildApp(baseConfig(), client);
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/019c9f0e-4521-775d-873e-e80039f16d75/history/compare?resolution=day&from=2026-02-27T00:00:00Z&to=2026-02-28T00:00:00Z'
    });

    expect(response.statusCode).toBe(200);
    expect(client.compareRollupRange).toHaveBeenCalledWith(
      expect.objectContaining({
        resolution: 'day',
        fromUnixMs: String(Date.parse('2026-02-27T00:00:00Z')),
        toUnixMs: String(Date.parse('2026-02-28T00:00:00Z')),
        usePreviousPeriod: true
      })
    );

    await app.close();
  });

  it('returns 400 for invalid query', async () => {
    const app = buildApp(baseConfig(), makeClient());
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/not-a-uuid/history?resolution=hour&from=1&to=2'
    });

    expect(response.statusCode).toBe(400);
    await app.close();
  });

  it('maps grpc permission denied', async () => {
    const client = makeClient({
      queryRollupRange: vi.fn(async () => {
        const error = new Error('not allowed') as ServiceError;
        error.code = grpcStatus.PERMISSION_DENIED;
        error.details = 'not allowed';
        throw error;
      })
    });
    const app = buildApp(baseConfig(), client);
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/019c9f0e-4521-775d-873e-e80039f16d75/history?resolution=hour&from=1&to=2'
    });

    expect(response.statusCode).toBe(403);
    expect(response.json()).toEqual(
      expect.objectContaining({ error: 'upstream_grpc_error', grpcCode: grpcStatus.PERMISSION_DENIED })
    );
    await app.close();
  });

  it('enforces bearer token when auth is enabled', async () => {
    const verifier = vi.fn(async () => ({ subject: 'sub-1', roles: [], rawJwt: 'jwt' }));
    const authConfig: AppConfig = {
      ...baseConfig(),
      auth: {
        mode: 'keycloak',
        issuerUrl: 'https://issuer.example',
        audience: 'pulse-platform',
        allowMissingJwt: false
      }
    };
    const { makeVerifierPreHandler } = await import('../src/auth.js');
    const app = buildApp(authConfig, makeClient(), {
      authPreHandler: makeVerifierPreHandler(verifier, false)
    });

    const missing = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/019c9f0e-4521-775d-873e-e80039f16d75/history?resolution=hour&from=1&to=2'
    });
    expect(missing.statusCode).toBe(401);

    const ok = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/019c9f0e-4521-775d-873e-e80039f16d75/history?resolution=hour&from=1&to=2',
      headers: { authorization: 'Bearer jwt' }
    });
    expect(ok.statusCode).toBe(200);
    expect(verifier).toHaveBeenCalledWith('jwt');

    await app.close();
  });

  it('keeps health unauthenticated', async () => {
    const verifier = vi.fn();
    const authConfig: AppConfig = {
      ...baseConfig(),
      auth: {
        mode: 'keycloak',
        issuerUrl: 'https://issuer.example',
        audience: 'pulse-platform',
        allowMissingJwt: false
      }
    };
    const app = buildApp(authConfig, makeClient(), {
      authPreHandler: async (request, reply) => {
        if (request.url !== '/healthz') {
          void reply.code(401).send({ error: 'missing_bearer_token' });
        }
      }
    });

    const response = await app.inject({ method: 'GET', url: '/healthz' });
    expect(response.statusCode).toBe(200);
    expect(verifier).not.toHaveBeenCalled();
    await app.close();
  });
});
