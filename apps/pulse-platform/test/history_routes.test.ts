import type { ServiceError } from '@grpc/grpc-js';
import { status as grpcStatus } from '@grpc/grpc-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { buildApp } from '../src/app.js';
import type { AppConfig } from '../src/config.js';
import type { DeviceClient } from '../src/grpc/deviceClient.js';
import type { CompareRollupSeries, RollupSeries, TelemetryHistoryClient } from '../src/grpc/telemetryClient.js';

function baseConfig(): AppConfig {
  return {
    host: '127.0.0.1',
    port: 18081,
    grpcApiAddr: '127.0.0.1:9090',
    grpcDeadlineMs: 2500,
    historyRateLimit: {
      max: 120,
      timeWindowMs: 60000
    },
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

function makeMinuteSeries(
  deviceId = '019c9f0e-4521-775d-873e-e80039f16d75',
  from = Date.UTC(2026, 2, 6, 5, 0, 0)
): RollupSeries {
  return {
    deviceId,
    resolution: 'minute',
    fromUnixMs: String(from),
    toUnixMs: String(from + 3 * 60 * 60 * 1000),
    points: [
      {
        bucketStartUnixMs: String(from + 6 * 60 * 60 * 1000),
        bucketEndUnixMs: String(from + 6 * 60 * 60 * 1000 + 60_000),
        sampleCount: 4,
        firstTsUnixMs: String(from + 6 * 60 * 60 * 1000),
        lastTsUnixMs: String(from + 6 * 60 * 60 * 1000 + 59_000),
        metrics: {
          socAvgPct: 50,
          socMinPct: 49,
          socMaxPct: 51,
          acInAvgW: 0,
          acInMaxW: 0,
          pvAvgW: 120,
          pvMaxW: 125,
          dcAvgW: 0,
          dcMaxW: 0,
          loadAvgW: 0,
          loadMaxW: 0,
          netAvgW: 120,
          netMinW: 118,
          netMaxW: 122,
          batteryAvgW: 100,
          batteryMinW: 95,
          batteryMaxW: 105,
          tempAvgC: 24,
          tempMinC: 23,
          tempMaxC: 25,
          solarGeneratedWh: 2
        }
      },
      {
        bucketStartUnixMs: String(from + 6 * 60 * 60 * 1000 + 10 * 60_000),
        bucketEndUnixMs: String(from + 6 * 60 * 60 * 1000 + 11 * 60_000),
        sampleCount: 4,
        firstTsUnixMs: String(from + 6 * 60 * 60 * 1000 + 10 * 60_000),
        lastTsUnixMs: String(from + 6 * 60 * 60 * 1000 + 10 * 60_000 + 59_000),
        metrics: {
          socAvgPct: 52,
          socMinPct: 51,
          socMaxPct: 53,
          acInAvgW: 0,
          acInMaxW: 0,
          pvAvgW: 180,
          pvMaxW: 185,
          dcAvgW: 0,
          dcMaxW: 0,
          loadAvgW: 0,
          loadMaxW: 0,
          netAvgW: 180,
          netMinW: 178,
          netMaxW: 182,
          batteryAvgW: 140,
          batteryMinW: 135,
          batteryMaxW: 145,
          tempAvgC: 25,
          tempMinC: 24,
          tempMaxC: 26,
          solarGeneratedWh: 3
        }
      }
    ]
  };
}

function makeClient(overrides: Partial<TelemetryHistoryClient> = {}): TelemetryHistoryClient {
  return {
    getSnapshot: vi.fn(async () => ({
      snapshot: {
        deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
        cursor: { seq: '1', tsUnixMs: String(Date.now()) },
        metrics: {}
      }
    })),
    queryRollupRange: vi.fn(async () => makeSeries()),
    compareRollupRange: vi.fn(async () => ({
      current: makeSeries(),
      previous: { ...makeSeries(), points: [] }
    } satisfies CompareRollupSeries)),
    close: vi.fn(),
    ...overrides
  };
}

function makeDeviceClient(): DeviceClient {
  return {
    listDevices: vi.fn(async () => []),
    getDevice: vi.fn(async () => null),
    close: vi.fn()
  };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('pulse-platform history routes', () => {
  it('returns range history via grpc client', async () => {
    const client = makeClient();
    const app = buildApp(baseConfig(), client, makeDeviceClient());
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
    const app = buildApp(baseConfig(), client, makeDeviceClient());
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

  it('returns backend-computed device solar history', async () => {
    const current = makeMinuteSeries();
    const previousFirstPoint = current.points[0]!;
    const client = makeClient({
      compareRollupRange: vi.fn(async () => ({
        current,
        previous: {
          ...current,
          points: [{ ...previousFirstPoint, metrics: { ...previousFirstPoint.metrics, solarGeneratedWh: 1 } }]
        }
      } satisfies CompareRollupSeries))
    });
    const app = buildApp(baseConfig(), client, makeDeviceClient());
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/019c9f0e-4521-775d-873e-e80039f16d75/history/solar?from=2026-03-06T00:00:00-05:00&to=2026-03-06T15:00:00-05:00'
    });

    expect(response.statusCode).toBe(200);
    expect(client.compareRollupRange).toHaveBeenCalledWith(
      expect.objectContaining({
        resolution: 'minute',
        usePreviousPeriod: true
      })
    );
    expect(response.json()).toEqual(
      expect.objectContaining({
        todayWh: 5,
        yesterdayWh: 1,
        deltaPct: 400,
        seriesWh: expect.arrayContaining([2, 3]),
        yesterdaySeriesWh: expect.arrayContaining([1])
      })
    );

    await app.close();
  });

  it('returns backend-computed fleet solar history', async () => {
    const deviceA = '019c9f0e-4521-775d-873e-e80039f16d75';
    const deviceB = '019c9f0e-452b-70eb-b3af-1c0f15c34416';
    const seriesA = makeMinuteSeries(deviceA);
    const seriesBBase = makeMinuteSeries(deviceB);
    const seriesBFirstPoint = seriesBBase.points[0]!;
    const seriesB: RollupSeries = {
      ...seriesBBase,
      points: [{ ...seriesBFirstPoint, metrics: { ...seriesBFirstPoint.metrics, solarGeneratedWh: 4 } }]
    };
    const client = makeClient({
      compareRollupRange: vi.fn(async (input) => ({
        current: input.deviceId === deviceA ? seriesA : seriesB,
        previous: { ...makeMinuteSeries(input.deviceId), points: [] }
      } satisfies CompareRollupSeries))
    });
    const app = buildApp(baseConfig(), client, makeDeviceClient());
    const response = await app.inject({
      method: 'GET',
      url: `/api/v1/history/solar/fleet?deviceId=${deviceA}&deviceId=${deviceB}&from=2026-03-06T00:00:00-05:00&to=2026-03-06T15:00:00-05:00`
    });

    expect(response.statusCode).toBe(200);
    expect(client.compareRollupRange).toHaveBeenCalledTimes(2);
    const body = response.json();
    expect(body).toEqual(
      expect.objectContaining({
        todayWh: 9,
        yesterdayWh: 0,
        deltaPct: null
      })
    );
    expect(body.seriesWh[0]).toBe(6);
    expect(body.seriesWh[1]).toBe(3);
    expect(body.yesterdaySeriesWh).toEqual(expect.arrayContaining([0]));

    await app.close();
  });

  it('returns 400 for invalid query', async () => {
    const app = buildApp(baseConfig(), makeClient(), makeDeviceClient());
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
    const app = buildApp(baseConfig(), client, makeDeviceClient());
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
    const app = buildApp(authConfig, makeClient(), makeDeviceClient(), {
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
    const app = buildApp(authConfig, makeClient(), makeDeviceClient(), {
      authPreHandler: async (request: { url: string }, reply: { code: (statusCode: number) => { send: (body: unknown) => void } }) => {
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

  it('rate limits history routes', async () => {
    const app = buildApp(
      {
        ...baseConfig(),
        historyRateLimit: {
          max: 1,
          timeWindowMs: 60000
        }
      },
      makeClient(),
      makeDeviceClient()
    );

    const first = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/019c9f0e-4521-775d-873e-e80039f16d75/history?resolution=hour&from=1&to=2',
      headers: {
        'x-forwarded-for': '203.0.113.10'
      }
    });
    expect(first.statusCode).toBe(200);

    const second = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/019c9f0e-4521-775d-873e-e80039f16d75/history?resolution=hour&from=1&to=2',
      headers: {
        'x-forwarded-for': '203.0.113.10'
      }
    });
    expect(second.statusCode).toBe(429);

    await app.close();
  });
});
