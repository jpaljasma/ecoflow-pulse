import { afterEach, describe, expect, it, vi } from 'vitest';
import type { preHandlerHookHandler } from 'fastify';

import { buildApp } from '../src/app.js';
import type { AppConfig } from '../src/config.js';
import type {
  ControlPlaneClient,
  EcoFlowBLEAuthStatus,
  ProviderCredential
} from '../src/grpc/controlPlaneClient.js';
import type { DeviceClient } from '../src/grpc/deviceClient.js';
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

function sampleBLEStatus(overrides: Partial<EcoFlowBLEAuthStatus> = {}): EcoFlowBLEAuthStatus {
  return {
    connected: true,
    status: 'connected',
    accountMask: 'owne...test',
    updatedAtUnixMs: '1772197190000',
    ...overrides
  };
}

function sampleIntegration(overrides: Partial<ProviderCredential> = {}): ProviderCredential {
  return {
    id: '11111111-1111-7111-8111-111111111111',
    provider: 'ecoflow',
    accessKeyMask: 'owne...test',
    config: {},
    isActive: true,
    createdAtUnixMs: '1772190000000',
    updatedAtUnixMs: '1772197190000',
    ...overrides
  };
}

function makeControlPlaneClient(overrides: Partial<ControlPlaneClient> = {}): ControlPlaneClient {
  return {
    getCurrentUser: vi.fn(),
    updateCurrentUser: vi.fn(),
    refreshCurrentUserIdentity: vi.fn(),
    listProviderCredentials: vi.fn(async () => []),
    createProviderCredential: vi.fn(),
    updateProviderCredential: vi.fn(),
    setProviderCredentialActive: vi.fn(),
    listUserDevices: vi.fn(async () => []),
    listDevices: vi.fn(async () => []),
    listAvailableProviderDevices: vi.fn(async () => ({ devices: [], hasActiveCredentials: false })),
    getEcoFlowBLEAuthStatus: vi.fn(async () => sampleBLEStatus({ connected: false, status: 'not_connected' })),
    connectEcoFlowBLEAuth: vi.fn(async () => sampleBLEStatus()),
    setEcoFlowBLEAuthUserID: vi.fn(async () => sampleBLEStatus({ accountMask: 'Manu...e ID' })),
    testProviderDeviceMQTT: vi.fn(),
    enableProviderDevice: vi.fn(),
    importProviderDevice: vi.fn(),
    searchAdminLogFilters: vi.fn(async () => []),
    close: vi.fn(),
    ...overrides
  } as unknown as ControlPlaneClient;
}

const devAuthPreHandler: preHandlerHookHandler = async (request) => {
  request.auth = { subject: 'dev-user' } as never;
};

afterEach(() => {
  vi.restoreAllMocks();
});

