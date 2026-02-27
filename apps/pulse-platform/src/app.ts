import Fastify, { type FastifyInstance, type preHandlerHookHandler } from 'fastify';
import { RateLimiterMemory } from 'rate-limiter-flexible';

import { buildAuthPreHandler } from './auth.js';
import type { AppConfig } from './config.js';
import type { TelemetryHistoryClient } from './grpc/telemetryClient.js';
import { registerHistoryRoutes } from './routes/history.js';

type BuildAppOptions = {
  authPreHandler?: preHandlerHookHandler;
};

export function buildApp(
  config: AppConfig,
  historyClient: TelemetryHistoryClient,
  options: BuildAppOptions = {}
): FastifyInstance {
  const app = Fastify({ logger: false });
  const authPreHandler = options.authPreHandler ?? buildAuthPreHandler(config);
  const historyRateLimitPreHandler = buildHistoryRateLimitPreHandler(config);
  app.decorate('telemetryDeadlineMs', config.grpcDeadlineMs);
  app.get('/healthz', async () => ({ ok: true }));
  void app.register(async (scopedApp) => {
    registerHistoryRoutes(scopedApp, historyClient, authPreHandler, historyRateLimitPreHandler);
  });
  app.addHook('onClose', async () => {
    historyClient.close();
  });
  return app;
}

function buildHistoryRateLimitPreHandler(config: AppConfig): preHandlerHookHandler {
  const limiter = new RateLimiterMemory({
    keyPrefix: 'pulse-platform:history',
    points: config.historyRateLimit.max,
    duration: Math.max(1, Math.ceil(config.historyRateLimit.timeWindowMs / 1000))
  });
  return async (request, reply) => {
    try {
      await limiter.consume(request.ip);
    } catch (error) {
      const retryAfterSeconds = getRetryAfterSeconds(error);
      if (retryAfterSeconds !== undefined) {
        void reply.header('retry-after', String(retryAfterSeconds));
      }
      void reply.code(429).send({ error: 'rate_limited' });
    }
  };
}

function getRetryAfterSeconds(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null || !('msBeforeNext' in error)) {
    return undefined;
  }
  const msBeforeNext = Number(error.msBeforeNext);
  if (!Number.isFinite(msBeforeNext) || msBeforeNext <= 0) {
    return undefined;
  }
  return Math.max(1, Math.ceil(msBeforeNext / 1000));
}

declare module 'fastify' {
  interface FastifyInstance {
    telemetryDeadlineMs: number;
  }
}
