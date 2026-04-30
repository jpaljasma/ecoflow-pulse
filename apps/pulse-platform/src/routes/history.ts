import type { FastifyInstance, preHandlerHookHandler } from 'fastify';
import { z } from 'zod';
import type { ServiceError } from '@grpc/grpc-js';
import { status as grpcStatus } from '@grpc/grpc-js';

import type { AppConfig } from '../config.js';
import type { InferenceClient } from '../grpc/inferenceClient.js';
import type { TelemetryHistoryClient } from '../grpc/telemetryClient.js';
import {
  buildCompareSolarHistoryView,
  combineSolarHistoryViews
} from '../history/solarView.js';

const resolutionSchema = z.enum(['minute', 'hour', 'day']);
const timeParamSchema = z.union([z.string(), z.number()]);
const booleanQuerySchema = z
  .union([z.boolean(), z.enum(['true', 'false', '1', '0'])])
  .transform((value) => value === true || value === 'true' || value === '1');

const querySchema = z.object({
  resolution: resolutionSchema,
  from: timeParamSchema,
  to: timeParamSchema
});

const compareQuerySchema = querySchema.extend({
  compare: z.enum(['previous_period']).optional().default('previous_period')
});

const solarQuerySchema = z.object({
  from: timeParamSchema,
  to: timeParamSchema,
  compareFrom: timeParamSchema.optional(),
  compareTo: timeParamSchema.optional(),
  windowStartMinutes: z.coerce.number().int().min(0).max(1430).optional(),
  windowEndMinutes: z.coerce.number().int().min(10).max(1440).optional(),
  compare: z.enum(['previous_period']).optional().default('previous_period')
}).superRefine(validateSolarWindow);

const solarFleetQuerySchema = z.object({
  deviceId: z.union([z.string().uuid(), z.array(z.string().uuid()).nonempty()]),
  from: timeParamSchema,
  to: timeParamSchema,
  compareFrom: timeParamSchema.optional(),
  compareTo: timeParamSchema.optional(),
  windowStartMinutes: z.coerce.number().int().min(0).max(1430).optional(),
  windowEndMinutes: z.coerce.number().int().min(10).max(1440).optional(),
  compare: z.enum(['previous_period']).optional().default('previous_period')
}).superRefine(validateSolarWindow);