describe('pulse-platform integration routes', () => {
  it('hides dedicated EcoFlow BLE auth credentials from the generic integration list', async () => {
    const controlPlaneClient = makeControlPlaneClient({
      listProviderCredentials: vi.fn(async () => [
        sampleIntegration(),
        sampleIntegration({
          id: '22222222-2222-7222-8222-222222222222',
          provider: 'ecoflow_ble',
          accessKeyMask: 'ble...auth'
        })
      ])
    });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/integrations'
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toMatchObject({
      integrations: [
        {
          id: '11111111-1111-7111-8111-111111111111',
          provider: 'ecoflow'
        }
      ]
    });
    expect(JSON.stringify(response.json())).not.toContain('ecoflow_ble');
    await app.close();
  });

  it('returns EcoFlow BLE auth readiness without secret material', async () => {
    const controlPlaneClient = makeControlPlaneClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/integrations/ecoflow-ble-auth'
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({
      status: {
        connected: false,
        status: 'not_connected',
        accountMask: 'owne...test',
        updatedAtUnixMs: '1772197190000'
      }
    });
    expect(controlPlaneClient.getEcoFlowBLEAuthStatus).toHaveBeenCalledWith(expect.objectContaining({
      userSubject: 'dev-user'
    }));
    await app.close();
  });

  it('connects EcoFlow BLE auth with one-time credentials', async () => {
    const controlPlaneClient = makeControlPlaneClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/integrations/ecoflow-ble-auth/connect',
      payload: {
        email: 'owner@example.test',
        password: ' owner-password '
      }
    });

    expect(response.statusCode).toBe(200);
    expect(JSON.stringify(response.json())).not.toContain('owner-password');
    expect(controlPlaneClient.connectEcoFlowBLEAuth).toHaveBeenCalledWith(expect.objectContaining({
      userSubject: 'dev-user',
      email: 'owner@example.test',
      password: ' owner-password '
    }));
    await app.close();
  });

  it('rejects generic EcoFlow BLE credential creation', async () => {
    const createProviderCredential = vi.fn();
    const controlPlaneClient = makeControlPlaneClient({ createProviderCredential });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/integrations',
      payload: {
        provider: 'ecoflow_ble',
        accessKey: 'owner@example.test',
        accessSecret: 'ble-user-123'
      }
    });

    expect(response.statusCode).toBe(400);
    expect(createProviderCredential).not.toHaveBeenCalled();
    await app.close();
  });

  it('sets manual EcoFlow BLE user ID without echoing the raw ID', async () => {
    const controlPlaneClient = makeControlPlaneClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/integrations/ecoflow-ble-auth/manual',
      payload: {
        userId: 'manual-ble-user',
        accountLabel: 'Manual EcoFlow BLE ID'
      }
    });

    expect(response.statusCode).toBe(200);
    expect(JSON.stringify(response.json())).not.toContain('manual-ble-user');
    expect(controlPlaneClient.setEcoFlowBLEAuthUserID).toHaveBeenCalledWith(expect.objectContaining({
      userSubject: 'dev-user',
      userId: 'manual-ble-user',
      accountLabel: 'Manual EcoFlow BLE ID'
    }));
    await app.close();
  });

  it('rejects manual EcoFlow BLE user ID in non-local auth mode', async () => {
    const controlPlaneClient = makeControlPlaneClient();
    const app = buildApp(
      {
        ...baseConfig(),
        auth: {
          mode: 'keycloak',
          issuerUrl: 'https://keycloak.example.test/realms/pulse',
          audience: 'pulse',
          allowMissingJwt: true
        }
      },
      makeHistoryClient(),
      makeDeviceClient(),
      makeInferenceClient(),
      { controlPlaneClient, authPreHandler: devAuthPreHandler }
    );

    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/integrations/ecoflow-ble-auth/manual',
      payload: {
        userId: 'manual-ble-user'
      }
    });

    expect(response.statusCode).toBe(403);
    expect(controlPlaneClient.setEcoFlowBLEAuthUserID).not.toHaveBeenCalled();
    await app.close();
  });

  it('allows manual EcoFlow BLE user ID in non-local auth mode with explicit override', async () => {
    const controlPlaneClient = makeControlPlaneClient();
    const app = buildApp(
      {
        ...baseConfig(),
        ecoFlowBLEManualAuthEnabled: true,
        auth: {
          mode: 'keycloak',
          issuerUrl: 'https://keycloak.example.test/realms/pulse',
          audience: 'pulse',
          allowMissingJwt: true
        }
      },
      makeHistoryClient(),
      makeDeviceClient(),
      makeInferenceClient(),
      { controlPlaneClient, authPreHandler: devAuthPreHandler }
    );

    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/integrations/ecoflow-ble-auth/manual',
      payload: {
        userId: 'manual-ble-user'
      }
    });

    expect(response.statusCode).toBe(200);
    expect(controlPlaneClient.setEcoFlowBLEAuthUserID).toHaveBeenCalledWith(expect.objectContaining({
      userSubject: 'dev-user',
      userId: 'manual-ble-user'
    }));
    await app.close();
  });
});
