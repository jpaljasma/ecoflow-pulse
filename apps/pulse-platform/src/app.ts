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
  const historyRateLimiter = buildHistoryRateLimiter(config);
  app.decorate('telemetryDeadlineMs', config.grpcDeadlineMs);
  app.get('/healthz', async () => ({ ok: true }));
  void app.register(async (scopedApp) => {
    registerHistoryRoutes(scopedApp, historyClient, authPreHandler, historyRateLimiter);
  });
  app.addHook('onClose', async () => {
    historyClient.close();
  });
  return app;
}

function buildHistoryRateLimiter(config: AppConfig): RateLimiterMemory {
  return new RateLimiterMemory({
    keyPrefix: 'pulse-platform:history',
    points: config.historyRateLimit.max,
    duration: Math.max(1, Math.ceil(config.historyRateLimit.timeWindowMs / 1000))
  });
}

declare module 'fastify' {
  interface FastifyInstance {
    telemetryDeadlineMs: number;
  }
}
