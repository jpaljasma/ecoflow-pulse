import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ServiceError } from '@grpc/grpc-js';
import { status as grpcStatus } from '@grpc/grpc-js';

import { buildApp } from '../src/app.js';
import type { AppConfig } from '../src/config.js';
import type {
  AvailableDevicesResult,
  AvailableDeviceSummary,
  DeviceClient,
  DeviceSummary
} from '../src/grpc/deviceClient.js';
import type { DeviceInsights, InferenceClient } from '../src/grpc/inferenceClient.js';
import type { TelemetryHistoryClient } from '../src/grpc/telemetryClient.js';

function baseConfig(): AppConfig {
  return {
    host: '127.0.0.1',
    port: 18081,
    grpcApiAddr: '127.0.0.1:9090',
    energyGrpcApiAddr: '127.0.0.1:9091',
    grpcDeadlineMs: 2500,
    devUserSubject: 'dev-user@example.com',
    publicPreconnectOrigins: [],
    corsAllowedOrigins: [],
    historyRateLimit: {
      max: 120,
      timeWindowMs: 60000
    },
    auth: { mode: 'noop', allowMissingJwt: true }
  };
}

function makeHistoryClient(): TelemetryHistoryClient {
  return {
    getSnapshot: vi.fn(),
    queryRollupRange: vi.fn(),
    compareRollupRange: vi.fn(),
    close: vi.fn()
  } as unknown as TelemetryHistoryClient;
}

function sampleDevice(overrides: Partial<DeviceSummary> = {}): DeviceSummary {
  return {
    id: '019c9f0e-4521-775d-873e-e80039f16d75',
    serialNumber: 'DEMODPU0000294',
    name: 'DPU A 12 kWh',
    model: 'DELTA Pro Ultra',
    online: true,
    batteryPct: 54.8,
    state: 'discharging',
    etaMinutes: 315,
    pvW: 22.63,
    acInW: 6.76,
    dcW: 0,
    loadW: 0,
    netW: 22.63,
    tempC: 20,
    telemetryTsMs: 1772197190000,
    capabilities: {
      batteryPacks: 2,
      pvInputCount: 2,
      batteryCapacityKWh: 12
    },
    details: {
      bpCount: 2,
      socWindowMinPct: 12,
      socWindowMaxPct: 95,
      backupReservePct: 18,
      stormGuardActive: true,
      stormGuardEndsAtUnixMs: 1773306000 * 1000,
      solarPorts: [
        {
          id: 'pv-low',
          name: 'PV Low',
          state: 'charging',
          volts: 64.2,
          amps: 1.2,
          watts: 77.04,
          maxWatts: 1600,
          maxVolts: 150,
          maxAmps: 15
        }
      ],
      packs: [
        {
          id: 'bp1',
          socPct: 48.5,
          powerW: 12.1,
          tempC: 19.5,
          heatingOn: true,
          energyWh: 5980,
          remainMinutes: 322,
          socMinPct: 10,
          socMaxPct: 95
        }
      ],
      batteryHeatingOn: true
    },
    ...overrides
  };
}

function sampleAvailableDevice(
  overrides: Partial<AvailableDeviceSummary> = {}
): AvailableDeviceSummary {
  return {
    provider: 'ecoflow',
    providerDeviceId: 'DEMOD2M00001057',
    credentialId: 'cred-1',
    serialNumber: 'DEMOD2M00001057',
    name: 'Kitchen Delta 2 Max',
    model: 'DELTA 2 Max',
    ...overrides
  };
}

function sampleAvailableDevicesResult(
  overrides: Partial<AvailableDevicesResult> = {}
): AvailableDevicesResult {
  return {
    devices: [sampleAvailableDevice()],
    hasActiveCredentials: true,
    ...overrides
  };
}

function makeDeviceClient(overrides: Partial<DeviceClient> = {}): DeviceClient {
  return {
    listDevices: vi.fn(async () => [sampleDevice()]),
    getDevice: vi.fn(async (_request, routeDeviceId) => {
    const device = sampleDevice();
      return routeDeviceId === device.id || routeDeviceId === device.serialNumber ? device : null;
    }),
    listAvailableDevices: vi.fn(async () => sampleAvailableDevicesResult()),
    testAvailableDeviceMQTT: vi.fn(async () => ({
      success: true,
      status: 'ok',
      sampleTopic: '/open/open-account/DEMOD2M00001057/quota',
      payloadBytes: '512',
      observedAtUnixMs: '1772197190000'
    })),
    enableAvailableDevice: vi.fn(async () => ({
      deviceId: '019c9f0e-4521-775d-873e-e80039f16d75'
    })),
    close: vi.fn(),
    ...overrides
  };
}

