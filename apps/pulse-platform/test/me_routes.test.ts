import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ServiceError } from '@grpc/grpc-js';
import { status as grpcStatus } from '@grpc/grpc-js';

import { buildApp } from '../src/app.js';
import type { AppConfig } from '../src/config.js';
import type {
  ControlPlaneClient,
  CurrentUser,
  CurrentUserBootstrap,
  ProviderCredential
} from '../src/grpc/controlPlaneClient.js';
import type { DeviceClient } from '../src/grpc/deviceClient.js';
import type { InferenceClient } from '../src/grpc/inferenceClient.js';
import type { TelemetryHistoryClient } from '../src/grpc/telemetryClient.js';
import { resetPublicMetrics } from '../src/metrics.js';

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

function makeHistoryClient(): TelemetryHistoryClient {
  return {
    getSnapshot: vi.fn(),
    queryRollupRange: vi.fn(),
    compareRollupRange: vi.fn(),
    getEnergyDashboard: vi.fn(),
    getPvHistory: vi.fn(),
    close: vi.fn()
  } as unknown as TelemetryHistoryClient;
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
    getEnergyComparisonInsight: vi.fn(),
    close: vi.fn()
  } as unknown as InferenceClient;
}

function sampleCurrentUser(): CurrentUser {
  return {
    id: '019d2b2c-98cd-7f33-b39d-5c8b7fd4c111',
    keycloakSubject: 'kc-user-1',
    email: 'user@example.com',
    emailVerified: true,
    displayName: 'Pulse User',
    displayNameSource: 'pulse',
    avatarUrl: 'https://example.com/avatar.png',
    authMethod: 'google',
    givenName: 'Pulse',
    familyName: 'User',
    locale: 'en-US',
    timezone: 'America/New_York',
    weatherLocationEnabled: true,
    weatherLocationSource: 'auto',
    weatherLocationLabel: 'Naples, NY',
    weatherLatitude: 42.6159,
    weatherLongitude: -77.4014,
    hasWeatherLocation: true,
    lastLoginAtUnixMs: '1773430800000',
    createdAtUnixMs: '1773430000000',
    updatedAtUnixMs: '1773430800000'
  };
}

function sampleBootstrap(): CurrentUserBootstrap {
  return {
    user: sampleCurrentUser(),
    authorization: {
      tokenRoles: ['viewer'],
      deviceCount: 3
    }
  };
}

function sampleProviderCredential(): ProviderCredential {
  return {
    id: '019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
    provider: 'ecoflow',
    accessKeyMask: 'AK12...7890',
    config: {},
    isActive: true,
    createdAtUnixMs: '1773430000000',
    updatedAtUnixMs: '1773430800000'
  };
}

function makeControlPlaneClient(overrides: Partial<ControlPlaneClient> = {}): ControlPlaneClient {
  return {
    getCurrentUser: vi.fn(async () => sampleBootstrap()),
    updateCurrentUser: vi.fn(async () => sampleCurrentUser()),
    refreshCurrentUserIdentity: vi.fn(async () => sampleCurrentUser()),
    listProviderCredentials: vi.fn(async () => [sampleProviderCredential()]),
    createProviderCredential: vi.fn(async () => sampleProviderCredential()),
    updateProviderCredential: vi.fn(async () => sampleProviderCredential()),
    setProviderCredentialActive: vi.fn(async () => sampleProviderCredential()),
    listUserDevices: vi.fn(async () => []),
    listDevices: vi.fn(async () => []),
    listAvailableProviderDevices: vi.fn(async () => ({ devices: [], hasActiveCredentials: false })),
    testProviderDeviceMQTT: vi.fn(),
    enableProviderDevice: vi.fn(),
    importProviderDevice: vi.fn(),
    close: vi.fn(),
    ...overrides
  };
}

afterEach(() => {
  vi.restoreAllMocks();
  resetPublicMetrics();
});

