import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { loadConfig } from '../src/config.js';

describe('pulse-platform config', () => {
  it('defaults local BFF port to 18081 to avoid Expo web collisions', () => {
    const config = loadConfig({});

    expect(config.port).toBe(18081);
    expect(config.energyGrpcApiAddr).toBe('127.0.0.1:9090');
    expect(config.corsAllowedOrigins).toEqual([]);
    expect(config.dataPlane).toBe('local');
    expect(config.bffCache?.enabled).toBe(false);
    expect(config.bffCache?.weatherForecastTtlMs).toBe(30000);
  });

  it('supports optional noop dev subject override', () => {
    const config = loadConfig({
      PULSE_PLATFORM_DEV_SUBJECT: 'dev-user@example.com'
    });

    expect(config.devUserSubject).toBe('dev-user@example.com');
  });

  it('parses optional public preconnect origins', () => {
    const config = loadConfig({
      PULSE_PLATFORM_PUBLIC_PRECONNECT_ORIGINS: 'https://api.example.com,wss://ws.example.com'
    });

    expect(config.publicPreconnectOrigins).toEqual([
      'https://api.example.com',
      'wss://ws.example.com'
    ]);
  });

  it('parses optional cors allowed origins', () => {
    const config = loadConfig({
      PULSE_PLATFORM_CORS_ALLOWED_ORIGINS: 'http://localhost:8081,https://localhost:8081'
    });

    expect(config.corsAllowedOrigins).toEqual([
      'http://localhost:8081',
      'https://localhost:8081'
    ]);
  });

  it('keeps hosted cloud public-app CORS aligned with the shared localhost edge', () => {
    const testDir = path.dirname(fileURLToPath(import.meta.url));
    const valuesPath = path.resolve(testDir, '../../../deploy/env/cloud/values.platform.yaml');
    const values = readFileSync(valuesPath, 'utf8');
    const match = values.match(/corsAllowedOrigins:\s*"([^"]+)"/);

    expect(match?.[1]?.split(',')).toContain('https://localhost');
  });

  it('keeps hosted cloud grpc upstreams aligned between public app and realtime gateway', () => {
    const testDir = path.dirname(fileURLToPath(import.meta.url));
    const valuesPath = path.resolve(testDir, '../../../deploy/env/cloud/values.platform.yaml');
    const values = readFileSync(valuesPath, 'utf8');
    const grpcMatches = [...values.matchAll(/grpcApiAddr:\s*([^\s]+)/g)].map((match) => match[1]);

    expect(grpcMatches.length).toBeGreaterThanOrEqual(2);
    expect(grpcMatches[0]).toBe('pulse-services-cloud-go-grpc-api.pulse-services.svc.cluster.local:9090');
    expect(grpcMatches[1]).toBe('pulse-services-cloud-go-grpc-api.pulse-services.svc.cluster.local:9090');
  });

  it('supports a dedicated energy grpc upstream override', () => {
    const config = loadConfig({
      GRPC_API_ADDR: '127.0.0.1:9090',
      ENERGY_GRPC_API_ADDR: '127.0.0.1:9191'
    });

    expect(config.grpcApiAddr).toBe('127.0.0.1:9090');
    expect(config.energyGrpcApiAddr).toBe('127.0.0.1:9191');
  });

  it('parses optional BFF cache settings', () => {
    const config = loadConfig({
      PULSE_PLATFORM_DATA_PLANE: 'cloud',
      PULSE_PLATFORM_BFF_CACHE_ENABLED: 'true',
      PULSE_PLATFORM_BFF_CACHE_MAX_ENTRIES: '2500',
      PULSE_PLATFORM_WEATHER_FORECAST_CACHE_TTL_MS: '45000'
    });

    expect(config.dataPlane).toBe('cloud');
    expect(config.bffCache).toEqual({
      enabled: true,
      maxEntries: 2500,
      weatherForecastTtlMs: 45000
    });
  });
});
