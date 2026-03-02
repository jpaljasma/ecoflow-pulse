import type { FastifyInstance, preHandlerHookHandler } from 'fastify';

import type { DeviceClient } from '../grpc/deviceClient.js';
import type { AppConfig } from '../config.js';

export function registerDeviceRoutes(
  app: FastifyInstance,
  config: AppConfig,
  deviceClient: DeviceClient,
  authPreHandler: preHandlerHookHandler
) {
  app.get('/api/devices', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const devices = await deviceClient.listDevices(request);
      return { devices };
    } catch (error) {
      if (isMissingUserSubjectError(error)) {
        return reply.code(503).send({
          error: 'missing_user_subject',
          message:
            config.auth.mode === 'noop'
              ? 'Set PULSE_PLATFORM_DEV_SUBJECT or send x-user-subject in noop mode.'
              : 'Authenticated user subject required.'
        });
      }
      throw error;
    }
  });

  app.get('/api/devices/:deviceId', { preHandler: authPreHandler }, async (request, reply) => {
    const routeDeviceId = String((request.params as { deviceId?: string }).deviceId ?? '').trim();
    if (!routeDeviceId) {
      return reply.code(400).send({ error: 'device_id_required' });
    }
    try {
      const device = await deviceClient.getDevice(request, routeDeviceId);
      if (!device) {
        return reply.code(404).send({ error: 'device_not_found' });
      }
      return device;
    } catch (error) {
      if (isMissingUserSubjectError(error)) {
        return reply.code(503).send({
          error: 'missing_user_subject',
          message:
            config.auth.mode === 'noop'
              ? 'Set PULSE_PLATFORM_DEV_SUBJECT or send x-user-subject in noop mode.'
              : 'Authenticated user subject required.'
        });
      }
      throw error;
    }
  });
}

function isMissingUserSubjectError(error: unknown): boolean {
  return error instanceof Error && error.message === 'missing_user_subject';
}
