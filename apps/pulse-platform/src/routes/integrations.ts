import type { FastifyInstance, preHandlerHookHandler } from 'fastify';
import { z } from 'zod';

import type { AppConfig } from '../config.js';
import type { ControlPlaneClient, ProviderCredential } from '../grpc/controlPlaneClient.js';
import {
  getAuthHeader,
  getRequestID,
  handleGrpcRouteError,
  resolveUserSubject
} from './currentUserContext.js';

const integrationListQuerySchema = z.object({
  provider: z.string().trim().min(1).max(64).optional()
});

const integrationConfigSchema = z.record(z.unknown());
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

export function registerIntegrationRoutes(
  app: FastifyInstance,
  config: AppConfig,
  controlPlaneClient: ControlPlaneClient,
  authPreHandler: preHandlerHookHandler
): void {
  app.get('/api/v1/integrations', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const query = integrationListQuerySchema.parse(request.query ?? {});
      const integrations = await controlPlaneClient.listProviderCredentials({
        userSubject: resolveUserSubject(config, request),
        provider: query.provider,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
      return {
        integrations: integrations.map(toIntegrationResponse)
      };
    } catch (error) {
      if (error instanceof z.ZodError) {
        return reply.code(400).send({ error: 'invalid_request', issues: error.issues });
      }
      return handleGrpcRouteError(config, reply, error);
    }
  });

  app.post('/api/v1/integrations', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const body = createIntegrationSchema.parse(request.body);
      const integration = await controlPlaneClient.createProviderCredential({
        userSubject: resolveUserSubject(config, request),
        provider: body.provider,
        accessKey: body.accessKey,
        secretKey: body.accessSecret,
        config: body.config,
        isActive: body.isActive,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
      return reply.code(201).send({ integration: toIntegrationResponse(integration) });
    } catch (error) {
      if (error instanceof z.ZodError) {
        return reply.code(400).send({ error: 'invalid_request', issues: error.issues });
      }
      return handleGrpcRouteError(config, reply, error);
    }
  });

  app.patch('/api/v1/integrations/:credentialId', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const params = z.object({ credentialId: z.string().trim().uuid() }).parse(request.params ?? {});
      const body = updateIntegrationSchema.parse(request.body);
      const integration = await controlPlaneClient.updateProviderCredential({
        userSubject: resolveUserSubject(config, request),
        credentialId: params.credentialId,
        accessKey: body.accessKey,
        secretKey: body.accessSecret,
        ...(body.config === undefined ? {} : { config: body.config }),
        isActive: body.isActive,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
      return { integration: toIntegrationResponse(integration) };
    } catch (error) {
      if (error instanceof z.ZodError) {
        return reply.code(400).send({ error: 'invalid_request', issues: error.issues });
      }
      return handleGrpcRouteError(config, reply, error);
    }
  });

  app.patch('/api/v1/integrations/:credentialId/active', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const params = z.object({ credentialId: z.string().trim().uuid() }).parse(request.params ?? {});
      const body = setActiveSchema.parse(request.body);
      const integration = await controlPlaneClient.setProviderCredentialActive({
        userSubject: resolveUserSubject(config, request),
        credentialId: params.credentialId,
        isActive: body.isActive,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
      return { integration: toIntegrationResponse(integration) };
    } catch (error) {
      if (error instanceof z.ZodError) {
        return reply.code(400).send({ error: 'invalid_request', issues: error.issues });
      }
      return handleGrpcRouteError(config, reply, error);
    }
  });
}

function toIntegrationResponse(integration: ProviderCredential): Record<string, unknown> {
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
