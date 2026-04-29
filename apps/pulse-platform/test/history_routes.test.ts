import type { ServiceError } from '@grpc/grpc-js';
import { status as grpcStatus } from '@grpc/grpc-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { buildApp } from '../src/app.js';
import type { AppConfig } from '../src/config.js';
import type { DeviceClient } from '../src/grpc/deviceClient.js';
import type { InferenceClient } from '../src/grpc/inferenceClient.js';
import type {
  BatterySummary,
  CompareRollupSeries,
  EnergyDashboard,
  EnergySummary,
  EnergyValueComparison,
  RollupPoint,
  RollupSeries,
  TelemetryHistoryClient
} from '../src/grpc/telemetryClient.js';

function baseConfig(): AppConfig {
  return {
    host: '127.0.0.1',
    port: 18081,
    grpcApiAddr: '127.0.0.1:9090',
    energyGrpcApiAddr: '127.0.0.1:9091',
    grpcDeadlineMs: 2500,
    devUserSubject: 'dev-user-subject',
    publicPreconnectOrigins: [],
    corsAllowedOrigins: [],
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
          acOutputAvgW: 30,
          acOutputMaxW: 36,
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
          solarGeneratedWh: 12,
          acInputEnergyWh: 10,
          acOutputEnergyWh: 30,
          dcOutputEnergyWh: 0,
          loadEnergyWh: 30,
          batteryChargeEnergyWh: 40,
          batteryDischargeEnergyWh: 0
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
          acOutputAvgW: 0,
          acOutputMaxW: 0,
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
          solarGeneratedWh: 2,
          acInputEnergyWh: 0,
          acOutputEnergyWh: 0,
          dcOutputEnergyWh: 0,
          loadEnergyWh: 0,
          batteryChargeEnergyWh: 2,
          batteryDischargeEnergyWh: 0
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
          acOutputAvgW: 0,
          acOutputMaxW: 0,
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
          solarGeneratedWh: 3,
          acInputEnergyWh: 0,
          acOutputEnergyWh: 0,
          dcOutputEnergyWh: 0,
          loadEnergyWh: 0,
          batteryChargeEnergyWh: 3,
          batteryDischargeEnergyWh: 0
        }
      }
    ]
  };
}

function makeEnergyValueComparison(current: number, previous: number, deltaPct: number | null): EnergyValueComparison {
  return {
    current,
    previous,
    delta: current - previous,
    deltaPct
  };
}

function makeBatterySummary(): BatterySummary {
  return {
    chargeKwh: 1.2,
    dischargeKwh: 0.8,
    netKwh: 0.4,
    socStartPct: 42,
    socEndPct: 57,
    socMinPct: 40,
    socMaxPct: 60
  };
}

function makeEnergySummary(): EnergySummary {
  return {
    solarGeneratedKwh: makeEnergyValueComparison(2.4, 1.6, 50),
    loadConsumedKwh: makeEnergyValueComparison(3.2, 3, 6.6667),
    selfSufficiencyPct: makeEnergyValueComparison(75, 53.33, 40.625),
    batteryNetKwh: makeEnergyValueComparison(0.4, -0.2, null),
    estimatedValue: makeEnergyValueComparison(0.72, 0.48, 50),
    estimatedAcInputCost: makeEnergyValueComparison(0.24, 0.3, -20),
    currency: 'USD'
  };
}

function makeEnergyDashboard(overrides: Partial<EnergyDashboard> = {}): EnergyDashboard {
  const currentEnergyPoint: RollupPoint = {
    bucketStartUnixMs: '1772715600000',
    bucketEndUnixMs: '1772719200000',
    sampleCount: 3,
    firstTsUnixMs: '1772715610000',
    lastTsUnixMs: '1772719190000',
    metrics: {
      socAvgPct: 51,
      socMinPct: 48,
      socMaxPct: 57,
      acInAvgW: 20,
      acInMaxW: 28,
      acOutputAvgW: 145,
      acOutputMaxW: 170,
      pvAvgW: 180,
      pvMaxW: 220,
      dcAvgW: 15,
      dcMaxW: 20,
      loadAvgW: 160,
      loadMaxW: 190,
      netAvgW: 20,
      netMinW: -10,
      netMaxW: 45,
      batteryAvgW: 55,
      batteryMinW: -30,
      batteryMaxW: 90,
      tempAvgC: 24,
      tempMinC: 22,
      tempMaxC: 26,
      solarGeneratedWh: 240,
      acInputEnergyWh: 20,
      acOutputEnergyWh: 145,
      dcOutputEnergyWh: 15,
      loadEnergyWh: 160,
      batteryChargeEnergyWh: 55,
      batteryDischargeEnergyWh: 0
    }
  };

  return {
    scope: {
      mode: 'device',
      deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
      resolvedDeviceIds: ['019c9f0e-4521-775d-873e-e80039f16d75']
    },
    window: {
      preset: 'today',
      timezone: 'America/New_York',
      fromUnixMs: '1772686800000',
      toUnixMs: '1772719200000',
      previousFromUnixMs: '1772600400000',
      previousToUnixMs: '1772632800000'
    },
    summary: makeEnergySummary(),
    battery: makeBatterySummary(),
    currentEnergyPoints: [currentEnergyPoint],
    previousEnergyPoints: [],
    currentPowerPoints: [currentEnergyPoint],
    previousPowerPoints: [],
    pvPortHistory: [],
    ...overrides
  };
}

