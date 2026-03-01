import { z } from 'zod';

const envSchema = z.object({
  PULSE_PLATFORM_HOST: z.string().trim().min(1).default('0.0.0.0'),
  PULSE_PLATFORM_PORT: z.coerce.number().int().min(1).max(65535).default(18081),
  GRPC_API_ADDR: z.string().trim().min(1).default('127.0.0.1:9090'),
  GRPC_API_DEADLINE_MS: z.coerce.number().int().min(100).max(60000).default(10000),
  PULSE_PLATFORM_HISTORY_RATE_LIMIT_MAX: z.coerce.number().int().min(1).max(10000).default(120),
  PULSE_PLATFORM_HISTORY_RATE_LIMIT_WINDOW_MS: z.coerce.number().int().min(1000).max(3600000).default(60000),
  NODE_AUTH_MODE: z.enum(['noop', 'keycloak']).default('noop'),
  KEYCLOAK_ISSUER_URL: z.string().trim().default(''),
  KEYCLOAK_AUDIENCE: z.string().trim().default(''),
  KEYCLOAK_JWKS_URL: z.string().trim().default(''),
  KEYCLOAK_ALLOW_MISSING_JWT: z
    .union([z.literal('true'), z.literal('false'), z.literal('1'), z.literal('0'), z.literal('')])
    .default('')
});

export type AppConfig = {
  host: string;
  port: number;
  grpcApiAddr: string;
  grpcDeadlineMs: number;
  historyRateLimit: {
    max: number;
    timeWindowMs: number;
  };
  auth:
    | { mode: 'noop'; allowMissingJwt: true }
    | {
        mode: 'keycloak';
        issuerUrl: string;
        audience: string;
        jwksUrl?: string;
        allowMissingJwt: boolean;
      };
};

export function loadConfig(env: NodeJS.ProcessEnv): AppConfig {
  const parsed = envSchema.parse(env);
  const allowMissingJwt =
    parsed.KEYCLOAK_ALLOW_MISSING_JWT === 'true' || parsed.KEYCLOAK_ALLOW_MISSING_JWT === '1';
  if (parsed.NODE_AUTH_MODE === 'noop') {
    return {
      host: parsed.PULSE_PLATFORM_HOST,
      port: parsed.PULSE_PLATFORM_PORT,
      grpcApiAddr: parsed.GRPC_API_ADDR,
      grpcDeadlineMs: parsed.GRPC_API_DEADLINE_MS,
      historyRateLimit: {
        max: parsed.PULSE_PLATFORM_HISTORY_RATE_LIMIT_MAX,
        timeWindowMs: parsed.PULSE_PLATFORM_HISTORY_RATE_LIMIT_WINDOW_MS
      },
      auth: { mode: 'noop', allowMissingJwt: true }
    };
  }
  if (!parsed.KEYCLOAK_ISSUER_URL) {
    throw new Error('KEYCLOAK_ISSUER_URL is required when NODE_AUTH_MODE=keycloak');
  }
  if (!parsed.KEYCLOAK_AUDIENCE) {
    throw new Error('KEYCLOAK_AUDIENCE is required when NODE_AUTH_MODE=keycloak');
  }
  return {
    host: parsed.PULSE_PLATFORM_HOST,
    port: parsed.PULSE_PLATFORM_PORT,
    grpcApiAddr: parsed.GRPC_API_ADDR,
    grpcDeadlineMs: parsed.GRPC_API_DEADLINE_MS,
    historyRateLimit: {
      max: parsed.PULSE_PLATFORM_HISTORY_RATE_LIMIT_MAX,
      timeWindowMs: parsed.PULSE_PLATFORM_HISTORY_RATE_LIMIT_WINDOW_MS
    },
    auth: {
      mode: 'keycloak',
      issuerUrl: parsed.KEYCLOAK_ISSUER_URL,
      audience: parsed.KEYCLOAK_AUDIENCE,
      jwksUrl: parsed.KEYCLOAK_JWKS_URL || undefined,
      allowMissingJwt
    }
  };
}