function sampleInsights(overrides: Partial<DeviceInsights> = {}): DeviceInsights {
  return {
    deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
    status: 'ready',
    statusDetail: 'derived from live inference projection',
    refreshedAtUnixMs: '1772197190000',
    insights: [
      {
        id: 'ins-1',
        deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
        kind: 'battery_expansion',
        title: 'Add extra battery capacity',
        summary: 'Your DELTA Pro Ultra can add more battery packs.',
        score: 0.9,
        rank: 1,
        modelKey: 'battery-expansion-rule',
        modelVersion: 'v1',
        generatedAtUnixMs: '1772197190000',
        expiresAtUnixMs: '1772218790000',
        tags: ['battery', 'upsell'],
        evidence: [],
        actions: [
          {
            kind: 'external_url',
            label: 'Get More Batteries (3)',
            target:
              'https://us.ecoflow.com/products/delta-pro-ultra-battery?variant=41446274465865&inviteCode=ATH7F3EF1P'
          }
        ],
        attributes: {
          current_battery_packs: 2,
          max_battery_packs: 5,
          recommended_additional_packs: 3
        }
      }
    ],
    ...overrides
  };
}

function makeInferenceClient(overrides: Partial<InferenceClient> = {}): InferenceClient {
  return {
    getDeviceInsights: vi.fn(async () => sampleInsights()),
    getEnergyComparisonInsight: vi.fn(async () => ({
      status: 'ready' as const,
      statusDetail: 'ok'
    })),
    close: vi.fn(),
    ...overrides
  };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('pulse-platform device routes', () => {
  it('returns user devices', async () => {
    const client = makeDeviceClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), client, makeInferenceClient());

    const response = await app.inject({
      method: 'GET',
      url: '/api/devices'
    });

    expect(response.statusCode).toBe(200);
    expect(client.listDevices).toHaveBeenCalledOnce();
    expect(response.json()).toEqual({
      devices: [sampleDevice()]
    });

    await app.close();
  });

  it('allows browser cors requests from configured localhost dev origins', async () => {
    const client = makeDeviceClient();
    const app = buildApp(
      {
        ...baseConfig(),
        corsAllowedOrigins: ['http://localhost:8081', 'https://localhost:8081']
      },
      makeHistoryClient(),
      client,
      makeInferenceClient()
    );

    const preflight = await app.inject({
      method: 'OPTIONS',
      url: '/api/devices',
      headers: {
        origin: 'http://localhost:8081',
        'access-control-request-method': 'GET'
      }
    });

    expect(preflight.statusCode).toBe(204);
    expect(preflight.headers['access-control-allow-origin']).toBe('http://localhost:8081');
    expect(preflight.headers['access-control-allow-credentials']).toBe('true');

    const response = await app.inject({
      method: 'GET',
      url: '/api/devices',
      headers: {
        origin: 'http://localhost:8081'
      }
    });

    expect(response.statusCode).toBe(200);
    expect(response.headers['access-control-allow-origin']).toBe('http://localhost:8081');
    expect(response.headers['access-control-allow-credentials']).toBe('true');

    await app.close();
  });

  it('returns device detail by serial number', async () => {
    const client = makeDeviceClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), client, makeInferenceClient());

    const response = await app.inject({
      method: 'GET',
      url: '/api/devices/DEMODPU0000294'
    });

    expect(response.statusCode).toBe(200);
    expect(client.getDevice).toHaveBeenCalledWith(expect.anything(), 'DEMODPU0000294');
    expect(response.json()).toEqual(sampleDevice());

    await app.close();
  });

  it('returns available devices', async () => {
    const client = makeDeviceClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), client, makeInferenceClient());

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/available'
    });

    expect(response.statusCode).toBe(200);
    expect(client.listAvailableDevices).toHaveBeenCalledOnce();
    expect(response.json()).toEqual(sampleAvailableDevicesResult());

    await app.close();
  });

  it('degrades invalid available-device credentials into a warning response', async () => {
    const details = 'list available provider devices: list ecoflow devices: ecoflow api business error code=8513 message=accessKey is invalid';
    const client = makeDeviceClient({
      listAvailableDevices: vi.fn(async () => {
        throw {
          code: grpcStatus.INTERNAL,
          details,
          message: details,
          name: 'Error'
        } satisfies Partial<ServiceError>;
      })
    });
    const app = buildApp(baseConfig(), makeHistoryClient(), client, makeInferenceClient());

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/available'
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({
      devices: [],
      hasActiveCredentials: true,
      warningCode: 'credential_invalid',
      warningMessage:
        'An active provider credential is being rejected by the provider. Update it in Settings > Integrations before scanning for new devices.'
    });

    await app.close();
  });

  it('tests device mqtt from the available-devices route', async () => {
    const client = makeDeviceClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), client, makeInferenceClient());

    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/devices/available/test-mqtt',
      payload: {
        provider: 'ecoflow',
        credentialId: 'cred-1',
        providerDeviceId: 'DEMOD2M00001057'
      }
    });

    expect(response.statusCode).toBe(200);
    expect(client.testAvailableDeviceMQTT).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        provider: 'ecoflow',
        credentialId: 'cred-1',
        providerDeviceId: 'DEMOD2M00001057'
      })
    );

    await app.close();
  });

  it('enables an available device', async () => {
    const client = makeDeviceClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), client, makeInferenceClient());

    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/devices/available/enable',
      payload: {
        provider: 'ecoflow',
        credentialId: 'cred-1',
        providerDeviceId: 'DEMOD2M00001057'
      }
    });

    expect(response.statusCode).toBe(200);
    expect(client.enableAvailableDevice).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        provider: 'ecoflow',
        credentialId: 'cred-1',
        providerDeviceId: 'DEMOD2M00001057'
      })
    );
    expect(response.json()).toEqual({ deviceId: '019c9f0e-4521-775d-873e-e80039f16d75' });

    await app.close();
  });

  it('returns device detail by uuid', async () => {
    const client = makeDeviceClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), client, makeInferenceClient());

    const response = await app.inject({
      method: 'GET',
      url: '/api/devices/019c9f0e-4521-775d-873e-e80039f16d75'
    });

    expect(response.statusCode).toBe(200);
    expect(client.getDevice).toHaveBeenCalledWith(expect.anything(), '019c9f0e-4521-775d-873e-e80039f16d75');
    expect(response.json()).toEqual(sampleDevice());

    await app.close();
  });

  it('returns capabilities and detail payloads', async () => {
    const client = makeDeviceClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), client, makeInferenceClient());

    const response = await app.inject({
      method: 'GET',
      url: '/api/devices'
    });

    expect(response.statusCode).toBe(200);
    const body = response.json() as { devices: DeviceSummary[] };
    expect(body.devices[0]?.capabilities).toEqual(
      expect.objectContaining({
        batteryPacks: 2,
        pvInputCount: 2,
        batteryCapacityKWh: 12
      })
    );
    expect(body.devices[0]?.details).toEqual(
      expect.objectContaining({
        bpCount: 2,
        batteryHeatingOn: true,
        socWindowMinPct: 12,
        socWindowMaxPct: 95,
        backupReservePct: 18,
        stormGuardActive: true,
        stormGuardEndsAtUnixMs: 1773306000 * 1000,
        packs: [
          expect.objectContaining({
            id: 'bp1',
            energyWh: 5980,
            remainMinutes: 322,
            socMinPct: 10,
            socMaxPct: 95
          })
        ],
        solarPorts: [
          expect.objectContaining({
            id: 'pv-low',
            state: 'charging',
            maxWatts: 1600
          })
        ]
      })
    );

    await app.close();
  });

  it('returns 404 when device is missing', async () => {
    const client = makeDeviceClient({
      getDevice: vi.fn(async () => null)
    });
    const app = buildApp(baseConfig(), makeHistoryClient(), client, makeInferenceClient());

    const response = await app.inject({
      method: 'GET',
      url: '/api/devices/missing-device'
    });

    expect(response.statusCode).toBe(404);
    expect(response.json()).toEqual({ error: 'device_not_found' });

    await app.close();
  });

  it('returns 503 when noop mode has no user subject configured', async () => {
    const client = makeDeviceClient({
      listDevices: vi.fn(async () => {
        throw new Error('missing_user_subject');
      })
    });
    const app = buildApp(
      {
        ...baseConfig(),
        devUserSubject: undefined
      },
      makeHistoryClient(),
      client,
      makeInferenceClient()
    );

    const response = await app.inject({
      method: 'GET',
      url: '/api/devices'
    });

    expect(response.statusCode).toBe(503);
    expect(response.json()).toEqual(
      expect.objectContaining({
        error: 'missing_user_subject'
      })
    );

    await app.close();
  });

  it('maps authenticated device authorization failures to 403', async () => {
    const client = makeDeviceClient({
      listDevices: vi.fn(async () => {
        const error = new Error('device access denied') as ServiceError;
        error.code = grpcStatus.PERMISSION_DENIED;
        error.details = 'device access denied';
        throw error;
      })
    });
    const app = buildApp(baseConfig(), makeHistoryClient(), client, makeInferenceClient());

    const response = await app.inject({
      method: 'GET',
      url: '/api/devices'
    });

    expect(response.statusCode).toBe(403);
    expect(response.json()).toEqual(
      expect.objectContaining({
        error: 'upstream_grpc_error',
        grpcCode: grpcStatus.PERMISSION_DENIED
      })
    );

    await app.close();
  });

  it('returns device insights', async () => {
    const inferenceClient = makeInferenceClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), inferenceClient);

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/devices/019c9f0e-4521-775d-873e-e80039f16d75/insights?kind=battery_expansion&maxItems=1'
    });

    expect(response.statusCode).toBe(200);
    expect(inferenceClient.getDeviceInsights).toHaveBeenCalledWith(
      expect.objectContaining({
        deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
        kinds: ['battery_expansion'],
        maxItems: 1
      })
    );
    expect(response.json()).toEqual(sampleInsights());

    await app.close();
  });
});
