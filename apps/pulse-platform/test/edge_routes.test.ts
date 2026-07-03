import { afterEach, describe, expect, it, vi } from 'vitest';

import { buildApp } from '../src/app.js';
import type { AppConfig } from '../src/config.js';
import type { DeviceClient } from '../src/grpc/deviceClient.js';
import type { EdgeClient, EdgeCollector, EdgeDeviceSource } from '../src/grpc/edgeClient.js';
import type { InferenceClient } from '../src/grpc/inferenceClient.js';
import type { TelemetryHistoryClient } from '../src/grpc/telemetryClient.js';

function baseConfig(): AppConfig {
  return {
    host: '127.0.0.1',
    port: 18081,
    grpcApiAddr: '127.0.0.1:9090',
    energyGrpcApiAddr: '127.0.0.1:9091',
    grpcDeadlineMs: 2500,
    devUserSubject: 'dev-user',
    publicPreconnectOrigins: [],
    corsAllowedOrigins: [],
    historyRateLimit: { max: 120, timeWindowMs: 60000 },
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

function makeDeviceClient(): DeviceClient {
  return {
    listDevices: vi.fn(async () => []),
    getDevice: vi.fn(async () => null),
    listAvailableDevices: vi.fn(async () => ({ devices: [], hasActiveCredentials: false })),
    testAvailableDeviceMQTT: vi.fn(),
    enableAvailableDevice: vi.fn(),
    importAvailableDevice: vi.fn(),
    close: vi.fn()
  } as unknown as DeviceClient;
}

function makeInferenceClient(): InferenceClient {
  return {
    getDeviceInsights: vi.fn(),
    getEnergyComparisonInsight: vi.fn(),
    close: vi.fn()
  } as unknown as InferenceClient;
}

function sampleCollector(overrides: Partial<EdgeCollector> = {}): EdgeCollector {
  return {
    id: 'edgecol-1',
    displayName: 'Pi 5',
    isActive: true,
    lastHeartbeatAtUnixMs: '1772197190000',
    createdAtUnixMs: '1772190000000',
    updatedAtUnixMs: '1772197190000',
    collectorVersion: 'test',
    hostname: 'pi',
    ...overrides
  };
}

function sampleSource(overrides: Partial<EdgeDeviceSource> = {}): EdgeDeviceSource {
  return {
    id: 'edgesrc-1',
    collectorId: 'edgecol-1',
    provider: 'ecoflow',
    transport: 'ble',
    providerDeviceId: 'DEMOEDGE0001',
    displayName: 'Demo edge device',
    model: 'EcoFlow RIVER 3 Plus',
    status: 'pending',
    linkedDeviceId: '',
    rssiDbm: -59,
    lastSeenAtUnixMs: '1772197190000',
    createdAtUnixMs: '1772190000000',
    updatedAtUnixMs: '1772197190000',
    metadata: {},
    ...overrides
  };
}

function makeEdgeClient(overrides: Partial<EdgeClient> = {}): EdgeClient {
  return {
    createCollector: vi.fn(async () => ({ collector: sampleCollector({ isActive: false }), setupToken: 'setup-token' })),
    listCollectors: vi.fn(async () => [sampleCollector()]),
    enrollCollector: vi.fn(async () => ({
      collector: sampleCollector(),
      collectorSecret: 'collector-secret',
      collectorEnv: { ECOFLOW_BLE_USER_ID: 'ecoflow-user-1' }
    })),
    heartbeat: vi.fn(async () => sampleCollector()),
    uploadDiscovery: vi.fn(async () => ({ acceptedCount: 1 })),
    listDeviceSources: vi.fn(async () => [sampleSource()]),
    approveDeviceSource: vi.fn(async () => ({
      source: sampleSource({ status: 'linked', linkedDeviceId: 'device-1' }),
      deviceId: 'device-1'
    })),
    uploadTelemetry: vi.fn(async () => ({ acceptedCount: 1, droppedCount: 0 })),
    close: vi.fn(),
    ...overrides
  };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('pulse-platform edge routes', () => {
  it('creates collector setup tokens through authenticated routes', async () => {
    const edgeClient = makeEdgeClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      edgeClient
    });
    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/edge/collectors',
      payload: { displayName: 'Garage Pi' }
    });

    expect(response.statusCode).toBe(201);
    expect(response.json()).toMatchObject({
      collector: { id: 'edgecol-1' },
      setupToken: 'setup-token'
    });
    expect(edgeClient.createCollector).toHaveBeenCalledWith(expect.objectContaining({
      userSubject: 'dev-user',
      displayName: 'Garage Pi'
    }));
    await app.close();
  });

  it('returns collector env only from the enrollment route', async () => {
    const edgeClient = makeEdgeClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      edgeClient
    });
    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/edge/enroll',
      payload: {
        setupToken: 'setup-token',
        collectorVersion: 'test',
        hostname: 'pi'
      }
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toMatchObject({
      collector: { id: 'edgecol-1' },
      collectorSecret: 'collector-secret',
      collectorEnv: { ECOFLOW_BLE_USER_ID: 'ecoflow-user-1' }
    });
    expect(edgeClient.enrollCollector).toHaveBeenCalledWith(expect.objectContaining({
      setupToken: 'setup-token',
      collectorVersion: 'test',
      hostname: 'pi'
    }));
    await app.close();
  });

  it('accepts collector telemetry batches without user auth', async () => {
    const edgeClient = makeEdgeClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      edgeClient
    });
    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/edge/telemetry',
      payload: {
        collectorSecret: 'collector-secret',
        samples: [{
          provider: 'ecoflow',
          transport: 'ble',
          providerDeviceId: 'DEMOEDGE0001',
          clientSampleId: 'edge-sample-1',
          metrics: { output_power_w: 118 }
        }]
      }
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({ acceptedCount: 1, droppedCount: 0 });
    expect(edgeClient.uploadTelemetry).toHaveBeenCalledWith(expect.objectContaining({
      collectorSecret: 'collector-secret',
      samples: [expect.objectContaining({
        providerDeviceId: 'DEMOEDGE0001',
        clientSampleId: 'edge-sample-1'
      })]
    }));
    await app.close();
  });
});
