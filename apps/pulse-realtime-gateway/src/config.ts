import { z } from 'zod';

import { preferredLocalValkeyAddrs } from './snapshot/valkeySnapshotStore.js';

const envSchema = z.object({
  PULSE_REALTIME_GATEWAY_HOST: z.string().trim().min(1).default('0.0.0.0'),
  PULSE_REALTIME_GATEWAY_PORT: z.coerce.number().int().min(1).max(65535).default(8082),
  GRPC_API_ADDR: z.string().trim().min(1).default('127.0.0.1:9090'),
  GRPC_API_DEADLINE_MS: z.coerce.number().int().min(100).max(60000).default(10000),
  GRPC_RECONNECT_BASE_MS: z.coerce.number().int().min(10).max(10000).default(250),
  GRPC_RECONNECT_MAX_MS: z.coerce.number().int().min(10).max(60000).default(2000),
  NATS_URLS: z.string().trim().default('nats://127.0.0.1:4222'),
  VALKEY_ADDRS: z.string().trim().default('127.0.0.1:6379'),
  VALKEY_SENTINEL_MASTER_SET: z.string().trim().default(''),
  VALKEY_SENTINEL_USERNAME: z.string().trim().default(''),
  VALKEY_SENTINEL_PASSWORD: z.string().trim().default(''),
  VALKEY_USERNAME: z.string().trim().default(''),
  VALKEY_PASSWORD: z.string().trim().default(''),
  PROJECTION_KEY_PREFIX: z.string().trim().min(1).default('pulse:projection'),
  TELEMETRY_SUBJECT_PREFIX: z.string().trim().min(1).default('pulse.telemetry'),
  LOGS_NATS_JS_STREAM_NAME: z.string().trim().min(1).default('PULSE_TELEMETRY_INGEST'),
  LOGS_REPLAY_LIMIT: z.coerce.number().int().min(1).max(200).default(200),
  LOGS_REPLAY_WINDOW_MS: z.coerce.number().int().min(1000).max(3600000).default(300000),
  LOGS_DEV_ADMIN_ENABLED: z
    .union([z.literal('true'), z.literal('false'), z.literal('1'), z.literal('0'), z.literal('')])
    .default(''),
  WS_DELIVERY_FAST_INTERVAL_MS: z.coerce.number().int().min(50).max(5000).default(250),
  WS_DELIVERY_STEADY_INTERVAL_MS: z.coerce.number().int().min(50).max(5000).default(500),
  WS_DELIVERY_SLOW_INTERVAL_MS: z.coerce.number().int().min(100).max(10000).default(1000),
  WS_DELIVERY_HIGH_WATERMARK: z.coerce.number().int().min(1).max(1000).default(8),
  WS_BUFFERED_AMOUNT_HIGH_WATER_BYTES: z.coerce.number().int().min(1024).max(16777216).default(262144),
  WS_QUIET_TICKS_TO_RECOVER: z.coerce.number().int().min(1).max(100).default(4),
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
  reconnectBackoff: {
    baseMs: number;
    maxMs: number;
  };
  natsUrls: string[];
  valkey: {
    addrs: string[];
    sentinelMasterSet?: string;
    sentinelUsername?: string;
    sentinelPassword?: string;
    username?: string;
    password?: string;
    keyPrefix: string;
  };
  telemetrySubjectPrefix: string;
  logs: {
    streamName: string;
    replayLimit: number;
    replayWindowMs: number;
    devAdminEnabled: boolean;
  };
  delivery: {
    fastIntervalMs: number;
    steadyIntervalMs: number;
    slowIntervalMs: number;
    highWatermark: number;
    bufferedAmountHighWaterBytes: number;
    quietTicksToRecover: number;
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
  const natsUrls = splitCsvList(parsed.NATS_URLS, ['nats://127.0.0.1:4222']);
  const sentinelMasterSet = parsed.VALKEY_SENTINEL_MASTER_SET || undefined;
  const valkeyAddrs = sentinelMasterSet
    ? splitCsvList(parsed.VALKEY_ADDRS, ['127.0.0.1:26379'])
    : preferredLocalValkeyAddrs(splitCsvList(parsed.VALKEY_ADDRS, ['127.0.0.1:6379']));
  const delivery = {
    fastIntervalMs: parsed.WS_DELIVERY_FAST_INTERVAL_MS,
    steadyIntervalMs: parsed.WS_DELIVERY_STEADY_INTERVAL_MS,
    slowIntervalMs: parsed.WS_DELIVERY_SLOW_INTERVAL_MS,
    highWatermark: parsed.WS_DELIVERY_HIGH_WATERMARK,
    bufferedAmountHighWaterBytes: parsed.WS_BUFFERED_AMOUNT_HIGH_WATER_BYTES,
    quietTicksToRecover: parsed.WS_QUIET_TICKS_TO_RECOVER
  };
  const logs = {
    streamName: parsed.LOGS_NATS_JS_STREAM_NAME,
    replayLimit: parsed.LOGS_REPLAY_LIMIT,
    replayWindowMs: parsed.LOGS_REPLAY_WINDOW_MS,
    devAdminEnabled:
      parsed.LOGS_DEV_ADMIN_ENABLED === 'true' || parsed.LOGS_DEV_ADMIN_ENABLED === '1'
  };

  if (parsed.NODE_AUTH_MODE === 'noop') {
    return {
      host: parsed.PULSE_REALTIME_GATEWAY_HOST,
      port: parsed.PULSE_REALTIME_GATEWAY_PORT,
      grpcApiAddr: parsed.GRPC_API_ADDR,
      grpcDeadlineMs: parsed.GRPC_API_DEADLINE_MS,
      reconnectBackoff: {
        baseMs: parsed.GRPC_RECONNECT_BASE_MS,
        maxMs: parsed.GRPC_RECONNECT_MAX_MS
      },
      natsUrls,
      valkey: {
        addrs: valkeyAddrs,
        sentinelMasterSet,
        sentinelUsername: parsed.VALKEY_SENTINEL_USERNAME || undefined,
        sentinelPassword: parsed.VALKEY_SENTINEL_PASSWORD || undefined,
        username: parsed.VALKEY_USERNAME || undefined,
        password: parsed.VALKEY_PASSWORD || undefined,
        keyPrefix: parsed.PROJECTION_KEY_PREFIX
      },
      telemetrySubjectPrefix: parsed.TELEMETRY_SUBJECT_PREFIX,
      logs,
      delivery,
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
    host: parsed.PULSE_REALTIME_GATEWAY_HOST,
    port: parsed.PULSE_REALTIME_GATEWAY_PORT,
    grpcApiAddr: parsed.GRPC_API_ADDR,
    grpcDeadlineMs: parsed.GRPC_API_DEADLINE_MS,
    reconnectBackoff: {
      baseMs: parsed.GRPC_RECONNECT_BASE_MS,
      maxMs: parsed.GRPC_RECONNECT_MAX_MS
    },
    natsUrls,
    valkey: {
      addrs: valkeyAddrs,
      sentinelMasterSet,
      sentinelUsername: parsed.VALKEY_SENTINEL_USERNAME || undefined,
      sentinelPassword: parsed.VALKEY_SENTINEL_PASSWORD || undefined,
      username: parsed.VALKEY_USERNAME || undefined,
      password: parsed.VALKEY_PASSWORD || undefined,
      keyPrefix: parsed.PROJECTION_KEY_PREFIX
    },
    telemetrySubjectPrefix: parsed.TELEMETRY_SUBJECT_PREFIX,
    logs,
    delivery,
    auth: {
      mode: 'keycloak',
      issuerUrl: parsed.KEYCLOAK_ISSUER_URL,
      audience: parsed.KEYCLOAK_AUDIENCE,
      jwksUrl: parsed.KEYCLOAK_JWKS_URL || undefined,
      allowMissingJwt
    }
  };
}

function splitCsvList(input: string, fallback: string[]): string[] {
  const values = input
    .split(/[,\s]+/)
    .map((value) => value.trim())
    .filter(Boolean);
  return values.length > 0 ? values : fallback;
}