describe('pulse-platform current user routes', () => {
  it('returns the bootstrap payload from /api/v1/me', async () => {
    const controlPlaneClient = makeControlPlaneClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/me'
    });

    expect(response.statusCode).toBe(200);
    expect(controlPlaneClient.getCurrentUser).toHaveBeenCalledOnce();
    expect(response.json()).toEqual({
      user: {
        id: '019d2b2c-98cd-7f33-b39d-5c8b7fd4c111',
        email: 'user@example.com',
        emailVerified: true,
        displayName: 'Pulse User',
        avatarUrl: 'https://example.com/avatar.png',
        authMethod: 'google',
        givenName: 'Pulse',
        familyName: 'User',
        locale: 'en-US',
        timezone: 'America/New_York',
        weatherLocationEnabled: true,
        weatherLocation: {
          label: 'Naples, NY',
          latitude: 42.6159,
          longitude: -77.4014
        }
      },
      authorization: {
        roles: ['viewer'],
        deviceCount: 3
      }
    });

    await app.close();
  });

  it('lists integrations from /api/v1/integrations', async () => {
    const listProviderCredentials = vi.fn(async () => [sampleProviderCredential()]);
    const controlPlaneClient = makeControlPlaneClient({ listProviderCredentials });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/integrations?provider=ecoflow'
    });

    expect(response.statusCode).toBe(200);
    expect(listProviderCredentials).toHaveBeenCalledWith(
      expect.objectContaining({
        provider: 'ecoflow'
      })
    );
    expect(response.json()).toEqual({
      integrations: [
        {
          id: '019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
          provider: 'ecoflow',
          accessKeyMask: 'AK12...7890',
          config: {},
          isActive: true,
          createdAtUnixMs: '1773430000000',
          updatedAtUnixMs: '1773430800000'
        }
      ]
    });

    await app.close();
  });

  it('updates provider credentials through /api/v1/integrations/:credentialId', async () => {
    const updateProviderCredential = vi.fn<ControlPlaneClient['updateProviderCredential']>(async () => ({
      ...sampleProviderCredential(),
      accessKeyMask: 'NEW1...9999',
      config: { region: 'eu' },
      updatedAtUnixMs: '1773431800000'
    }));
    const controlPlaneClient = makeControlPlaneClient({ updateProviderCredential });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'PATCH',
      url: '/api/v1/integrations/019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
      payload: {
        accessKey: 'NEW123456789999',
        accessSecret: 'SECRET123456789999',
        config: { region: 'eu' },
        isActive: true
      }
    });

    expect(response.statusCode).toBe(200);
    expect(updateProviderCredential).toHaveBeenCalledWith(
      expect.objectContaining({
        credentialId: '019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
        accessKey: 'NEW123456789999',
        secretKey: 'SECRET123456789999',
        config: { region: 'eu' },
        isActive: true
      })
    );
    expect(response.json()).toEqual({
      integration: {
        id: '019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
        provider: 'ecoflow',
        accessKeyMask: 'NEW1...9999',
        config: { region: 'eu' },
        isActive: true,
        createdAtUnixMs: '1773430000000',
        updatedAtUnixMs: '1773431800000'
      }
    });

    await app.close();
  });

  it('preserves provider credential config when an update omits config', async () => {
    const updateProviderCredential = vi.fn<ControlPlaneClient['updateProviderCredential']>(async () => ({
      ...sampleProviderCredential(),
      provider: 'pecron',
      accessKeyMask: 'owne...test',
      config: { region: 'eu' },
      updatedAtUnixMs: '1773431800000'
    }));
    const controlPlaneClient = makeControlPlaneClient({ updateProviderCredential });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'PATCH',
      url: '/api/v1/integrations/019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
      payload: {
        accessKey: 'owner@example.test',
        accessSecret: 'new-pecron-password',
        isActive: true
      }
    });

    expect(response.statusCode).toBe(200);
    const updateInput = updateProviderCredential.mock.calls[0]?.[0];
    expect(updateInput).toBeDefined();
    expect(Object.prototype.hasOwnProperty.call(updateInput, 'config')).toBe(false);
    expect(response.json()).toEqual({
      integration: {
        id: '019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
        provider: 'pecron',
        accessKeyMask: 'owne...test',
        config: { region: 'eu' },
        isActive: true,
        createdAtUnixMs: '1773430000000',
        updatedAtUnixMs: '1773431800000'
      }
    });

    await app.close();
  });

  it('patches only provided Anker SOLIX config keys', async () => {
    const updateProviderCredential = vi.fn<ControlPlaneClient['updateProviderCredential']>(async () => ({
      ...sampleProviderCredential(),
      provider: 'anker_solix',
      accessKeyMask: 'owne...test',
      config: { server: 'eu', country: 'FI' },
      updatedAtUnixMs: '1773431800000'
    }));
    const controlPlaneClient = makeControlPlaneClient({ updateProviderCredential });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'PATCH',
      url: '/api/v1/integrations/019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
      payload: {
        accessKey: 'owner@example.test',
        accessSecret: 'new-anker-password',
        config: { country: 'fi' },
        isActive: true
      }
    });

    expect(response.statusCode).toBe(200);
    expect(updateProviderCredential).toHaveBeenCalledWith(
      expect.objectContaining({
        config: { country: 'FI' }
      })
    );
    expect(response.body).not.toContain('new-anker-password');

    await app.close();
  });

  it('creates Pecron integrations with non-secret region config', async () => {
    const createProviderCredential = vi.fn(async () => ({
      ...sampleProviderCredential(),
      provider: 'pecron',
      accessKeyMask: 'owne...test',
      config: { region: 'eu' }
    }));
    const controlPlaneClient = makeControlPlaneClient({ createProviderCredential });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/integrations',
      payload: {
        provider: 'pecron',
        accessKey: 'owner@example.test',
        accessSecret: 'pecron-password',
        config: { region: 'eu' },
        isActive: true
      }
    });

    expect(response.statusCode).toBe(201);
    expect(createProviderCredential).toHaveBeenCalledWith(
      expect.objectContaining({
        provider: 'pecron',
        accessKey: 'owner@example.test',
        secretKey: 'pecron-password',
        config: { region: 'eu' },
        isActive: true
      })
    );
    expect(response.json()).toEqual({
      integration: {
        id: '019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
        provider: 'pecron',
        accessKeyMask: 'owne...test',
        config: { region: 'eu' },
        isActive: true,
        createdAtUnixMs: '1773430000000',
        updatedAtUnixMs: '1773430800000'
      }
    });
    expect(response.body).not.toContain('pecron-password');

    await app.close();
  });

  it('creates Anker SOLIX integrations with non-secret cloud config', async () => {
    const createProviderCredential = vi.fn(async () => ({
      ...sampleProviderCredential(),
      provider: 'anker_solix',
      accessKeyMask: 'owne...test',
      config: { server: 'com', country: 'US' }
    }));
    const controlPlaneClient = makeControlPlaneClient({ createProviderCredential });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/integrations',
      payload: {
        provider: 'anker_solix',
        accessKey: 'owner@example.test',
        accessSecret: 'anker-password',
        config: { server: 'com', country: 'US' },
        isActive: true
      }
    });

    expect(response.statusCode).toBe(201);
    expect(createProviderCredential).toHaveBeenCalledWith(
      expect.objectContaining({
        provider: 'anker_solix',
        accessKey: 'owner@example.test',
        secretKey: 'anker-password',
        config: { server: 'com', country: 'US' },
        isActive: true
      })
    );
    expect(response.json()).toEqual({
      integration: {
        id: '019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111',
        provider: 'anker_solix',
        accessKeyMask: 'owne...test',
        config: { server: 'com', country: 'US' },
        isActive: true,
        createdAtUnixMs: '1773430000000',
        updatedAtUnixMs: '1773430800000'
      }
    });
    expect(response.body).not.toContain('anker-password');

    await app.close();
  });

  it('defaults Anker SOLIX cloud config when clients omit it', async () => {
    const createProviderCredential = vi.fn(async () => ({
      ...sampleProviderCredential(),
      provider: 'anker_solix',
      accessKeyMask: 'owne...test',
      config: { server: 'com', country: 'US' }
    }));
    const controlPlaneClient = makeControlPlaneClient({ createProviderCredential });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/integrations',
      payload: {
        provider: 'anker_solix',
        accessKey: 'owner@example.test',
        accessSecret: 'anker-password',
        isActive: true
      }
    });

    expect(response.statusCode).toBe(201);
    expect(createProviderCredential).toHaveBeenCalledWith(
      expect.objectContaining({
        provider: 'anker_solix',
        config: { server: 'com', country: 'US' }
      })
    );
    expect(response.body).not.toContain('anker-password');

    await app.close();
  });

  it('rejects invalid Anker SOLIX cloud config at the BFF boundary', async () => {
    const createProviderCredential = vi.fn(async () => sampleProviderCredential());
    const controlPlaneClient = makeControlPlaneClient({ createProviderCredential });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/integrations',
      payload: {
        provider: 'anker_solix',
        accessKey: 'owner@example.test',
        accessSecret: 'anker-password',
        config: { server: 'cn', country: 'USA' },
        isActive: true
      }
    });

    expect(response.statusCode).toBe(400);
    expect(createProviderCredential).not.toHaveBeenCalled();
    expect(response.body).not.toContain('anker-password');

    await app.close();
  });

  it('returns conflict when creating a duplicate integration access key', async () => {
    const createProviderCredential = vi.fn(async () => {
      throw {
        code: grpcStatus.ALREADY_EXISTS,
        details: 'provider credential access key already exists',
        message: 'provider credential access key already exists',
        name: 'Error'
      } satisfies Partial<ServiceError>;
    });
    const controlPlaneClient = makeControlPlaneClient({ createProviderCredential });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/v1/integrations',
      payload: {
        provider: 'ecoflow',
        accessKey: 'DUPLICATE123456789',
        accessSecret: 'SECRET123456789',
        isActive: true
      }
    });

    expect(response.statusCode).toBe(409);
    expect(response.json()).toEqual({
      error: 'upstream_grpc_error',
      message: 'provider credential access key already exists',
      grpcCode: grpcStatus.ALREADY_EXISTS
    });

    await app.close();
  });

  it('returns failed-precondition when activating an integration fails validation', async () => {
    const setProviderCredentialActive = vi.fn(async () => {
      throw {
        code: grpcStatus.FAILED_PRECONDITION,
        details: 'validate provider credential discovery: list ecoflow devices: ecoflow api business error code=8513 message=accessKey is invalid',
        message:
          'validate provider credential discovery: list ecoflow devices: ecoflow api business error code=8513 message=accessKey is invalid',
        name: 'Error'
      } satisfies Partial<ServiceError>;
    });
    const controlPlaneClient = makeControlPlaneClient({ setProviderCredentialActive });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'PATCH',
      url: '/api/v1/integrations/019d4a0d-0ff1-7d36-b8a1-b4dcb3c5e111/active',
      payload: {
        isActive: true
      }
    });

    expect(response.statusCode).toBe(412);
    expect(response.json()).toEqual({
      error: 'upstream_grpc_error',
      message:
        'validate provider credential discovery: list ecoflow devices: ecoflow api business error code=8513 message=accessKey is invalid',
      grpcCode: grpcStatus.FAILED_PRECONDITION
    });

    await app.close();
  });

  it('updates profile preferences through /api/v1/me', async () => {
    const updateCurrentUser = vi.fn(async () => ({
      ...sampleCurrentUser(),
      displayName: 'Updated Pulse User',
      timezone: 'America/Los_Angeles',
      weatherLocationEnabled: false,
      weatherLocationSource: 'none',
      weatherLocationLabel: '',
      weatherLatitude: undefined,
      weatherLongitude: undefined,
      hasWeatherLocation: false
    }));
    const controlPlaneClient = makeControlPlaneClient({ updateCurrentUser });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'PATCH',
      url: '/api/v1/me',
      payload: {
        displayName: 'Updated Pulse User',
        timezone: 'America/Los_Angeles',
        weatherLocationEnabled: false
      }
    });

    expect(response.statusCode).toBe(200);
    expect(updateCurrentUser).toHaveBeenCalledWith(
      expect.objectContaining({
        displayName: 'Updated Pulse User',
        timezone: 'America/Los_Angeles',
        weatherLocationEnabled: false,
        hasWeatherLocation: false
      })
    );
    expect(response.json()).toEqual({
      user: {
        id: '019d2b2c-98cd-7f33-b39d-5c8b7fd4c111',
        email: 'user@example.com',
        emailVerified: true,
        displayName: 'Updated Pulse User',
        avatarUrl: 'https://example.com/avatar.png',
        authMethod: 'google',
        givenName: 'Pulse',
        familyName: 'User',
        locale: 'en-US',
        timezone: 'America/Los_Angeles',
        weatherLocationEnabled: false,
        weatherLocation: null
      }
    });

    await app.close();
  });

  it('rejects invalid timezone values before calling gRPC', async () => {
    const updateCurrentUser = vi.fn(async () => sampleCurrentUser());
    const controlPlaneClient = makeControlPlaneClient({ updateCurrentUser });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'PATCH',
      url: '/api/v1/me',
      payload: {
        displayName: 'Updated Pulse User',
        timezone: 'Mars/Olympus_Mons',
        weatherLocationEnabled: false
      }
    });

    expect(response.statusCode).toBe(400);
    expect(updateCurrentUser).not.toHaveBeenCalled();
    expect(response.json()).toEqual(
      expect.objectContaining({
        error: 'invalid_request'
      })
    );

    await app.close();
  });

  it('maps current-user authorization failures to 403', async () => {
    const controlPlaneClient = makeControlPlaneClient({
      getCurrentUser: vi.fn(async () => {
        const error = new Error('not authorized') as ServiceError;
        error.code = grpcStatus.PERMISSION_DENIED;
        error.details = 'not authorized';
        throw error;
      })
    });
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/me'
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

  it('emits scrapeable metrics for current-user requests', async () => {
    const controlPlaneClient = makeControlPlaneClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient
    });

    await app.inject({
      method: 'GET',
      url: '/api/v1/me'
    });

    const metrics = await app.inject({
      method: 'GET',
      url: '/metrics'
    });

    expect(metrics.statusCode).toBe(200);
    expect(metrics.headers['content-type']).toContain('text/plain');
    expect(metrics.body).toContain('pulse_public_http_requests_total');
    expect(metrics.body).toContain('route="/api/v1/me"');
    expect(metrics.body).toContain('method="GET"');

    await app.close();
  });

  it('emits scrapeable metrics for browser auth session recovery outcomes', async () => {
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient: makeControlPlaneClient()
    });

    const event = await app.inject({
      method: 'POST',
      url: '/api/v1/auth/session-events',
      payload: {
        outcome: 'recovered_refresh'
      }
    });

    expect(event.statusCode).toBe(202);

    const metrics = await app.inject({
      method: 'GET',
      url: '/metrics'
    });

    expect(metrics.statusCode).toBe(200);
    expect(metrics.body).toContain('pulse_public_auth_session_recovery_total');
    expect(metrics.body).toContain('outcome="recovered_refresh"');

    await app.close();
  });

  it('emits scrapeable metrics for client-observed REST request outcomes', async () => {
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient: makeControlPlaneClient()
    });

    const event = await app.inject({
      method: 'POST',
      url: '/api/v1/client-metrics/rest',
      payload: {
        route: '/api/v1/energy/dashboard',
        method: 'GET',
        outcome: 'http_error',
        statusClass: '5xx',
        durationMs: 812,
        errorKind: 'status_5xx'
      }
    });

    expect(event.statusCode).toBe(202);

    const metrics = await app.inject({
      method: 'GET',
      url: '/metrics'
    });

    expect(metrics.statusCode).toBe(200);
    expect(metrics.body).toContain('pulse_public_client_rest_requests_total');
    expect(metrics.body).toContain('route="/api/v1/energy/dashboard"');
    expect(metrics.body).toContain('outcome="http_error"');
    expect(metrics.body).toContain('pulse_public_client_rest_errors_total');
    expect(metrics.body).toContain('error_kind="status_5xx"');

    await app.close();
  });

  it('accepts client-observed REST metrics for the energy calendar route', async () => {
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient: makeControlPlaneClient()
    });

    const event = await app.inject({
      method: 'POST',
      url: '/api/v1/client-metrics/rest',
      payload: {
        route: '/api/v1/energy/calendar',
        method: 'GET',
        outcome: 'success',
        statusClass: '2xx',
        durationMs: 45,
        errorKind: 'none'
      }
    });

    expect(event.statusCode).toBe(202);

    await app.close();
  });

  it('accepts client-observed REST metrics for available-device routes', async () => {
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient: makeControlPlaneClient()
    });

    const event = await app.inject({
      method: 'POST',
      url: '/api/v1/client-metrics/rest',
      payload: {
        route: '/api/v1/devices/available',
        method: 'GET',
        outcome: 'success',
        statusClass: '2xx',
        durationMs: 121,
        errorKind: 'none'
      }
    });

    expect(event.statusCode).toBe(202);

    const metrics = await app.inject({
      method: 'GET',
      url: '/metrics'
    });

    expect(metrics.statusCode).toBe(200);
    expect(metrics.body).toContain('route="/api/v1/devices/available"');

    await app.close();
  });

  it('emits scrapeable metrics for client-observed websocket outcomes', async () => {
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient: makeControlPlaneClient()
    });

    const connectionEvent = await app.inject({
      method: 'POST',
      url: '/api/v1/client-metrics/ws',
      payload: {
        eventType: 'connection',
        phase: 'reconnect',
        outcome: 'connected',
        durationMs: 430
      }
    });
    expect(connectionEvent.statusCode).toBe(202);

    const disconnectEvent = await app.inject({
      method: 'POST',
      url: '/api/v1/client-metrics/ws',
      payload: {
        eventType: 'disconnect',
        reason: 'stalled'
      }
    });
    expect(disconnectEvent.statusCode).toBe(202);

    const freshnessEvent = await app.inject({
      method: 'POST',
      url: '/api/v1/client-metrics/ws',
      payload: {
        eventType: 'freshness_transition',
        state: 'stale'
      }
    });
    expect(freshnessEvent.statusCode).toBe(202);

    const recoveryEvent = await app.inject({
      method: 'POST',
      url: '/api/v1/client-metrics/ws',
      payload: {
        eventType: 'stale_recovery',
        durationMs: 9200
      }
    });
    expect(recoveryEvent.statusCode).toBe(202);

    const metrics = await app.inject({
      method: 'GET',
      url: '/metrics'
    });

    expect(metrics.statusCode).toBe(200);
    expect(metrics.body).toContain('pulse_public_client_ws_connections_total');
    expect(metrics.body).toContain('phase="reconnect"');
    expect(metrics.body).toContain('outcome="connected"');
    expect(metrics.body).toContain('pulse_public_client_ws_disconnects_total');
    expect(metrics.body).toContain('reason="stalled"');
    expect(metrics.body).toContain('pulse_public_client_ws_freshness_transitions_total');
    expect(metrics.body).toContain('state="stale"');
    expect(metrics.body).toContain('pulse_public_client_ws_stale_recovery_duration_seconds');

    await app.close();
  });

  it('refreshes current user identity through Keycloak userinfo', async () => {
    const refreshCurrentUserIdentity = vi.fn(async () => ({
      ...sampleCurrentUser(),
      avatarUrl: 'https://example.com/avatar-refreshed.png',
      givenName: 'Jaan',
      familyName: 'Paljasma'
    }));
    const controlPlaneClient = makeControlPlaneClient({ refreshCurrentUserIdentity });
    const app = buildApp(
      {
        ...baseConfig(),
        auth: {
          mode: 'keycloak',
          issuerUrl: 'https://issuer.example/realms/pulse',
          audience: 'pulse-universal-app',
          jwksUrl: 'https://issuer.example/jwks',
          userInfoUrl: 'https://issuer.example/userinfo',
          allowMissingJwt: false
        }
      },
      makeHistoryClient(),
      makeDeviceClient(),
      makeInferenceClient(),
      {
        controlPlaneClient,
        authPreHandler: async (request) => {
          request.auth = { subject: 'kc-user-1' } as never;
        }
      }
    );

    const originalFetch = global.fetch;
    global.fetch = vi.fn(async () =>
      new Response(
        JSON.stringify({
          email: 'jpaljasma@gmail.com',
          email_verified: true,
          name: 'Jaan Paljasma',
          given_name: 'Jaan',
          family_name: 'Paljasma',
          picture: 'https://example.com/avatar-refreshed.png',
          locale: 'en-US'
        }),
        {
          status: 200,
          headers: { 'content-type': 'application/json' }
        }
      )
    ) as typeof fetch;

    try {
      const response = await app.inject({
        method: 'POST',
        url: '/api/v1/me/identity-refresh',
        headers: {
          authorization: 'Bearer test-token'
        }
      });

      expect(response.statusCode).toBe(200);
      expect(refreshCurrentUserIdentity).toHaveBeenCalledWith(
        expect.objectContaining({
          userSubject: 'kc-user-1',
          email: 'jpaljasma@gmail.com',
          displayName: 'Jaan Paljasma',
          avatarUrl: 'https://example.com/avatar-refreshed.png',
          givenName: 'Jaan',
          familyName: 'Paljasma',
          locale: 'en-US'
        })
      );
      expect(response.json()).toEqual({
        user: expect.objectContaining({
          avatarUrl: 'https://example.com/avatar-refreshed.png',
          givenName: 'Jaan',
          familyName: 'Paljasma'
        })
      });
    } finally {
      global.fetch = originalFetch;
      await app.close();
    }
  });
});
