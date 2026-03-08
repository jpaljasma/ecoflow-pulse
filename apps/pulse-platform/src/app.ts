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
import {
  buildHtmlDeliveryPlan,
  buildStaticHeaderPlan,
  type HtmlDeliveryPlan
} from './httpDelivery.js';
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
    const indexHtmlPath = path.join(config.publicDir, 'index.html');
    const indexHtml = fs.readFileSync(indexHtmlPath, 'utf8');
    const htmlDeliveryPlan = buildHtmlDeliveryPlan(indexHtml, config.publicPreconnectOrigins);
    app.addHook('onSend', async (request, reply, payload) => {
      const contentType = String(reply.getHeader('content-type') ?? '');
      if (request.method === 'GET' && contentType.startsWith('text/html')) {
        applyHtmlDeliveryHeaders(reply, htmlDeliveryPlan);
      }
      return payload;
    });
    void app.register(fastifyStatic, {
      root: config.publicDir,
      prefix: '/',
      index: false,
      cacheControl: false,
      setHeaders: (res, filePath) => {
        const plan = buildStaticHeaderPlan(config.publicDir!, filePath);
        if (plan.cacheControl) {
          res.setHeader('Cache-Control', plan.cacheControl);
        }
      }
    });
    app.get('/', async (request, reply) => {
      applyHtmlDeliveryHeaders(reply, htmlDeliveryPlan);
      return reply.type('text/html; charset=utf-8').send(indexHtml);
    });
    app.head('/', async (_request, reply) => {
      applyHtmlDeliveryHeaders(reply, htmlDeliveryPlan);
      return reply.code(200).send();
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
      applyHtmlDeliveryHeaders(reply, htmlDeliveryPlan);
      return reply.type('text/html; charset=utf-8').send(indexHtml);
    });
  }
  app.addHook('onClose', async () => {
    historyClient.close();
    deviceClient.close();
  });
  return app;
}

function applyHtmlDeliveryHeaders(
  reply: { header: (name: string, value: string) => unknown; raw: { writeEarlyHints?: (hints: Record<string, string | string[]>) => void } },
  plan: HtmlDeliveryPlan
): void {
  reply.header('Cache-Control', plan.cacheControl);
  if (plan.linkHeaderValues.length > 0) {
    reply.header('Link', plan.linkHeaderValues.join(', '));
    if (typeof reply.raw.writeEarlyHints === 'function') {
      reply.raw.writeEarlyHints({
        Link: plan.linkHeaderValues
      });
    }
  }
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
