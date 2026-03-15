import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { observeAuthSessionRecovery, type AuthSessionRecoveryOutcome } from '../metrics.js';

const AuthSessionEventSchema = z.object({
  outcome: z.enum(['recovered_in_memory', 'recovered_refresh', 'reauth_redirect'])
});

export function registerAuthSessionEventRoutes(app: FastifyInstance): void {
  app.post('/api/v1/auth/session-events', async (request, reply) => {
    const parsed = AuthSessionEventSchema.safeParse(request.body);
    if (!parsed.success) {
      return reply.code(400).send({
        error: 'invalid_auth_session_event',
        details: parsed.error.flatten()
      });
    }

    observeAuthSessionRecovery(parsed.data.outcome as AuthSessionRecoveryOutcome);
    return reply.code(202).send({ ok: true });
  });
}
