import { afterEach, describe, expect, it, vi } from 'vitest';

import { buildApp } from '../src/app.js';
import type { AppConfig } from '../src/config.js';
import type { DeviceClient, DeviceSummary } from '../src/grpc/deviceClient.js';
import type { TelemetryHistoryClient } from '../src/grpc/telemetryClient.js';

function baseConfig(): AppConfig {
  return {
    host: '127.0.0.1',
    port: 18081,
    grpcApiAddr: '127.0.0.1:9090',
    grpcDeadlineMs: 2500,
    devUserSubject: 'jpaljasma@gmail.com',
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
    serialNumber: 'Y711ZABA9H2P0294',
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
    const app = buildApp(baseConfig(), makeHistoryClient(), client);

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

  it('returns device detail by serial number', async () => {
    const client = makeDeviceClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), client);

    const response = await app.inject({
      method: 'GET',
      url: '/api/devices/Y711ZABA9H2P0294'
    });

    expect(response.statusCode).toBe(200);
    expect(client.getDevice).toHaveBeenCalledWith(expect.anything(), 'Y711ZABA9H2P0294');
    expect(response.json()).toEqual(sampleDevice());

    await app.close();
  });

  it('returns 404 when device is missing', async () => {
    const client = makeDeviceClient({
      getDevice: vi.fn(async () => null)
    });
    const app = buildApp(baseConfig(), makeHistoryClient(), client);

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
      client
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
});
