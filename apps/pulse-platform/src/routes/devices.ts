import type { FastifyInstance, preHandlerHookHandler } from 'fastify';
import { z } from 'zod';
import type { ServiceError } from '@grpc/grpc-js';
import { status as grpcStatus } from '@grpc/grpc-js';

import type { DeviceClient } from '../grpc/deviceClient.js';
import type { AppConfig } from '../config.js';
import type { InferenceClient, InsightKind } from '../grpc/inferenceClient.js';

export function registerDeviceRoutes(
  app: FastifyInstance,
  config: AppConfig,
  deviceClient: DeviceClient,
  inferenceClient: InferenceClient,
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
      if (isServiceError(error)) {
        return reply.code(mapGrpcCodeToHTTP(error.code)).send({
          error: 'upstream_grpc_error',
          message: error.details || error.message,
          grpcCode: error.code
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
      if (isServiceError(error)) {
        return reply.code(mapGrpcCodeToHTTP(error.code)).send({
          error: 'upstream_grpc_error',
          message: error.details || error.message,
          grpcCode: error.code
        });
      }
      throw error;
    }
  });

  app.get('/api/v1/devices/available', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const available = await deviceClient.listAvailableDevices(request);
      return available;
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
      if (isServiceError(error)) {
        return reply.code(mapGrpcCodeToHTTP(error.code)).send({
          error: 'upstream_grpc_error',
          message: error.details || error.message,
          grpcCode: error.code
        });
      }
      throw error;
    }
  });

  app.post('/api/v1/devices/available/test-mqtt', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const body = z.object({
        provider: z.string().trim().min(1),
        credentialId: z.string().trim().min(1),
        providerDeviceId: z.string().trim().min(1)
      }).parse(request.body);
      const result = await deviceClient.testAvailableDeviceMQTT(request, body);
      return result;
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
      if (error instanceof z.ZodError) {
        return reply.code(400).send({ error: 'invalid_request', issues: error.issues });
      }
      if (isServiceError(error)) {
        return reply.code(mapGrpcCodeToHTTP(error.code)).send({
          error: 'upstream_grpc_error',
          message: error.details || error.message,
          grpcCode: error.code
        });
      }
      throw error;
    }
  });

  app.post('/api/v1/devices/available/enable', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const body = z.object({
        provider: z.string().trim().min(1),
        credentialId: z.string().trim().min(1),
        providerDeviceId: z.string().trim().min(1)
      }).parse(request.body);
      const result = await deviceClient.enableAvailableDevice(request, body);
      return result;
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
      if (error instanceof z.ZodError) {
        return reply.code(400).send({ error: 'invalid_request', issues: error.issues });
      }
      if (isServiceError(error)) {
        return reply.code(mapGrpcCodeToHTTP(error.code)).send({
          error: 'upstream_grpc_error',
          message: error.details || error.message,
          grpcCode: error.code
        });
      }
      throw error;
    }
  });

  app.get('/api/v1/devices/:deviceId/insights', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const params = z.object({ deviceId: z.string().uuid() }).parse(request.params);
      const query = z.object({
        kind: z
          .union([z.enum(['battery_expansion', 'solar_add_on', 'solar_upgrade', 'energy_shift', 'maintenance']), z.array(z.enum(['battery_expansion', 'solar_add_on', 'solar_upgrade', 'energy_shift', 'maintenance']))])
          .optional(),
        maxItems: z.coerce.number().int().positive().max(25).optional()
      }).parse(request.query);

      const kinds = normalizeKinds(query.kind);
      const insights = await inferenceClient.getDeviceInsights({
        deviceId: params.deviceId,
        kinds,
        maxItems: query.maxItems,
        authHeader: extractAuthHeader(request),
        requestID: request.id,
        deadlineMs: app.telemetryDeadlineMs
      });
      return insights;
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
      if (error instanceof z.ZodError) {
        return reply.code(400).send({ error: 'invalid_request', issues: error.issues });
      }
      if (isServiceError(error)) {
        return reply.code(mapGrpcCodeToHTTP(error.code)).send({
          error: 'upstream_grpc_error',
          message: error.details || error.message,
          grpcCode: error.code
        });
      }
      throw error;
    }
  });
}

function isMissingUserSubjectError(error: unknown): boolean {
  return error instanceof Error && error.message === 'missing_user_subject';
}

function normalizeKinds(kind: InsightKind | InsightKind[] | undefined): InsightKind[] | undefined {
  if (!kind) {
    return undefined;
  }
  return Array.isArray(kind) ? kind : [kind];
}

function extractAuthHeader(request: { headers: Record<string, unknown> }): string | undefined {
  const raw = request.headers.authorization;
  return typeof raw === 'string' && raw.trim() ? raw.trim() : undefined;
}

function isServiceError(error: unknown): error is ServiceError {
  return typeof error === 'object' && error !== null && 'code' in error;
}

function mapGrpcCodeToHTTP(code: number): number {
  switch (code) {
    case grpcStatus.INVALID_ARGUMENT:
      return 400;
    case grpcStatus.UNAUTHENTICATED:
      return 401;
    case grpcStatus.PERMISSION_DENIED:
      return 403;
    case grpcStatus.NOT_FOUND:
      return 404;
    case grpcStatus.DEADLINE_EXCEEDED:
      return 504;
    case grpcStatus.UNAVAILABLE:
      return 503;
    default:
      return 500;
  }
}
