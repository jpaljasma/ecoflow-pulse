import type { FastifyInstance, FastifyRequest, preHandlerHookHandler } from 'fastify';

import type { AppConfig } from '../config.js';
import type { BffResponseCache } from '../cache/bffCache.js';
import type { ControlPlaneClient } from '../grpc/controlPlaneClient.js';
import type { WeatherClient, WeatherRequest } from '../grpc/weatherClient.js';
import {
  buildMissingWeatherLocationError,
  getAuthHeader,
  getRequestID,
  handleGrpcRouteError,
  loadWeatherContext
} from './currentUserContext.js';

export function registerWeatherRoutes(
  app: FastifyInstance,
  config: AppConfig,
  controlPlaneClient: ControlPlaneClient,
  weatherClient: WeatherClient,
  authPreHandler: preHandlerHookHandler,
  bffCache?: BffResponseCache
): void {
  app.get('/api/v1/weather/forecast', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const context = await loadWeatherContext(app, config, controlPlaneClient, request);
      if (!context) {
        return reply.code(409).send(buildMissingWeatherLocationError());
      }
      const input: WeatherRequest = {
        latitude: context.location.latitude,
        longitude: context.location.longitude,
        unitSystem: 'metric',
        panelTiltDegrees: 45,
        panelAzimuthDegrees: 0,
        timezone: context.timezone,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      };
      const result = await loadWithOptionalBffCache(
        bffCache,
        'weather_forecast',
        buildWeatherCacheKey(input, requestCacheDimensions(request, config)),
        config.bffCache?.weatherForecastTtlMs ?? 0,
        () => weatherClient.get7DayForecast(input)
      );
      return result;
    } catch (error) {
      return handleGrpcRouteError(config, reply, error);
    }
  });

  app.get('/api/v1/weather/yesterday', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const context = await loadWeatherContext(app, config, controlPlaneClient, request);
      if (!context) {
        return reply.code(409).send(buildMissingWeatherLocationError());
      }
      const input: WeatherRequest = {
        latitude: context.location.latitude,
        longitude: context.location.longitude,
        unitSystem: 'metric',
        panelTiltDegrees: 45,
        panelAzimuthDegrees: 0,
        timezone: context.timezone,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      };
      const result = await loadWithOptionalBffCache(
        bffCache,
        'weather_yesterday',
        buildWeatherYesterdayCacheKey(input, requestCacheDimensions(request, config)),
        config.bffCache?.weatherYesterdayTtlMs ?? 0,
        () => weatherClient.getYesterdayVerification(input)
      );
      return result;
    } catch (error) {
      return handleGrpcRouteError(config, reply, error);
    }
  });
}

async function loadWithOptionalBffCache<T>(
  cache: BffResponseCache | undefined,
  namespace: string,
  key: string | undefined,
  ttlMs: number,
  loader: () => Promise<T>
): Promise<T> {
  if (!cache || !key) {
    return await loader();
  }
  return await cache.getOrLoad(namespace, key, ttlMs, loader);
}

function buildWeatherCacheKey(input: {
  latitude: number;
  longitude: number;
  unitSystem: string;
  panelTiltDegrees?: number;
  panelAzimuthDegrees?: number;
  timezone: string;
}, extras: Record<string, string> = {}): string {
  return JSON.stringify({
    latitude: input.latitude,
    longitude: input.longitude,
    unitSystem: input.unitSystem,
    panelTiltDegrees: input.panelTiltDegrees,
    panelAzimuthDegrees: input.panelAzimuthDegrees,
    timezone: input.timezone,
    ...extras
  });
}

function buildWeatherYesterdayCacheKey(
  input: WeatherRequest,
  extras: Record<string, string>
): string | undefined {
  const localDate = localDateBucket(input.timezone, Date.now());
  if (!localDate) {
    return undefined;
  }
  return buildWeatherCacheKey(input, { ...extras, localDate });
}

function requestCacheDimensions(
  request: FastifyRequest,
  config: AppConfig
): Record<string, string> {
  return {
    dataPlane: requestDataPlane(request, config)
  };
}

function requestDataPlane(request: FastifyRequest, config: AppConfig): 'local' | 'cloud' {
  const header = headerString(request, 'x-pulse-data-plane')?.toLowerCase();
  if (header === 'local' || header === 'cloud') {
    return header;
  }
  return config.dataPlane ?? 'local';
}

function headerString(request: FastifyRequest, key: string): string | undefined {
  const value = request.headers[key];
  if (typeof value === 'string') {
    const trimmed = value.trim();
    return trimmed ? trimmed : undefined;
  }
  return undefined;
}

function localDateBucket(timezone: string, nowMs: number): string | undefined {
  if (!timezone || timezone === 'auto') {
    return undefined;
  }
  try {
    const parts = new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit'
    }).formatToParts(new Date(nowMs));
    const year = parts.find((part) => part.type === 'year')?.value;
    const month = parts.find((part) => part.type === 'month')?.value;
    const day = parts.find((part) => part.type === 'day')?.value;
    if (!year || !month || !day) {
      return undefined;
    }
    return `${year}-${month}-${day}`;
  } catch (error) {
    if (error instanceof RangeError) {
      return undefined;
    }
    throw error;
  }
}
