import Fastify, { type FastifyInstance, type preHandlerHookHandler } from 'fastify';
import cors from '@fastify/cors';
import rateLimit, { type RateLimitOptions } from 'fastify-rate-limit';

import { buildAuthPreHandler } from './auth.js';
import type { AppConfig } from './config.js';
import type { DeviceClient } from './grpc/deviceClient.js';
import type { TelemetryHistoryClient } from './grpc/telemetryClient.js';
import { registerDeviceRoutes } from './routes/devices.js';
import { registerHistoryRoutes } from './routes/history.js';

type BuildAppOptions = {
  authPreHandler?: preHandlerHookHandler;
};

export function buildApp(
  config: AppConfig,
  historyClient: TelemetryHistoryClient,
  deviceClient: DeviceClient,
  options: BuildAppOptions = {}
): FastifyInstance {
  const app = Fastify({ logger: false });
  const authPreHandler = options.authPreHandler ?? buildAuthPreHandler(config);
  const historyRateLimit = buildHistoryRateLimit(config);
  app.decorate('telemetryDeadlineMs', config.grpcDeadlineMs);
  app.decorate('historyRateLimit', historyRateLimit);
  app.get('/healthz', async () => ({ ok: true }));
  void app.register(async (scopedApp) => {
    await scopedApp.register(cors, {
      origin: true,
      credentials: true
    });
    await scopedApp.register(rateLimit, {
      global: false,
      hook: 'preHandler'
    });
    registerDeviceRoutes(scopedApp, config, deviceClient, authPreHandler);
    registerHistoryRoutes(scopedApp, historyClient, authPreHandler);
  });
  app.addHook('onClose', async () => {
    historyClient.close();
    deviceClient.close();
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
