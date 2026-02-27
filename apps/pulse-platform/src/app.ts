import Fastify, { type FastifyInstance, type preHandlerHookHandler } from 'fastify';
import rateLimit from '@fastify/rate-limit';

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
  app.decorate('telemetryDeadlineMs', config.grpcDeadlineMs);
  app.decorate('historyRateLimit', {
    max: config.historyRateLimit.max,
    timeWindow: config.historyRateLimit.timeWindowMs
  });
  app.get('/healthz', async () => ({ ok: true }));
  void app.register(async (scopedApp) => {
    await scopedApp.register(rateLimit, {
      global: false,
      addHeadersOnExceeding: {
        'x-ratelimit-limit': false,
        'x-ratelimit-remaining': false,
        'x-ratelimit-reset': false
      },
      addHeaders: {
        'x-ratelimit-limit': true,
        'x-ratelimit-remaining': true,
        'x-ratelimit-reset': true,
        'retry-after': true
      }
    });
    registerHistoryRoutes(scopedApp, historyClient, authPreHandler);
  });
  app.addHook('onClose', async () => {
    historyClient.close();
  });
  return app;
}

declare module 'fastify' {
  interface FastifyInstance {
    telemetryDeadlineMs: number;
    historyRateLimit: {
      max: number;
      timeWindow: number;
    };
  }
}
