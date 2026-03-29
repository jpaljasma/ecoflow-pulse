import type { FastifyInstance, preHandlerHookHandler } from 'fastify';
import { z } from 'zod';

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

const solarOutlookQuerySchema = z
  .object({
    scope: z.enum(['all', 'device']).optional().default('all'),
    deviceId: z.string().uuid().optional()
  })
  .superRefine((value, ctx) => {
    if (value.scope === 'device' && !value.deviceId) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['deviceId'],
        message: 'deviceId is required when scope=device'
      });
    }
  });

export function registerSolarRoutes(
  app: FastifyInstance,
  config: AppConfig,
  controlPlaneClient: ControlPlaneClient,
  solarForecastClient: SolarForecastClient,
  authPreHandler: preHandlerHookHandler
): void {
  app.get('/api/v1/solar/outlook', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const query = solarOutlookQuerySchema.parse(request.query);
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
        deviceId: query.scope === 'device' ? query.deviceId : undefined,
        useAllDevices: query.scope !== 'device',
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
    } catch (error) {
      if (error instanceof z.ZodError) {
        return reply.code(400).send({
          error: 'invalid_request',
          message: error.issues[0]?.message ?? 'invalid request'
        });
      }
      return handleGrpcRouteError(config, reply, error);
    }
  });
}
