import type { FastifyInstance, preHandlerHookHandler } from 'fastify';

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
        buildWeatherCacheKey(input),
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
        buildWeatherCacheKey(input),
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
  key: string,
  ttlMs: number,
  loader: () => Promise<T>
): Promise<T> {
  if (!cache) {
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
}): string {
  return JSON.stringify({
    latitude: input.latitude,
    longitude: input.longitude,
    unitSystem: input.unitSystem,
    panelTiltDegrees: input.panelTiltDegrees,
    panelAzimuthDegrees: input.panelAzimuthDegrees,
    timezone: input.timezone
  });
}
