import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { observeClientRestRequest } from '../metrics.js';

const CanonicalClientRestRouteSchema = z.enum([
  '/api/devices',
  '/api/devices/:deviceId',
  '/api/v1/me',
  '/api/v1/me/identity-refresh',
  '/api/v1/integrations',
  '/api/v1/integrations/:credentialId',
  '/api/v1/integrations/:credentialId/active',
  '/api/v1/energy/dashboard',
  '/api/v1/energy/pv-history',
  '/api/v1/energy/comparison-insight',
  '/api/v1/solar/outlook',
  '/api/v1/devices',
  '/api/v1/devices/:deviceId',
  '/api/v1/devices/available',
  '/api/v1/devices/available/test-mqtt',
  '/api/v1/devices/available/enable',
  '/api/v1/devices/:deviceId/history',
  '/api/v1/devices/:deviceId/history/compare',
  '/api/v1/devices/:deviceId/history/solar',
  '/api/v1/devices/:deviceId/insights',
  '/api/other'
]);

const ClientRestMetricSchema = z.object({
  route: CanonicalClientRestRouteSchema,
  method: z.enum(['GET', 'POST', 'PUT', 'PATCH', 'DELETE']),
  outcome: z.enum(['success', 'http_error', 'network_error', 'client_error']),
  statusClass: z.enum(['none', '2xx', '3xx', '4xx', '5xx']),
  durationMs: z.number().finite().min(0).max(600_000),
  errorKind: z.string().min(1).max(64)
});

export function registerClientMetricsRoutes(app: FastifyInstance): void {
  app.post('/api/v1/client-metrics/rest', async (request, reply) => {
    const parsed = ClientRestMetricSchema.safeParse(request.body);
    if (!parsed.success) {
      return reply.code(400).send({
        error: 'invalid_client_rest_metric',
        details: parsed.error.flatten()
      });
    }

    observeClientRestRequest({
      route: parsed.data.route,
      method: parsed.data.method,
      outcome: parsed.data.outcome,
      statusClass: parsed.data.statusClass,
      durationSeconds: parsed.data.durationMs / 1000,
      errorKind: parsed.data.errorKind
    });
    return reply.code(202).send({ ok: true });
  });
}
