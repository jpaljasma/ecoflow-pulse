import fs from 'node:fs';
import path from 'node:path';

import Fastify, { type FastifyInstance, type preHandlerHookHandler } from 'fastify';
import cors from '@fastify/cors';
import fastifyStatic from '@fastify/static';
import rateLimit, { type RateLimitOptions } from 'fastify-rate-limit';

import { buildAuthPreHandler } from './auth.js';
import { createBffCache } from './cache/bffCache.js';
import type { AppConfig } from './config.js';
import type { ControlPlaneClient } from './grpc/controlPlaneClient.js';
import type { DeviceClient } from './grpc/deviceClient.js';
import type { InferenceClient } from './grpc/inferenceClient.js';
import type { SolarForecastClient } from './grpc/solarForecastClient.js';
import type { TelemetryHistoryClient } from './grpc/telemetryClient.js';
import type { WeatherClient } from './grpc/weatherClient.js';
import {
  buildHtmlDeliveryPlan,
  buildStaticHeaderPlan,
  type HtmlDeliveryPlan
} from './httpDelivery.js';
import {
  observeBffCacheOperation,
  observePublicRequest,
  publicMetricsContentType,
  renderPublicMetrics
} from './metrics.js';
import { registerDeviceRoutes } from './routes/devices.js';
import { registerAdminLogRoutes } from './routes/adminLogs.js';
import { registerAuthSessionEventRoutes } from './routes/authSessionEvents.js';
import { registerClientMetricsRoutes } from './routes/clientMetrics.js';
import { registerClientWsMetricsRoutes } from './routes/clientWsMetrics.js';
import { registerHistoryRoutes } from './routes/history.js';
import { registerIntegrationRoutes } from './routes/integrations.js';
import { registerCurrentUserRoutes } from './routes/me.js';
import { registerSolarRoutes } from './routes/solar.js';
import { registerWeatherRoutes } from './routes/weather.js';

type BuildAppOptions = {
  authPreHandler?: preHandlerHookHandler;
  controlPlaneClient?: ControlPlaneClient;
  weatherClient?: WeatherClient;
  solarForecastClient?: SolarForecastClient;
};

export function buildApp(
  config: AppConfig,
  historyClient: TelemetryHistoryClient,
  deviceClient: DeviceClient,
  inferenceClient: InferenceClient,
  options: BuildAppOptions = {}
): FastifyInstance {
  const app = Fastify({ logger: false });
  const authPreHandler = options.authPreHandler ?? buildAuthPreHandler(config);
  const historyRateLimit = buildHistoryRateLimit(config);
  const bffCacheConfig = config.bffCache ?? {
    enabled: false,
    maxEntries: 1000,
    weatherForecastTtlMs: 0,
    weatherYesterdayTtlMs: 0
  };
  const bffCache = createBffCache({
    enabled: bffCacheConfig.enabled,
    maxEntries: bffCacheConfig.maxEntries,
    observe: observeBffCacheOperation
  });
  app.decorate('telemetryDeadlineMs', config.grpcDeadlineMs);
  app.decorate('historyRateLimit', historyRateLimit);
  app.get('/healthz', async () => ({ ok: true }));
  app.get('/metrics', async (_request, reply) => {
    reply.header('Content-Type', publicMetricsContentType());
    return await renderPublicMetrics();
  });
  app.addHook('onRequest', async (request) => {
    request.metricsStartedAt = process.hrtime.bigint();
  });
  app.addHook('onResponse', async (request, reply) => {
    const startedAt = request.metricsStartedAt;
    if (!startedAt) {
      return;
    }
    const pathname = new URL(request.raw.url ?? '/', 'http://localhost').pathname;
    const durationSeconds = Number(process.hrtime.bigint() - startedAt) / 1_000_000_000;
    observePublicRequest({
      pathname,
      method: request.method,
      statusCode: reply.statusCode,
      durationSeconds
    });
  });
  void app.register(async (scopedApp) => {
    await scopedApp.register(cors, {
      origin: (origin, callback) => {
        callback(null, isAllowedCorsOrigin(origin, config.corsAllowedOrigins));
      },
      credentials: true
    });
    await scopedApp.register(rateLimit, {
      global: false,
      hook: 'preHandler'
    });
    registerAuthSessionEventRoutes(scopedApp);
    registerClientMetricsRoutes(scopedApp);
    registerClientWsMetricsRoutes(scopedApp);
    if (options.controlPlaneClient) {
      registerCurrentUserRoutes(scopedApp, config, options.controlPlaneClient, authPreHandler);
      registerIntegrationRoutes(scopedApp, config, options.controlPlaneClient, authPreHandler);
      registerAdminLogRoutes(scopedApp, config, options.controlPlaneClient, authPreHandler);
    }
    if (options.controlPlaneClient && options.weatherClient) {
      registerWeatherRoutes(
        scopedApp,
        { ...config, bffCache: bffCacheConfig },
        options.controlPlaneClient,
        options.weatherClient,
        authPreHandler,
        bffCache
      );
    }
    if (options.controlPlaneClient && options.solarForecastClient) {
      registerSolarRoutes(
        scopedApp,
        config,
        options.controlPlaneClient,
        options.solarForecastClient,
        authPreHandler
      );
    }
    registerDeviceRoutes(scopedApp, config, deviceClient, inferenceClient, authPreHandler);
    registerHistoryRoutes(scopedApp, config, historyClient, inferenceClient, authPreHandler, options.controlPlaneClient);
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
    bffCache.clear();
    options.controlPlaneClient?.close();
    options.weatherClient?.close();
    options.solarForecastClient?.close();
    historyClient.close();
    deviceClient.close();
    inferenceClient.close();
  });
  return app;
}

function isAllowedCorsOrigin(origin: string | undefined, allowedOrigins: string[]): boolean {
  if (!origin) {
    return true;
  }
  if (allowedOrigins.length === 0) {
    return true;
  }
  return allowedOrigins.includes(origin);
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
  interface FastifyRequest {
    metricsStartedAt?: bigint;
  }

  interface FastifyInstance {
    historyRateLimit: RateLimitOptions;
    telemetryDeadlineMs: number;
  }
}
