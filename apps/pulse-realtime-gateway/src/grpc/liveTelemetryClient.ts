import path from 'node:path';
import { fileURLToPath } from 'node:url';

import grpc from '@grpc/grpc-js';
import protoLoader from '@grpc/proto-loader';

export type LiveCursor = {
  seq: number;
  tsUnixMs: number;
};

export type LiveSnapshot = {
  deviceId: string;
  cursor: LiveCursor;
  metrics: Record<string, number>;
};

export type LiveDelta = {
  deviceId: string;
  cursor: LiveCursor;
  changed: Record<string, number>;
  cleared: string[];
};

export type LiveHeartbeat = {
  deviceId: string;
  cursor: LiveCursor;
};

export type SubscribeInput = {
  deviceId: string;
  includeInitialSnapshot: boolean;
  maxUpdateHz: number;
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
  onSnapshot: (snapshot: LiveSnapshot) => void;
  onDelta: (delta: LiveDelta) => void;
  onHeartbeat: (heartbeat: LiveHeartbeat) => void;
  onClose: (error?: Error & { code?: number }) => void;
};

export type LiveSubscription = {
  close: () => void;
};

export interface LiveTelemetryClient {
  subscribe(input: SubscribeInput): LiveSubscription;
  close(): void;
}

type GrpcTelemetryClient = {
  Subscribe: (
    request: Record<string, unknown>,
    metadata: grpc.Metadata,
    options: grpc.CallOptions
  ) => grpc.ClientReadableStream<unknown>;
  close: () => void;
};

type TelemetryProto = {
  pulse: {
    telemetry: {
      v1: {
        TelemetryService: new (
          address: string,
          credentials: grpc.ChannelCredentials,
          options?: Record<string, unknown>
        ) => GrpcTelemetryClient;
      };
    };
  };
};

type RawCursor = {
  seq?: unknown;
  tsUnixMs?: unknown;
};

type RawSnapshot = {
  deviceId?: unknown;
  cursor?: RawCursor;
  metrics?: Record<string, unknown>;
};

type RawDelta = {
  deviceId?: unknown;
  cursor?: RawCursor;
  changed?: Record<string, unknown>;
  cleared?: unknown;
};

type RawHeartbeat = {
  deviceId?: unknown;
  cursor?: RawCursor;
};

type RawSubscribeResponse = {
  payload?: unknown;
  snapshot?: RawSnapshot;
  delta?: RawDelta;
  heartbeat?: RawHeartbeat;
};

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, '../../../../');
const protoRoot = path.join(projectRoot, 'proto');
const telemetryProtoPath = path.join(protoRoot, 'pulse/telemetry/v1/telemetry.proto');

const packageDefinition = protoLoader.loadSync(telemetryProtoPath, {
  keepCase: false,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
  includeDirs: [protoRoot]
});
const telemetryProto = grpc.loadPackageDefinition(packageDefinition) as unknown as TelemetryProto;

export function createLiveTelemetryClient(address: string): LiveTelemetryClient {
  const client = new telemetryProto.pulse.telemetry.v1.TelemetryService(
    address,
    grpc.credentials.createInsecure()
  );

  return {
    subscribe(input) {
      const metadata = new grpc.Metadata();
      if (input.authHeader) {
        metadata.set('authorization', input.authHeader);
      }
      if (input.requestID) {
        metadata.set('x-request-id', input.requestID);
      }

      const stream = client.Subscribe(
        {
          deviceId: input.deviceId,
          includeInitialSnapshot: input.includeInitialSnapshot,
          maxUpdateHz: input.maxUpdateHz
        },
        metadata,
        { deadline: new Date(Date.now() + input.deadlineMs) }
      );

      let closed = false;
      let notified = false;
      const settle = (error?: Error & { code?: number }) => {
        if (closed || notified) {
          return;
        }
        notified = true;
        input.onClose(error);
      };

      stream.on('data', (message) => {
        const parsed = parseSubscribeResponse(message as RawSubscribeResponse);
        if (!parsed) {
          return;
        }
        switch (parsed.type) {
          case 'snapshot':
            input.onSnapshot(parsed.snapshot);
            break;
          case 'delta':
            input.onDelta(parsed.delta);
            break;
          case 'heartbeat':
            input.onHeartbeat(parsed.heartbeat);
            break;
        }
      });
      stream.on('error', (error) => {
        if (closed) {
          return;
        }
        settle(error as Error & { code?: number });
      });
      stream.on('end', () => {
        settle();
      });

      return {
        close() {
          if (closed) {
            return;
          }
          closed = true;
          stream.cancel();
        }
      };
    },
    close() {
      client.close();
    }
  };
}

function parseSubscribeResponse(message: RawSubscribeResponse):
  | { type: 'snapshot'; snapshot: LiveSnapshot }
  | { type: 'delta'; delta: LiveDelta }
  | { type: 'heartbeat'; heartbeat: LiveHeartbeat }
  | null {
  const payloadType = typeof message.payload === 'string' ? message.payload : '';
  if ((payloadType === 'snapshot' || message.snapshot) && message.snapshot) {
    return { type: 'snapshot', snapshot: normalizeSnapshot(message.snapshot) };
  }
  if ((payloadType === 'delta' || message.delta) && message.delta) {
    return { type: 'delta', delta: normalizeDelta(message.delta) };
  }
  if ((payloadType === 'heartbeat' || message.heartbeat) && message.heartbeat) {
    return { type: 'heartbeat', heartbeat: normalizeHeartbeat(message.heartbeat) };
  }
  return null;
}

function normalizeSnapshot(snapshot: RawSnapshot): LiveSnapshot {
  return {
    deviceId: normalizeString(snapshot.deviceId),
    cursor: normalizeCursor(snapshot.cursor),
    metrics: normalizeMetricMap(snapshot.metrics)
  };
}

function normalizeDelta(delta: RawDelta): LiveDelta {
  return {
    deviceId: normalizeString(delta.deviceId),
    cursor: normalizeCursor(delta.cursor),
    changed: normalizeMetricMap(delta.changed),
    cleared: Array.isArray(delta.cleared)
      ? delta.cleared.map((value) => normalizeString(value)).filter(Boolean)
      : []
  };
}

function normalizeHeartbeat(heartbeat: RawHeartbeat): LiveHeartbeat {
  return {
    deviceId: normalizeString(heartbeat.deviceId),
    cursor: normalizeCursor(heartbeat.cursor)
  };
}

function normalizeCursor(cursor: RawCursor | undefined): LiveCursor {
  return {
    seq: normalizeInt(cursor?.seq),
    tsUnixMs: normalizeInt(cursor?.tsUnixMs)
  };
}

function normalizeMetricMap(input: Record<string, unknown> | undefined): Record<string, number> {
  const output: Record<string, number> = {};
  if (!input) {
    return output;
  }
  for (const [key, value] of Object.entries(input)) {
    const normalized = normalizeNumber(value);
    if (normalized !== undefined) {
      output[key] = normalized;
    }
  }
  return output;
}

function normalizeString(value: unknown): string {
  if (typeof value === 'string') {
    return value;
  }
  if (typeof value === 'number' || typeof value === 'bigint') {
    return String(value);
  }
  return '';
}

function normalizeInt(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.trunc(value);
  }
  if (typeof value === 'bigint') {
    return Number(value);
  }
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number.parseInt(value, 10);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}

function normalizeNumber(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number.parseFloat(value);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  return undefined;
}
