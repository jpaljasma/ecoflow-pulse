import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ServiceError } from '@grpc/grpc-js';
import { status as grpcStatus } from '@grpc/grpc-js';

import { buildApp } from '../src/app.js';
import type { AppConfig } from '../src/config.js';
import type {
  ControlPlaneClient,
  CurrentUser,
  CurrentUserBootstrap
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

function makeControlPlaneClient(overrides: Partial<ControlPlaneClient> = {}): ControlPlaneClient {
  return {
    getCurrentUser: vi.fn(async () => sampleBootstrap()),
    updateCurrentUser: vi.fn(async () => sampleCurrentUser()),
    refreshCurrentUserIdentity: vi.fn(async () => sampleCurrentUser()),
    listUserDevices: vi.fn(async () => []),
    listDevices: vi.fn(async () => []),
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
