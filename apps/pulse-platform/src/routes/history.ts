import type { FastifyInstance, preHandlerHookHandler } from 'fastify';
import { z } from 'zod';
import type { ServiceError } from '@grpc/grpc-js';
import { status as grpcStatus } from '@grpc/grpc-js';

import type { TelemetryHistoryClient } from '../grpc/telemetryClient.js';

const resolutionSchema = z.enum(['minute', 'hour', 'day']);
const timeParamSchema = z.union([z.string(), z.number()]);

const querySchema = z.object({
  resolution: resolutionSchema,
  from: timeParamSchema,
  to: timeParamSchema
});

const compareQuerySchema = querySchema.extend({
  compare: z.enum(['previous_period']).optional().default('previous_period')
});

export function registerHistoryRoutes(
  app: FastifyInstance,
  historyClient: TelemetryHistoryClient,
  authPreHandler: preHandlerHookHandler,
  historyRateLimitPreHandler: preHandlerHookHandler
): void {
  const historyPreHandlers = [historyRateLimitPreHandler, authPreHandler];

  app.get(
    '/api/v1/devices/:deviceId/history',
    { preHandler: historyPreHandlers },
    async (request, reply) => {
      try {
        const params = z.object({ deviceId: z.string().uuid() }).parse(request.params);
        const query = querySchema.parse(request.query);
        const result = await historyClient.queryRollupRange({
          deviceId: params.deviceId,
          resolution: query.resolution,
          fromUnixMs: normalizeTime(query.from),
          toUnixMs: normalizeTime(query.to),
          authHeader: extractAuthHeader(request),
          requestID: request.id,
          deadlineMs: app.telemetryDeadlineMs
        });
        return {
          deviceId: result.deviceId,
          resolution: result.resolution,
          fromUnixMs: result.fromUnixMs,
          toUnixMs: result.toUnixMs,
          points: result.points
        };
      } catch (error) {
        return handleRouteError(reply, error);
      }
    }
  );

  app.get(
    '/api/v1/devices/:deviceId/history/compare',
    { preHandler: historyPreHandlers },
    async (request, reply) => {
      try {
        const params = z.object({ deviceId: z.string().uuid() }).parse(request.params);
        const query = compareQuerySchema.parse(request.query);
        const result = await historyClient.compareRollupRange({
          deviceId: params.deviceId,
          resolution: query.resolution,
          fromUnixMs: normalizeTime(query.from),
          toUnixMs: normalizeTime(query.to),
          usePreviousPeriod: query.compare === 'previous_period',
          authHeader: extractAuthHeader(request),
          requestID: request.id,
          deadlineMs: app.telemetryDeadlineMs
        });
        return {
          current: result.current,
          previous: result.previous
        };
      } catch (error) {
        return handleRouteError(reply, error);
      }
    }
  );
}

function normalizeTime(value: string | number): string {
  if (typeof value === 'number' || /^\d+$/.test(String(value).trim())) {
    const parsed = typeof value === 'number' ? value : Number.parseInt(String(value).trim(), 10);
    return String(parsed);
  }
  const parsed = Date.parse(String(value));
  if (Number.isNaN(parsed)) {
    throw new Error(`invalid time value: ${value}`);
  }
  return String(parsed);
}

function extractAuthHeader(request: { headers: Record<string, unknown> }): string | undefined {
  const raw = request.headers.authorization;
  if (typeof raw === 'string' && raw.trim()) {
    return raw.trim();
  }
  return undefined;
}

function handleRouteError(reply: { code: (code: number) => { send: (body: unknown) => unknown } }, error: unknown) {
  if (error instanceof z.ZodError) {
    return reply.code(400).send({ error: 'invalid_request', issues: error.issues });
  }
  if (error instanceof Error && error.message.startsWith('invalid time value:')) {
    return reply.code(400).send({ error: 'invalid_request', message: error.message });
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
