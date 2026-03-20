import type { FastifyInstance, preHandlerHookHandler } from 'fastify';

import type { AppConfig } from '../config.js';
import type { ControlPlaneClient } from '../grpc/controlPlaneClient.js';
import type { WeatherClient } from '../grpc/weatherClient.js';
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
  authPreHandler: preHandlerHookHandler
): void {
  app.get('/api/v1/weather/forecast', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const context = await loadWeatherContext(app, config, controlPlaneClient, request);
      if (!context) {
        return reply.code(409).send(buildMissingWeatherLocationError());
      }
      const result = await weatherClient.get7DayForecast({
        latitude: context.location.latitude,
        longitude: context.location.longitude,
        unitSystem: 'metric',
        panelTiltDegrees: 45,
        panelAzimuthDegrees: 0,
        timezone: context.timezone,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
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
      const result = await weatherClient.getYesterdayVerification({
        latitude: context.location.latitude,
        longitude: context.location.longitude,
        unitSystem: 'metric',
        panelTiltDegrees: 45,
        panelAzimuthDegrees: 0,
        timezone: context.timezone,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
      return result;
    } catch (error) {
      return handleGrpcRouteError(config, reply, error);
    }
  });
}
