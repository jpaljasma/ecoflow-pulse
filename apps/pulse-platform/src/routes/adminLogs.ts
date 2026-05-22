import type { FastifyInstance, preHandlerHookHandler } from 'fastify';
import { z } from 'zod';

import type { AppConfig } from '../config.js';
import type { ControlPlaneClient } from '../grpc/controlPlaneClient.js';
import {
  getAuthHeader,
  getRequestID,
  handleGrpcRouteError,
  loadCurrentUserBootstrap,
  resolveUserSubject
} from './currentUserContext.js';

const adminLogFilterOptionsSchema = z.object({
  kind: z.enum(['device', 'serial', 'user']).optional(),
  query: z.string().trim().max(128).optional().default(''),
  limit: z.number().int().min(1).max(50).optional().default(12)
});

export function registerAdminLogRoutes(
  app: FastifyInstance,
  config: AppConfig,
  controlPlaneClient: ControlPlaneClient,
  authPreHandler: preHandlerHookHandler
): void {
  app.post('/api/v1/admin/log-filter-options', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const body = adminLogFilterOptionsSchema.parse(request.body ?? {});
      const bootstrap = await loadCurrentUserBootstrap(app, config, controlPlaneClient, request);
      const isAdmin = bootstrap.authorization.tokenRoles.some((role) => role.toLowerCase() === 'admin');
      if (!isAdmin && body.kind === 'user') {
        return reply.code(403).send({ error: 'admin_role_required' });
      }
      const options = await controlPlaneClient.searchAdminLogFilters({
        userSubject: resolveUserSubject(config, request),
        query: body.query,
        kind: body.kind,
        limit: body.limit,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
      return {
        options: options.map((option) => ({
          kind: option.kind,
          id: option.id,
          label: option.label,
          secondaryLabel: option.secondaryLabel,
          deviceIds: option.deviceIds
        }))
      };
    } catch (error) {
      if (error instanceof z.ZodError) {
        return reply.code(400).send({ error: 'invalid_request', issues: error.issues });
      }
      return handleGrpcRouteError(config, reply, error);
    }
  });
}
