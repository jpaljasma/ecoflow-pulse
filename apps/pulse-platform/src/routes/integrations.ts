import type { ServiceError } from '@grpc/grpc-js';
import { status as grpcStatus } from '@grpc/grpc-js';
import type {
  FastifyInstance,
  FastifyRequest,
  preHandlerHookHandler
} from 'fastify';
import { z } from 'zod';

import type { AppConfig } from '../config.js';
import type {
  ControlPlaneClient,
  EcoFlowBLEAuthStatus,
  ProviderCredential
} from '../grpc/controlPlaneClient.js';
import {
  getAuthHeader,
  getRequestID,
  handleGrpcRouteError,
  resolveUserSubject
} from './currentUserContext.js';

const integrationListQuerySchema = z.object({
  provider: z.string().trim().min(1).max(64).optional()
});
const dedicatedCredentialProviders = new Set(['ecoflow_ble']);

const integrationConfigSchema = z.record(z.string(), z.unknown());
const integrationCredentialFields = {
  accessKey: z.string().trim().min(1).max(512),
  accessSecret: z.string().trim().min(1).max(512),
  isActive: z.boolean().optional().default(true)
};

const createIntegrationSchema = z.object({
  provider: z.string().trim().min(1).max(64),
  ...integrationCredentialFields,
  config: integrationConfigSchema.optional().default({})
});

const updateIntegrationSchema = z.object({
  ...integrationCredentialFields,
  config: integrationConfigSchema.optional()
});

const setActiveSchema = z.object({
  isActive: z.boolean()
});

const ecoFlowBLEConnectSchema = z.object({
  email: z.string().trim().email().max(320),
  password: z
    .string()
    .min(1)
    .max(512)
    .refine((value) => value.trim().length > 0, {
      message: 'password required'
    })
});

const ecoFlowBLEManualSchema = z.object({
  userId: z.string().trim().min(1).max(256),
  accountLabel: z.string().trim().max(128).optional()
});

