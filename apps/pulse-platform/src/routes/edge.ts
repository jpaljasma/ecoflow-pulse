import type { FastifyInstance, preHandlerHookHandler } from 'fastify';
import { z } from 'zod';

import type { AppConfig } from '../config.js';
import type {
  EdgeClient,
  EdgeCollector,
  EdgeDeviceSource
} from '../grpc/edgeClient.js';
import { edgeDeviceSourceFilterStatuses } from '../grpc/edgeClient.js';
import {
  getAuthHeader,
  getRequestID,
  handleGrpcRouteError,
  resolveUserSubject
} from './currentUserContext.js';

const edgeUuidSchema = z.string().trim().uuid();

const createCollectorSchema = z.object({
  displayName: z.string().trim().min(1).max(128).optional()
});

const collectorParamsSchema = z.object({
  collectorId: edgeUuidSchema
});

const collectorSecretSchema = z.object({
  collectorSecret: z.string().trim().min(1).max(512),
  collectorVersion: z.string().trim().max(128).optional(),
  hostname: z.string().trim().max(255).optional()
});

const enrollCollectorSchema = z.object({
  setupToken: z.string().trim().min(1).max(512),
  collectorVersion: z.string().trim().max(128).optional(),
  hostname: z.string().trim().max(255).optional()
});

const edgeDiscoverySchema = z.object({
  provider: z.string().trim().min(1).max(64).default('ecoflow'),
  transport: z.string().trim().min(1).max(32).default('ble'),
  providerDeviceId: z.string().trim().min(1).max(128),
  displayName: z.string().trim().max(128).optional(),
  model: z.string().trim().max(128).optional(),
  address: z.string().trim().max(256).optional(),
  rssiDbm: z.number().int().min(-150).max(50).optional(),
  observedAtUnixMs: z.number().int().positive().optional(),
  metadata: z.record(z.string(), z.unknown()).optional()
});

const uploadDiscoverySchema = z.object({
  collectorSecret: z.string().trim().min(1).max(512),
  discoveries: z.array(edgeDiscoverySchema).min(0).max(256)
});

const listSourcesQuerySchema = z.object({
  collectorId: edgeUuidSchema.optional(),
  status: z.enum(edgeDeviceSourceFilterStatuses).optional()
});

const approveSourceParamsSchema = z.object({
  sourceId: edgeUuidSchema
});

const approveSourceBodySchema = z.object({
  deviceId: edgeUuidSchema.optional(),
  productName: z.string().trim().max(128).optional(),
  model: z.string().trim().max(128).optional()
});

const edgeTelemetryMaxMetricFields = 128;
const edgeTelemetryMaxMetricKeyBytes = 128;
const edgeTelemetryMaxMetricStringBytes = 4096;

const edgeTelemetryMetricValueSchema = z.union([
  z.number().finite(),
  z
    .string()
    .refine(
      (value) =>
        Buffer.byteLength(value, 'utf8') <= edgeTelemetryMaxMetricStringBytes,
      { message: 'metric string value is too large' }
    ),
  z.boolean(),
  z.null()
]);

const edgeTelemetryMetricKeySchema = z
  .string()
  .min(1)
  .refine(
    (value) =>
      Buffer.byteLength(value, 'utf8') <= edgeTelemetryMaxMetricKeyBytes,
    { message: 'metric key is too large' }
  );

const edgeTelemetryMetricsSchema = z
  .record(edgeTelemetryMetricKeySchema, edgeTelemetryMetricValueSchema)
  .refine(
    (metrics) => Object.keys(metrics).length <= edgeTelemetryMaxMetricFields,
    {
      message: 'too many metric keys'
    }
  );

const edgeTelemetrySampleSchema = z.object({
  provider: z.string().trim().min(1).max(64).default('ecoflow'),
  transport: z.string().trim().min(1).max(32).default('ble'),
  providerDeviceId: z.string().trim().min(1).max(128),
  observedAtUnixMs: z.number().int().positive().optional(),
  clientSampleId: z.string().trim().min(1).max(256).optional(),
  metrics: edgeTelemetryMetricsSchema
});

const uploadTelemetrySchema = z.object({
  collectorSecret: z.string().trim().min(1).max(512),
  samples: z.array(edgeTelemetrySampleSchema).min(0).max(512)
});