function makeClient(overrides: Partial<TelemetryHistoryClient> = {}): TelemetryHistoryClient {
  return {
    queryRollupRange: vi.fn(async () => makeSeries()),
    compareRollupRange: vi.fn(async () => ({
      current: makeSeries(),
      previous: { ...makeSeries(), points: [] }
    } satisfies CompareRollupSeries)),
    getEnergyDashboard: vi.fn(async () => makeEnergyDashboard()),
    getEnergyPvPortHistory: vi.fn(async () => makeEnergyDashboard().pvPortHistory),
    close: vi.fn(),
    ...overrides
  };
}

function makeDeviceClient(): DeviceClient {
  return {
    listDevices: vi.fn(async () => []),
    getDevice: vi.fn(async () => null),
    listAvailableDevices: vi.fn(async () => ({ devices: [], hasActiveCredentials: false })),
    testAvailableDeviceMQTT: vi.fn(),
    enableAvailableDevice: vi.fn(async () => ({ deviceId: '' })),
    importAvailableDevice: vi.fn(async () => ({ deviceId: '' })),
    close: vi.fn()
  };
}

function makeInferenceClient(): InferenceClient {
  return {
    getDeviceInsights: vi.fn(),
    getEnergyComparisonInsight: vi.fn(async () => ({
      status: 'ready' as const,
      statusDetail: 'cached',
      insight: {
        id: 'insight-1',
        scope: {
          mode: 'all',
          deviceId: '',
          resolvedDeviceIds: ['019c9f0e-4521-775d-873e-e80039f16d75']
        },
        preset: 'last7d',
        timezone: 'America/New_York',
        verdictClass: 'solar_freedom_up',
        headline: 'More solar freedom',
        summary: 'Self-sufficiency improved.',
        score: 0.44,
        confidence: 0.81,
        modelKey: 'energy-comparison-score',
        modelVersion: 'v1',
        generatedAtUnixMs: '1772719200000',
        expiresAtUnixMs: '1772722800000',
        tags: ['energy', 'comparison'],
        cards: [],
        evidence: []
      }
    })),
    close: vi.fn()
  } as unknown as InferenceClient;
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('pulse-platform history routes', () => {
  it('returns range history via grpc client', async () => {
    const client = makeClient();
    const app = buildApp(baseConfig(), client, makeDeviceClient(), makeInferenceClient());
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
    const app = buildApp(baseConfig(), client, makeDeviceClient(), makeInferenceClient());
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
    const app = buildApp(baseConfig(), client, makeDeviceClient(), makeInferenceClient());
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/019c9f0e-4521-775d-873e-e80039f16d75/history/solar?from=2026-03-08T00:00:00-05:00&to=2026-03-08T07:00:00-04:00&compareFrom=2026-03-07T00:00:00-05:00&compareTo=2026-03-08T00:00:00-05:00'
    });

    expect(response.statusCode).toBe(200);
    expect(client.compareRollupRange).toHaveBeenCalledWith(
      expect.objectContaining({
        resolution: 'minute',
        fromUnixMs: String(Date.parse('2026-03-08T00:00:00-05:00')),
        toUnixMs: String(Date.parse('2026-03-08T07:00:00-04:00')),
        compareFromUnixMs: String(Date.parse('2026-03-07T00:00:00-05:00')),
        compareToUnixMs: String(Date.parse('2026-03-08T00:00:00-05:00')),
        usePreviousPeriod: false
      })
    );
    expect(response.json()).toEqual(
      expect.objectContaining({
        todayWh: 5,
        yesterdayWh: 2,
        yesterdayRunningWh: 0,
        deltaPct: null,
        seriesWh: expect.arrayContaining([2, 3]),
        yesterdaySeriesWh: expect.arrayContaining([2])
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
    const app = buildApp(baseConfig(), client, makeDeviceClient(), makeInferenceClient());
    const response = await app.inject({
      method: 'GET',
      url: `/api/v1/history/solar/fleet?deviceId=${deviceA}&deviceId=${deviceB}&from=2026-03-06T00:00:00-05:00&to=2026-03-06T15:00:00-05:00&compareFrom=2026-03-05T00:00:00-05:00&compareTo=2026-03-06T00:00:00-05:00`
    });

    expect(response.statusCode).toBe(200);
    expect(client.compareRollupRange).toHaveBeenCalledTimes(2);
    expect(client.compareRollupRange).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        compareFromUnixMs: String(Date.parse('2026-03-05T00:00:00-05:00')),
        compareToUnixMs: String(Date.parse('2026-03-06T00:00:00-05:00')),
        usePreviousPeriod: false
      })
    );
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
    const app = buildApp(baseConfig(), makeClient(), makeDeviceClient(), makeInferenceClient());
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/not-a-uuid/history?resolution=hour&from=1&to=2'
    });

    expect(response.statusCode).toBe(400);
    await app.close();
  });

  it('returns energy dashboard for a single device', async () => {
    const client = makeClient();
    const app = buildApp(baseConfig(), client, makeDeviceClient(), makeInferenceClient());
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/energy/dashboard?scope=device&deviceId=019c9f0e-4521-775d-873e-e80039f16d75&preset=today&timezone=America%2FNew_York&gridPricePerKwh=0.30&currency=USD',
      headers: {
        authorization: 'Bearer dashboard-token'
      }
    });

    expect(response.statusCode).toBe(200);
    expect(client.getEnergyDashboard).toHaveBeenCalledWith(
      expect.objectContaining({
        deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
        useAllDevices: false,
        preset: 'today',
        timezone: 'America/New_York',
        includeComparison: true,
        gridPricePerKwh: 0.3,
        currency: 'USD',
        authHeader: 'Bearer dashboard-token',
        userSubject: 'dev-user-subject',
        deadlineMs: 2500
      })
    );
    expect(response.json()).toEqual(
      expect.objectContaining({
        scope: expect.objectContaining({
          mode: 'device'
        }),
        summary: expect.objectContaining({
          currency: 'USD'
        })
      })
    );

    await app.close();
  });

  it('returns energy dashboard for all visible devices', async () => {
    const dashboard = makeEnergyDashboard({
      scope: {
        mode: 'all',
        deviceId: '',
        resolvedDeviceIds: [
          '019c9f0e-4521-775d-873e-e80039f16d75',
          '019c9f0e-452b-70eb-b3af-1c0f15c34416'
        ]
      }
    });
    const client = makeClient({
      getEnergyDashboard: vi.fn(async () => dashboard)
    });
    const app = buildApp(baseConfig(), client, makeDeviceClient(), makeInferenceClient());
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/energy/dashboard?scope=all&preset=last7d&timezone=America%2FLos_Angeles&includeComparison=false'
    });

    expect(response.statusCode).toBe(200);
    expect(client.getEnergyDashboard).toHaveBeenCalledWith(
      expect.objectContaining({
        deviceId: undefined,
        useAllDevices: true,
        preset: 'last7d',
        timezone: 'America/Los_Angeles',
        includeComparison: false,
        userSubject: 'dev-user-subject'
      })
    );
    expect(response.json()).toEqual(
      expect.objectContaining({
        scope: expect.objectContaining({
          mode: 'all',
          resolvedDeviceIds: expect.arrayContaining([
            '019c9f0e-4521-775d-873e-e80039f16d75',
            '019c9f0e-452b-70eb-b3af-1c0f15c34416'
          ])
        })
      })
    );

    await app.close();
  });

  it('returns energy pv history separately from the dashboard payload', async () => {
    const client = makeClient();
    const app = buildApp(baseConfig(), client, makeDeviceClient(), makeInferenceClient());
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/energy/pv-history?scope=all&preset=last7d&timezone=America%2FLos_Angeles'
    });

    expect(response.statusCode).toBe(200);
    expect(client.getEnergyPvPortHistory).toHaveBeenCalledWith(
      expect.objectContaining({
        deviceId: undefined,
        useAllDevices: true,
        preset: 'last7d',
        timezone: 'America/Los_Angeles',
        userSubject: 'dev-user-subject'
      })
    );
    expect(response.json()).toEqual(
      expect.objectContaining({
        pvPortHistory: expect.any(Array)
      })
    );

    await app.close();
  });

  it('returns cached energy comparison insight independently from the dashboard payload', async () => {
    const inferenceClient = makeInferenceClient();
    const app = buildApp(baseConfig(), makeClient(), makeDeviceClient(), inferenceClient);
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/energy/comparison-insight?scope=all&preset=last7d&timezone=America%2FLos_Angeles&gridPricePerKwh=0.30&currency=USD',
      headers: {
        authorization: 'Bearer comparison-token'
      }
    });

    expect(response.statusCode).toBe(200);
    expect(inferenceClient.getEnergyComparisonInsight).toHaveBeenCalledWith(
      expect.objectContaining({
        deviceId: undefined,
        useAllDevices: true,
        preset: 'last7d',
        timezone: 'America/Los_Angeles',
        gridPricePerKwh: 0.3,
        currency: 'USD',
        authHeader: 'Bearer comparison-token',
        deadlineMs: 2500
      })
    );
    expect(response.json()).toEqual(
      expect.objectContaining({
        status: 'ready',
        insight: expect.objectContaining({
          verdictClass: 'solar_freedom_up',
          headline: 'More solar freedom'
        })
      })
    );

    await app.close();
  });

  it('accepts the rolling past24h energy preset', async () => {
    const client = makeClient();
    const app = buildApp(baseConfig(), client, makeDeviceClient(), makeInferenceClient());
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/energy/dashboard?scope=all&preset=past24h&timezone=America%2FNew_York&includeComparison=true'
    });

    expect(response.statusCode).toBe(200);
    expect(client.getEnergyDashboard).toHaveBeenCalledWith(
      expect.objectContaining({
        deviceId: undefined,
        useAllDevices: true,
        preset: 'past24h',
        timezone: 'America/New_York',
        includeComparison: true
      })
    );

    await app.close();
  });

  it('accepts the completed last30d energy preset', async () => {
    const client = makeClient();
    const app = buildApp(baseConfig(), client, makeDeviceClient(), makeInferenceClient());
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/energy/dashboard?scope=all&preset=last30d&timezone=America%2FNew_York&includeComparison=true'
    });

    expect(response.statusCode).toBe(200);
    expect(client.getEnergyDashboard).toHaveBeenCalledWith(
      expect.objectContaining({
        deviceId: undefined,
        useAllDevices: true,
        preset: 'last30d',
        timezone: 'America/New_York',
        includeComparison: true
      })
    );

    await app.close();
  });

  it('accepts the calendar lastMonth energy preset', async () => {
    const client = makeClient();
    const app = buildApp(baseConfig(), client, makeDeviceClient(), makeInferenceClient());
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/energy/dashboard?scope=all&preset=lastMonth&timezone=America%2FNew_York&includeComparison=true'
    });

    expect(response.statusCode).toBe(200);
    expect(client.getEnergyDashboard).toHaveBeenCalledWith(
      expect.objectContaining({
        deviceId: undefined,
        useAllDevices: true,
        preset: 'lastMonth',
        timezone: 'America/New_York',
        includeComparison: true
      })
    );

    await app.close();
  });

  it('rejects energy dashboard device scope without a device id', async () => {
    const app = buildApp(baseConfig(), makeClient(), makeDeviceClient(), makeInferenceClient());
    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/energy/dashboard?scope=device&preset=today&timezone=America%2FNew_York'
    });

    expect(response.statusCode).toBe(400);
    expect(response.json()).toEqual(
      expect.objectContaining({
        error: 'invalid_request'
      })
    );

    await app.close();
  });

  it('maps authenticated grpc permission denied to 403', async () => {
    const client = makeClient({
      queryRollupRange: vi.fn(async () => {
        const error = new Error('not allowed') as ServiceError;
        error.code = grpcStatus.PERMISSION_DENIED;
        error.details = 'not allowed';
        throw error;
      })
    });
    const app = buildApp(baseConfig(), client, makeDeviceClient(), makeInferenceClient());
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
      energyGrpcApiAddr: '127.0.0.1:9091',
      auth: {
        mode: 'keycloak',
        issuerUrl: 'https://issuer.example',
        audience: 'pulse-platform',
        allowMissingJwt: false
      }
    };
    const { makeVerifierPreHandler } = await import('../src/auth.js');
    const app = buildApp(authConfig, makeClient(), makeDeviceClient(), makeInferenceClient(), {
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
      energyGrpcApiAddr: '127.0.0.1:9091',
      auth: {
        mode: 'keycloak',
        issuerUrl: 'https://issuer.example',
        audience: 'pulse-platform',
        allowMissingJwt: false
      }
    };
    const app = buildApp(authConfig, makeClient(), makeDeviceClient(), makeInferenceClient(), {
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
      makeDeviceClient(),
      makeInferenceClient()
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