export function registerIntegrationRoutes(
  app: FastifyInstance,
  config: AppConfig,
  controlPlaneClient: ControlPlaneClient,
  authPreHandler: preHandlerHookHandler
): void {
  const ecoFlowBLEConnectPreHandlers = [
    app.rateLimit({ max: 5, timeWindow: 60, cache: 1000 }),
    authPreHandler
  ];

  app.get(
    '/api/v1/integrations',
    { preHandler: authPreHandler },
    async (request, reply) => {
      try {
        const query = integrationListQuerySchema.parse(request.query ?? {});
        const integrations = await controlPlaneClient.listProviderCredentials({
          ...controlPlaneRequestContext(config, app, request),
          provider: query.provider
        });
        return {
          integrations: integrations
            .filter(
              (integration) =>
                !isDedicatedCredentialProvider(integration.provider)
            )
            .map(toIntegrationResponse)
        };
      } catch (error) {
        return handleIntegrationRouteError(config, reply, error);
      }
    }
  );

  app.get(
    '/api/v1/integrations/ecoflow-ble-auth',
    { preHandler: authPreHandler },
    async (request, reply) => {
      try {
        const status = await controlPlaneClient.getEcoFlowBLEAuthStatus({
          ...controlPlaneRequestContext(config, app, request)
        });
        return { status: toEcoFlowBLEAuthStatusResponse(status) };
      } catch (error) {
        return handleIntegrationRouteError(config, reply, error);
      }
    }
  );

  app.post(
    '/api/v1/integrations/ecoflow-ble-auth/connect',
    { preHandler: ecoFlowBLEConnectPreHandlers },
    async (request, reply) => {
      try {
        const body = ecoFlowBLEConnectSchema.parse(request.body);
        const status = await controlPlaneClient.connectEcoFlowBLEAuth({
          ...controlPlaneRequestContext(config, app, request),
          email: body.email,
          password: body.password
        });
        return { status: toEcoFlowBLEAuthStatusResponse(status) };
      } catch (error) {
        return handleEcoFlowBLEConnectError(config, reply, error);
      }
    }
  );

  app.post(
    '/api/v1/integrations/ecoflow-ble-auth/manual',
    { preHandler: authPreHandler },
    async (request, reply) => {
      try {
        const body = ecoFlowBLEManualSchema.parse(request.body);
        const status = await controlPlaneClient.setEcoFlowBLEAuthUserID({
          ...controlPlaneRequestContext(config, app, request),
          userId: body.userId,
          accountLabel: body.accountLabel
        });
        return { status: toEcoFlowBLEAuthStatusResponse(status) };
      } catch (error) {
        return handleIntegrationRouteError(config, reply, error);
      }
    }
  );

  app.post(
    '/api/v1/integrations',
    { preHandler: authPreHandler },
    async (request, reply) => {
      try {
        const body = createIntegrationSchema.parse(request.body);
        rejectDedicatedCredentialProvider(body.provider);
        const integration = await controlPlaneClient.createProviderCredential({
          ...controlPlaneRequestContext(config, app, request),
          provider: body.provider,
          accessKey: body.accessKey,
          secretKey: body.accessSecret,
          config: normalizeProviderCredentialConfig(body.provider, body.config),
          isActive: body.isActive
        });
        return reply
          .code(201)
          .send({ integration: toIntegrationResponse(integration) });
      } catch (error) {
        return handleIntegrationRouteError(config, reply, error);
      }
    }
  );

  app.patch(
    '/api/v1/integrations/:credentialId',
    { preHandler: authPreHandler },
    async (request, reply) => {
      try {
        const params = z
          .object({ credentialId: z.string().trim().uuid() })
          .parse(request.params ?? {});
        const body = updateIntegrationSchema.parse(request.body);
        const integration = await controlPlaneClient.updateProviderCredential({
          ...controlPlaneRequestContext(config, app, request),
          credentialId: params.credentialId,
          accessKey: body.accessKey,
          secretKey: body.accessSecret,
          ...(body.config === undefined
            ? {}
            : { config: normalizeProviderCredentialUpdateConfig(body.config) }),
          isActive: body.isActive
        });
        return { integration: toIntegrationResponse(integration) };
      } catch (error) {
        return handleIntegrationRouteError(config, reply, error);
      }
    }
  );

  app.patch(
    '/api/v1/integrations/:credentialId/active',
    { preHandler: authPreHandler },
    async (request, reply) => {
      try {
        const params = z
          .object({ credentialId: z.string().trim().uuid() })
          .parse(request.params ?? {});
        const body = setActiveSchema.parse(request.body);
        const integration =
          await controlPlaneClient.setProviderCredentialActive({
            ...controlPlaneRequestContext(config, app, request),
            credentialId: params.credentialId,
            isActive: body.isActive
          });
        return { integration: toIntegrationResponse(integration) };
      } catch (error) {
        return handleIntegrationRouteError(config, reply, error);
      }
    }
  );
}

function controlPlaneRequestContext(
  config: AppConfig,
  app: FastifyInstance,
  request: FastifyRequest
) {
  return {
    userSubject: resolveUserSubject(config, request),
    authHeader: getAuthHeader(request),
    requestID: getRequestID(request),
    deadlineMs: app.telemetryDeadlineMs
  };
}