export function registerEdgeRoutes(
  app: FastifyInstance,
  config: AppConfig,
  edgeClient: EdgeClient,
  authPreHandler: preHandlerHookHandler
): void {
  app.post(
    '/api/v1/edge/collectors',
    { preHandler: authPreHandler },
    async (request, reply) => {
      try {
        const body = createCollectorSchema.parse(request.body ?? {});
        const created = await edgeClient.createCollector({
          userSubject: resolveUserSubject(config, request),
          displayName: body.displayName,
          authHeader: getAuthHeader(request),
          requestID: getRequestID(request),
          deadlineMs: app.telemetryDeadlineMs
        });
        return reply.code(201).send({
          collector: toCollectorResponse(created.collector),
          setupToken: created.setupToken
        });
      } catch (error) {
        return handleEdgeRouteError(config, reply, error);
      }
    }
  );

  app.get(
    '/api/v1/edge/collectors',
    { preHandler: authPreHandler },
    async (request, reply) => {
      try {
        const collectors = await edgeClient.listCollectors({
          userSubject: resolveUserSubject(config, request),
          authHeader: getAuthHeader(request),
          requestID: getRequestID(request),
          deadlineMs: app.telemetryDeadlineMs
        });
        return { collectors: collectors.map(toCollectorResponse) };
      } catch (error) {
        return handleEdgeRouteError(config, reply, error);
      }
    }
  );

  app.post(
    '/api/v1/edge/collectors/:collectorId/revoke-setup-token',
    { preHandler: authPreHandler },
    async (request, reply) => {
      try {
        const params = collectorParamsSchema.parse(request.params ?? {});
        const collector = await edgeClient.revokeCollectorSetupToken({
          userSubject: resolveUserSubject(config, request),
          collectorId: params.collectorId,
          authHeader: getAuthHeader(request),
          requestID: getRequestID(request),
          deadlineMs: app.telemetryDeadlineMs
        });
        return { collector: toCollectorResponse(collector) };
      } catch (error) {
        return handleEdgeRouteError(config, reply, error);
      }
    }
  );

  app.post('/api/v1/edge/enroll', async (request, reply) => {
    try {
      const body = enrollCollectorSchema.parse(request.body);
      const enrolled = await edgeClient.enrollCollector({
        ...body,
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
      return {
        collector: toCollectorResponse(enrolled.collector),
        collectorSecret: enrolled.collectorSecret,
        collectorEnv: enrolled.collectorEnv
      };
    } catch (error) {
      return handleEdgeRouteError(config, reply, error);
    }
  });

  app.post('/api/v1/edge/heartbeat', async (request, reply) => {
    try {
      const body = collectorSecretSchema.parse(request.body);
      const collector = await edgeClient.heartbeat({
        ...body,
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
      return { collector: toCollectorResponse(collector) };
    } catch (error) {
      return handleEdgeRouteError(config, reply, error);
    }
  });

  app.post('/api/v1/edge/discoveries', async (request, reply) => {
    try {
      const body = uploadDiscoverySchema.parse(request.body);
      return await edgeClient.uploadDiscovery({
        collectorSecret: body.collectorSecret,
        discoveries: body.discoveries,
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
    } catch (error) {
      return handleEdgeRouteError(config, reply, error);
    }
  });

  app.get(
    '/api/v1/edge/device-sources',
    { preHandler: authPreHandler },
    async (request, reply) => {
      try {
        const query = listSourcesQuerySchema.parse(request.query ?? {});
        const sources = await edgeClient.listDeviceSources({
          userSubject: resolveUserSubject(config, request),
          collectorId: query.collectorId,
          status: query.status,
          authHeader: getAuthHeader(request),
          requestID: getRequestID(request),
          deadlineMs: app.telemetryDeadlineMs
        });
        return { sources: sources.map(toSourceResponse) };
      } catch (error) {
        return handleEdgeRouteError(config, reply, error);
      }
    }
  );

  app.post(
    '/api/v1/edge/device-sources/:sourceId/approve',
    { preHandler: authPreHandler },
    async (request, reply) => {
      try {
        const params = approveSourceParamsSchema.parse(request.params ?? {});
        const body = approveSourceBodySchema.parse(request.body ?? {});
        const approved = await edgeClient.approveDeviceSource({
          userSubject: resolveUserSubject(config, request),
          sourceId: params.sourceId,
          deviceId: body.deviceId,
          productName: body.productName,
          model: body.model,
          authHeader: getAuthHeader(request),
          requestID: getRequestID(request),
          deadlineMs: app.telemetryDeadlineMs
        });
        return {
          source: toSourceResponse(approved.source),
          deviceId: approved.deviceId
        };
      } catch (error) {
        return handleEdgeRouteError(config, reply, error);
      }
    }
  );

  app.post('/api/v1/edge/telemetry', async (request, reply) => {
    try {
      const body = uploadTelemetrySchema.parse(request.body);
      return await edgeClient.uploadTelemetry({
        collectorSecret: body.collectorSecret,
        samples: body.samples,
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
    } catch (error) {
      return handleEdgeRouteError(config, reply, error);
    }
  });
}

function handleEdgeRouteError(
  config: AppConfig,
  reply: { code: (statusCode: number) => { send: (body: unknown) => unknown } },
  error: unknown
) {
  if (error instanceof z.ZodError) {
    return reply
      .code(400)
      .send({ error: 'invalid_request', issues: error.issues });
  }
  return handleGrpcRouteError(config, reply, error);
}

function toCollectorResponse(
  collector: EdgeCollector
): Record<string, unknown> {
  return {
    id: collector.id,
    displayName: collector.displayName,
    isActive: collector.isActive,
    lastHeartbeatAtUnixMs: collector.lastHeartbeatAtUnixMs,
    createdAtUnixMs: collector.createdAtUnixMs,
    updatedAtUnixMs: collector.updatedAtUnixMs,
    collectorVersion: collector.collectorVersion,
    hostname: collector.hostname
  };
}

function toSourceResponse(source: EdgeDeviceSource): Record<string, unknown> {
  return {
    id: source.id,
    collectorId: source.collectorId,
    provider: source.provider,
    transport: source.transport,
    displayName: source.displayName,
    model: source.model,
    status: source.status,
    rawStatus: source.rawStatus,
    linkedDeviceId: source.linkedDeviceId,
    rssiDbm: source.rssiDbm,
    lastSeenAtUnixMs: source.lastSeenAtUnixMs,
    createdAtUnixMs: source.createdAtUnixMs,
    updatedAtUnixMs: source.updatedAtUnixMs
  };
}
