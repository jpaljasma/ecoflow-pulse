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
import type { SolarForecastClient, SolarOutlookResponse } from '../src/grpc/solarForecastClient.js';
import type { WeatherClient, WeatherForecastResponse, WeatherYesterdayVerificationResponse } from '../src/grpc/weatherClient.js';
import type { TelemetryHistoryClient } from '../src/grpc/telemetryClient.js';

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
    getEnergyPvPortHistory: vi.fn(),
    close: vi.fn()
  } as unknown as TelemetryHistoryClient;
}

function makeDeviceClient(): DeviceClient {
  return {
    listDevices: vi.fn(),
    getDevice: vi.fn(),
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

function sampleCurrentUser(overrides: Partial<CurrentUser> = {}): CurrentUser {
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
    updatedAtUnixMs: '1773430800000',
    ...overrides
  };
}

function sampleBootstrap(userOverrides: Partial<CurrentUser> = {}): CurrentUserBootstrap {
  return {
    user: sampleCurrentUser(userOverrides),
    authorization: {
      tokenRoles: ['viewer'],
      deviceCount: 3
    }
  };
}

function makeControlPlaneClient(
  overrides: Partial<ControlPlaneClient> = {},
  userOverrides: Partial<CurrentUser> = {}
): ControlPlaneClient {
  return {
    getCurrentUser: vi.fn(async () => sampleBootstrap(userOverrides)),
    updateCurrentUser: vi.fn(),
    refreshCurrentUserIdentity: vi.fn(),
    listProviderCredentials: vi.fn(async () => []),
    createProviderCredential: vi.fn(),
    updateProviderCredential: vi.fn(),
    setProviderCredentialActive: vi.fn(),
    listUserDevices: vi.fn(),
    listDevices: vi.fn(),
    listAvailableProviderDevices: vi.fn(async () => ({ devices: [], hasActiveCredentials: false })),
    testProviderDeviceMQTT: vi.fn(),
    enableProviderDevice: vi.fn(),
    importProviderDevice: vi.fn(),
    close: vi.fn(),
    ...overrides
  };
}

function sampleForecast(): WeatherForecastResponse {
  return {
    forecast: {
      issuedAtUnixMs: '1773430800000',
      timezone: 'America/New_York',
      unitSystem: 'metric',
      panelTiltDegrees: 45,
      panelAzimuthDegrees: 0,
      provenance: {
        source: 'open_meteo',
        modelSelection: 'best_match',
        actualSource: 'past_days'
      },
      current: {
        timestampIso: '2026-03-12T15:00:00.000Z',
        weatherCode: 2,
        weatherLabel: 'Partly cloudy',
        temperature2m: { raw: 15.4, corrected: 15.1, unit: 'C' }
      },
      hourly: [],
      daily: [
        {
          dateIso: '2026-03-17',
          weatherCode: 71,
          weatherLabel: 'Snow'
        }
      ]
    }
  };
}

function sampleVerification(): WeatherYesterdayVerificationResponse {
  return {
    verification: {
      issuedAtUnixMs: '1773344400000',
      timezone: 'America/New_York',
      verificationSource: 'snapshot',
      provenance: {
        source: 'open_meteo',
        modelSelection: 'best_match',
        actualSource: 'past_days',
        verificationSource: 'snapshot'
      },
      summary: {
        comparedHours: 0,
        matchedHours: 0
      },
      hours: []
    }
  };
}

function makeWeatherClient(overrides: Partial<WeatherClient> = {}): WeatherClient {
  return {
    get7DayForecast: vi.fn(async () => sampleForecast()),
    getYesterdayVerification: vi.fn(async () => sampleVerification()),
    close: vi.fn(),
    ...overrides
  };
}

function sampleSolarOutlook(): SolarOutlookResponse {
  return {
    outlook: {
      scope: {
        mode: 'all',
        resolvedDeviceIds: ['device-1', 'device-2']
      },
      provenance: {
        forecastSource: 'solarforecastd',
        forecastModel: 'deterministic_baseline_v1',
        servedVariant: 'site_calibrated',
        baselineModel: 'deterministic_baseline_v1',
        calibrationApplied: true,
        calibrationSampleCount: 24,
        calibrationUpdatedAtUnixMs: '1773430200000',
        sameDayCurtailmentApplied: false,
        actualsSource: 'telemetry_rollups',
        weatherSource: 'open_meteo',
        weatherModelSelection: 'best_match',
        timezone: 'America/New_York',
        canonicalLocationKey: 'grid-key',
        issuedAtUnixMs: '1773430800000',
        refreshedAtUnixMs: '1773430860000'
      },
      capacity: {
        estimatedPeakWatts: 1680,
        observedPvWatts: 1230,
        method: 'live_pv_and_irradiance'
      },
      today: {
        dateIso: '2026-03-18',
        actualGeneratedKwh: 5.2,
        forecastRemainingKwh: 1.8,
        forecastTotalKwh: 7,
        estimatedPeakWatts: 1680,
        peakTimeIso: '2026-03-18T18:00:00.000Z',
        confidence: 'high'
      },
      daily: [],
      next24Hours: []
    }
  };
}

function makeSolarForecastClient(overrides: Partial<SolarForecastClient> = {}): SolarForecastClient {
  return {
    getSolarOutlook: vi.fn(async () => sampleSolarOutlook()),
    close: vi.fn(),
    ...overrides
  };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('pulse-platform weather routes', () => {
  it('uses the saved weather profile location and fixed panel defaults for forecast requests', async () => {
    const controlPlaneClient = makeControlPlaneClient();
    const weatherClient = makeWeatherClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient,
      weatherClient
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/weather/forecast'
    });

    expect(response.statusCode).toBe(200);
    expect(controlPlaneClient.getCurrentUser).toHaveBeenCalledOnce();
    expect(weatherClient.get7DayForecast).toHaveBeenCalledWith(
      expect.objectContaining({
        latitude: 42.6159,
        longitude: -77.4014,
        unitSystem: 'metric',
        panelTiltDegrees: 45,
        panelAzimuthDegrees: 0,
        timezone: 'America/New_York'
      })
    );
    expect(response.json()).toEqual(sampleForecast());
    expect(response.json().forecast.daily[0]?.dateIso).toBe('2026-03-17');

    await app.close();
  });

  it('uses the saved weather profile location for solar outlook requests', async () => {
    const controlPlaneClient = makeControlPlaneClient();
    const solarForecastClient = makeSolarForecastClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient,
      solarForecastClient
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/solar/outlook'
    });

    expect(response.statusCode).toBe(200);
    expect(solarForecastClient.getSolarOutlook).toHaveBeenCalledWith(
      expect.objectContaining({
        latitude: 42.6159,
        longitude: -77.4014,
        timezone: 'America/New_York',
        panelTiltDegrees: 45,
        panelAzimuthDegrees: 0,
        useAllDevices: true
      })
    );
    expect(response.json()).toEqual(sampleSolarOutlook());

    await app.close();
  });

  it('passes through device scope for solar outlook requests', async () => {
    const controlPlaneClient = makeControlPlaneClient();
    const solarForecastClient = makeSolarForecastClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient,
      solarForecastClient
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/solar/outlook?scope=device&deviceId=019c9f0e-4521-775d-873e-e80039f16d75'
    });

    expect(response.statusCode).toBe(200);
    expect(solarForecastClient.getSolarOutlook).toHaveBeenCalledWith(
      expect.objectContaining({
        useAllDevices: false,
        deviceId: '019c9f0e-4521-775d-873e-e80039f16d75'
      })
    );

    await app.close();
  });

  it('rejects device-scoped solar outlook requests without a device id', async () => {
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient: makeControlPlaneClient(),
      solarForecastClient: makeSolarForecastClient()
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/solar/outlook?scope=device'
    });

    expect(response.statusCode).toBe(400);
    expect(response.json()).toEqual({
      error: 'invalid_request',
      message: 'deviceId is required when scope=device'
    });

    await app.close();
  });

  it('falls back to auto timezone when the saved profile timezone is blank', async () => {
    const controlPlaneClient = makeControlPlaneClient({}, { timezone: '   ' });
    const weatherClient = makeWeatherClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient,
      weatherClient
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/weather/yesterday'
    });

    expect(response.statusCode).toBe(200);
    expect(weatherClient.getYesterdayVerification).toHaveBeenCalledWith(
      expect.objectContaining({
        timezone: 'auto'
      })
    );
    expect(response.json()).toEqual(sampleVerification());

    await app.close();
  });

  it('returns a profile-actionable error when weather location consent is missing', async () => {
    const controlPlaneClient = makeControlPlaneClient({}, { weatherLocationEnabled: false, hasWeatherLocation: false });
    const weatherClient = makeWeatherClient();
    const app = buildApp(baseConfig(), makeHistoryClient(), makeDeviceClient(), makeInferenceClient(), {
      controlPlaneClient,
      weatherClient
    });

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/weather/forecast'
    });

    expect(response.statusCode).toBe(409);
    expect(weatherClient.get7DayForecast).not.toHaveBeenCalled();
    expect(response.json()).toEqual({
      error: 'weather_location_required',
      message: 'Enable weather location consent and save a weather location in your profile first.',
      action: {
        label: 'Open profile',
        target: '/profile'
      }
    });

    await app.close();
  });

  it('maps gRPC failures to HTTP responses', async () => {
    const weatherClient = makeWeatherClient({
      get7DayForecast: vi.fn(async () => {
        const error = new Error('upstream unavailable') as ServiceError;
        error.code = grpcStatus.UNAVAILABLE;
        error.details = 'weather upstream unavailable';
        throw error;
      })
    });
    const app = buildApp(
      baseConfig(),
      makeHistoryClient(),
      makeDeviceClient(),
      makeInferenceClient(),
      {
        controlPlaneClient: makeControlPlaneClient(),
        weatherClient
      }
    );

    const response = await app.inject({
      method: 'GET',
      url: '/api/v1/weather/forecast'
    });

    expect(response.statusCode).toBe(503);
    expect(response.json()).toEqual({
      error: 'upstream_grpc_error',
      message: 'weather upstream unavailable',
      grpcCode: grpcStatus.UNAVAILABLE
    });

    await app.close();
  });
});
