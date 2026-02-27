import Fastify, { type FastifyInstance, type preHandlerHookHandler } from 'fastify';

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
  app.decorate('telemetryDeadlineMs', config.grpcDeadlineMs);
  app.get('/healthz', async () => ({ ok: true }));
  app.addHook('preHandler', options.authPreHandler ?? buildAuthPreHandler(config));
  registerHistoryRoutes(app, historyClient);
  app.addHook('onClose', async () => {
    historyClient.close();
  });
  return app;
}
