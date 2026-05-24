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
