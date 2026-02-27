import Fastify, { type FastifyInstance, type preHandlerHookHandler } from 'fastify';
import rateLimit, { type RateLimitOptions } from 'fastify-rate-limit';

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
  const historyRateLimit = buildHistoryRateLimit(config);
  app.decorate('telemetryDeadlineMs', config.grpcDeadlineMs);
  app.decorate('historyRateLimit', historyRateLimit);
  app.get('/healthz', async () => ({ ok: true }));
  void app.register(async (scopedApp) => {
    await scopedApp.register(rateLimit, {
      global: false,
      hook: 'preHandler'
    });
    registerHistoryRoutes(scopedApp, historyClient, authPreHandler);
  });
  app.addHook('onClose', async () => {
    historyClient.close();
  });
  return app;
}

function buildHistoryRateLimit(config: AppConfig): RateLimitOptions {
  return {
    max: config.historyRateLimit.max,
    timeWindow: Math.max(1, Math.ceil(config.historyRateLimit.timeWindowMs / 1000)),
    cache: 10_000
  };
}

declare module 'fastify' {
  interface FastifyInstance {
    historyRateLimit: RateLimitOptions;
    telemetryDeadlineMs: number;
  }
}