function handleIntegrationRouteError(
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

function handleEcoFlowBLEConnectError(
  config: AppConfig,
  reply: { code: (statusCode: number) => { send: (body: unknown) => unknown } },
  error: unknown
) {
  if (error instanceof z.ZodError) {
    return handleIntegrationRouteError(config, reply, error);
  }
  if (isServiceError(error)) {
    return reply.code(mapIntegrationGrpcCodeToHTTP(error.code)).send({
      error: 'upstream_grpc_error',
      message: 'EcoFlow app login failed. Check the credentials and try again.',
      grpcCode: error.code
    });
  }
  return handleIntegrationRouteError(config, reply, error);
}

function isServiceError(error: unknown): error is ServiceError {
  return typeof error === 'object' && error !== null && 'code' in error;
}

function mapIntegrationGrpcCodeToHTTP(code: number): number {
  switch (code) {
    case grpcStatus.INVALID_ARGUMENT:
      return 400;
    case grpcStatus.FAILED_PRECONDITION:
      return 412;
    case grpcStatus.UNAUTHENTICATED:
      return 401;
    case grpcStatus.PERMISSION_DENIED:
      return 403;
    case grpcStatus.NOT_FOUND:
      return 404;
    case grpcStatus.ALREADY_EXISTS:
      return 409;
    case grpcStatus.DEADLINE_EXCEEDED:
      return 504;
    case grpcStatus.UNAVAILABLE:
      return 503;
    default:
      return 502;
  }
}

function rejectDedicatedCredentialProvider(provider: string): void {
  if (!isDedicatedCredentialProvider(provider)) {
    return;
  }
  throw new z.ZodError([
    {
      code: z.ZodIssueCode.custom,
      path: ['provider'],
      message:
        'EcoFlow BLE credentials must be managed through dedicated auth endpoints'
    }
  ]);
}

function isDedicatedCredentialProvider(provider: string): boolean {
  return dedicatedCredentialProviders.has(provider.trim().toLowerCase());
}

function normalizeProviderCredentialConfig(
  provider: string,
  config: Record<string, unknown> | undefined
): Record<string, unknown> {
  if (provider !== 'anker_solix') {
    return config ?? {};
  }
  return {
    server: normalizeAnkerSolixServer(config?.server),
    country: normalizeAnkerSolixCountry(config?.country)
  };
}

function normalizeProviderCredentialUpdateConfig(
  config: Record<string, unknown>
): Record<string, unknown> {
  const out = { ...config };
  if ('server' in config) {
    out.server = normalizeAnkerSolixServer(config.server, false);
  }
  if ('country' in config) {
    out.country = normalizeAnkerSolixCountry(config.country, false);
  }
  return out;
}

function normalizeAnkerSolixServer(
  value: unknown,
  allowDefault = true
): 'com' | 'eu' {
  if (value === undefined && allowDefault) {
    return 'com';
  }
  const text = typeof value === 'string' ? value.trim().toLowerCase() : '';
  if (text === 'com' || text === 'eu') {
    return text;
  }
  throwAnkerSolixConfigIssue('server', 'Anker SOLIX server must be com or eu');
}

function normalizeAnkerSolixCountry(
  value: unknown,
  allowDefault = true
): string {
  if (value === undefined && allowDefault) {
    return 'US';
  }
  const text = typeof value === 'string' ? value.trim().toUpperCase() : '';
  if (/^[A-Z]{2}$/.test(text)) {
    return text;
  }
  throwAnkerSolixConfigIssue(
    'country',
    'Anker SOLIX country must be an ISO-2 country code'
  );
}

function throwAnkerSolixConfigIssue(path: string, message: string): never {
  throw new z.ZodError([
    {
      code: z.ZodIssueCode.custom,
      path: ['config', path],
      message
    }
  ]);
}

function toIntegrationResponse(
  integration: ProviderCredential
): Record<string, unknown> {
  return {
    id: integration.id,
    provider: integration.provider,
    accessKeyMask: integration.accessKeyMask,
    config: integration.config ?? {},
    isActive: integration.isActive,
    createdAtUnixMs: integration.createdAtUnixMs,
    updatedAtUnixMs: integration.updatedAtUnixMs
  };
}

function toEcoFlowBLEAuthStatusResponse(
  status: EcoFlowBLEAuthStatus
): Record<string, unknown> {
  return {
    connected: status.connected,
    status: status.status,
    accountMask: status.accountMask,
    updatedAtUnixMs: status.updatedAtUnixMs
  };
}
