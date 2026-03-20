import type { FastifyInstance, preHandlerHookHandler } from 'fastify';

import type { AppConfig } from '../config.js';
import type { ControlPlaneClient } from '../grpc/controlPlaneClient.js';
import type { SolarForecastClient } from '../grpc/solarForecastClient.js';
import {
  buildMissingWeatherLocationError,
  getAuthHeader,
  getRequestID,
  handleGrpcRouteError,
  loadWeatherContext
} from './currentUserContext.js';

export function registerSolarRoutes(
  app: FastifyInstance,
  config: AppConfig,
  controlPlaneClient: ControlPlaneClient,
  solarForecastClient: SolarForecastClient,
  authPreHandler: preHandlerHookHandler
): void {
  app.get('/api/v1/solar/outlook', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const context = await loadWeatherContext(app, config, controlPlaneClient, request);
      if (!context) {
        return reply.code(409).send(buildMissingWeatherLocationError());
      }
      return await solarForecastClient.getSolarOutlook({
        latitude: context.location.latitude,
        longitude: context.location.longitude,
        timezone: context.timezone,
        panelTiltDegrees: 45,
        panelAzimuthDegrees: 0,
        useAllDevices: true,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
    } catch (error) {
      return handleGrpcRouteError(config, reply, error);
    }
  });
}