const energyDashboardQuerySchema = z
  .object({
    deviceId: z.string().uuid().optional(),
    scope: z.enum(['device', 'all']).optional().default('device'),
    preset: z.enum(['today', 'past24h', 'yesterday', 'last7d', 'last30d', 'thisWeek', 'previousWeek', 'thisMonth', 'lastMonth', 'last12m']),
    timezone: z.string().trim().min(1),
    includeComparison: booleanQuerySchema.optional().default(true),
    gridPricePerKwh: z.coerce.number().finite().nonnegative().optional(),
    currency: z.string().trim().min(1).max(8).optional()
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

export function registerHistoryRoutes(
  app: FastifyInstance,
  config: AppConfig,
  historyClient: TelemetryHistoryClient,
  inferenceClient: InferenceClient,
  authPreHandler: preHandlerHookHandler
): void {
  const historyPreHandlers = [app.rateLimit(app.historyRateLimit), authPreHandler];

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

  app.get(
    '/api/v1/devices/:deviceId/history/solar',
    { preHandler: historyPreHandlers },
    async (request, reply) => {
      try {
        const params = z.object({ deviceId: z.string().uuid() }).parse(request.params);
        const query = solarQuerySchema.parse(request.query);
        const compareWindow = normalizeCompareWindow(query.compareFrom, query.compareTo);
        const result = await historyClient.compareRollupRange({
          deviceId: params.deviceId,
          resolution: 'minute',
          fromUnixMs: normalizeTime(query.from),
          toUnixMs: normalizeTime(query.to),
          usePreviousPeriod: !compareWindow && query.compare === 'previous_period',
          ...(compareWindow ?? {}),
          authHeader: extractAuthHeader(request),
          requestID: request.id,
          deadlineMs: app.telemetryDeadlineMs
        });
        return buildCompareSolarHistoryView(result, {
          windowStartMinutes: query.windowStartMinutes,
          windowEndMinutes: query.windowEndMinutes
        });
      } catch (error) {
        return handleRouteError(reply, error);
      }
    }
  );

  app.get(
    '/api/v1/history/solar/fleet',
    { preHandler: historyPreHandlers },
    async (request, reply) => {
      try {
        const query = solarFleetQuerySchema.parse(request.query);
        const deviceIds = normalizeDeviceIDs(query.deviceId);
        const compareWindow = normalizeCompareWindow(query.compareFrom, query.compareTo);
        const views = await Promise.all(
          deviceIds.map(async (deviceId) => {
            const result = await historyClient.compareRollupRange({
              deviceId,
              resolution: 'minute',
              fromUnixMs: normalizeTime(query.from),
              toUnixMs: normalizeTime(query.to),
              usePreviousPeriod: !compareWindow && query.compare === 'previous_period',
              ...(compareWindow ?? {}),
              authHeader: extractAuthHeader(request),
              requestID: request.id,
              deadlineMs: app.telemetryDeadlineMs
            });
            return buildCompareSolarHistoryView(result, {
              windowStartMinutes: query.windowStartMinutes,
              windowEndMinutes: query.windowEndMinutes
            });
          })
        );
        return combineSolarHistoryViews(views);
      } catch (error) {
        return handleRouteError(reply, error);
      }
    }
  );

  app.get(
    '/api/v1/energy/dashboard',
    { preHandler: historyPreHandlers },
    async (request, reply) => {
      try {
        const query = energyDashboardQuerySchema.parse(request.query);
        const result = await historyClient.getEnergyDashboard({
          deviceId: query.scope === 'device' ? query.deviceId : undefined,
          useAllDevices: query.scope === 'all',
          preset: query.preset,
          timezone: query.timezone,
          includeComparison: query.includeComparison,
          gridPricePerKwh: query.gridPricePerKwh,
          currency: query.currency,
          authHeader: extractAuthHeader(request),
          userSubject: resolveUserSubject(config, request),
          requestID: request.id,
          deadlineMs: app.telemetryDeadlineMs
        });
        return result;
      } catch (error) {
        return handleRouteError(reply, error);
      }
    }
  );

  app.get(
    '/api/v1/energy/pv-history',
    { preHandler: historyPreHandlers },
    async (request, reply) => {
      try {
        const query = energyDashboardQuerySchema.parse(request.query);
        const result = await historyClient.getEnergyPvPortHistory({
          deviceId: query.scope === 'device' ? query.deviceId : undefined,
          useAllDevices: query.scope === 'all',
          preset: query.preset,
          timezone: query.timezone,
          authHeader: extractAuthHeader(request),
          userSubject: resolveUserSubject(config, request),
          requestID: request.id,
          deadlineMs: app.telemetryDeadlineMs
        });
        return { pvPortHistory: result };
      } catch (error) {
        return handleRouteError(reply, error);
      }
    }
  );

  app.get(
    '/api/v1/energy/comparison-insight',
    { preHandler: historyPreHandlers },
    async (request, reply) => {
      try {
        const query = energyDashboardQuerySchema.parse(request.query);
        const result = await inferenceClient.getEnergyComparisonInsight({
          deviceId: query.scope === 'device' ? query.deviceId : undefined,
          useAllDevices: query.scope === 'all',
          preset: query.preset,
          timezone: query.timezone,
          gridPricePerKwh: query.gridPricePerKwh,
          currency: query.currency,
          authHeader: extractAuthHeader(request),
          userSubject: resolveUserSubject(config, request),
          requestID: request.id,
          deadlineMs: app.telemetryDeadlineMs
        });
        return result;
      } catch (error) {
        return handleRouteError(reply, error);
      }
    }
  );
}

function validateSolarWindow(
  value: {
    windowStartMinutes?: number;
    windowEndMinutes?: number;
  },
  ctx: z.RefinementCtx
): void {
  const hasStart = value.windowStartMinutes !== undefined;
  const hasEnd = value.windowEndMinutes !== undefined;
  if (hasStart !== hasEnd) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      path: hasStart ? ['windowEndMinutes'] : ['windowStartMinutes'],
      message: 'windowStartMinutes and windowEndMinutes must be provided together'
    });
    return;
  }
  if (!hasStart || !hasEnd) {
    return;
  }
  if ((value.windowStartMinutes ?? 0) % 10 !== 0 || (value.windowEndMinutes ?? 0) % 10 !== 0) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      path: ['windowStartMinutes'],
      message: 'solar history window bounds must align to 10-minute buckets'
    });
    return;
  }
  if ((value.windowEndMinutes ?? 0) <= (value.windowStartMinutes ?? 0)) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      path: ['windowEndMinutes'],
      message: 'windowEndMinutes must be greater than windowStartMinutes'
    });
  }
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

function resolveUserSubject(
  config: AppConfig,
  request: { auth?: { subject?: string }; headers: Record<string, unknown> }
): string | undefined {
  if (request.auth?.subject) {
    return request.auth.subject;
  }
  if (config.auth.mode !== 'noop') {
    return undefined;
  }
  const fromHeader = request.headers['x-user-subject'];
  if (typeof fromHeader === 'string' && fromHeader.trim()) {
    return fromHeader.trim();
  }
  return config.devUserSubject;
}

function normalizeDeviceIDs(deviceIDValue: string | string[]): string[] {
  if (Array.isArray(deviceIDValue)) {
    return deviceIDValue;
  }
  return [deviceIDValue];
}

function normalizeCompareWindow(
  compareFrom: string | number | undefined,
  compareTo: string | number | undefined
): { compareFromUnixMs: string; compareToUnixMs: string } | undefined {
  if (compareFrom === undefined && compareTo === undefined) {
    return undefined;
  }
  if (compareFrom === undefined || compareTo === undefined) {
    throw new Error('invalid time value: compare_from and compare_to must both be set');
  }
  return {
    compareFromUnixMs: normalizeTime(compareFrom),
    compareToUnixMs: normalizeTime(compareTo)
  };
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
