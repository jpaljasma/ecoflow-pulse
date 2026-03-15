import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  observeClientWsConnection,
  observeClientWsDisconnect,
  observeClientWsFreshnessTransition,
  observeClientWsStaleRecoveryDuration
} from '../metrics.js';

const ClientWsMetricSchema = z.discriminatedUnion('eventType', [
  z.object({
    eventType: z.literal('connection'),
    phase: z.enum(['initial', 'reconnect']),
    outcome: z.enum(['connected', 'auth_required', 'connect_error', 'closed_before_open']),
    durationMs: z.number().finite().min(0).max(600_000)
  }),
  z.object({
    eventType: z.literal('disconnect'),
    reason: z.enum(['unexpected_close', 'socket_error', 'stalled', 'manual_disconnect', 'auth_required'])
  }),
  z.object({
    eventType: z.literal('freshness_transition'),
    state: z.enum(['stale', 'fresh'])
  }),
  z.object({
    eventType: z.literal('stale_recovery'),
    durationMs: z.number().finite().min(0).max(600_000)
  })
]);

export function registerClientWsMetricsRoutes(app: FastifyInstance): void {
  app.post('/api/v1/client-metrics/ws', async (request, reply) => {
    const parsed = ClientWsMetricSchema.safeParse(request.body);
    if (!parsed.success) {
      return reply.code(400).send({
        error: 'invalid_client_ws_metric',
        details: parsed.error.flatten()
      });
    }

    switch (parsed.data.eventType) {
      case 'connection':
        observeClientWsConnection({
          phase: parsed.data.phase,
          outcome: parsed.data.outcome,
          durationSeconds: parsed.data.durationMs / 1000
        });
        break;
      case 'disconnect':
        observeClientWsDisconnect(parsed.data.reason);
        break;
      case 'freshness_transition':
        observeClientWsFreshnessTransition(parsed.data.state);
        break;
      case 'stale_recovery':
        observeClientWsStaleRecoveryDuration(parsed.data.durationMs / 1000);
        break;
    }

    return reply.code(202).send({ ok: true });
  });
}
