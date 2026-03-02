import fs from 'node:fs';
import path from 'node:path';

import Fastify, { type FastifyInstance, type preHandlerHookHandler } from 'fastify';
import cors from '@fastify/cors';
import fastifyStatic from '@fastify/static';
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
  if (config.publicDir && fs.existsSync(config.publicDir)) {
    void app.register(fastifyStatic, {
      root: config.publicDir,
      prefix: '/',
      index: false
    });
    app.get('/', async (request, reply) => {
      if (request.method !== 'GET') {
        return reply.code(405).send();
      }
      return reply.sendFile('index.html');
    });
    app.setNotFoundHandler(async (request, reply) => {
      const requestedPath = request.raw.url?.replace(/^\//, '').trim() ?? '';
      if (!requestedPath || requestedPath.startsWith('api/') || requestedPath === 'healthz' || requestedPath.startsWith('ws')) {
        return reply.code(404).send();
      }
      const filePath = path.join(config.publicDir!, requestedPath);
      if (fs.existsSync(filePath) && fs.statSync(filePath).isFile()) {
        return reply.sendFile(requestedPath);
      }
      return reply.type('text/html; charset=utf-8').send(fs.readFileSync(path.join(config.publicDir!, 'index.html'), 'utf8'));
    });
  }
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
