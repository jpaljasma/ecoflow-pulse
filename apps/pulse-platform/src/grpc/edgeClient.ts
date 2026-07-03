import path from 'node:path';
import { fileURLToPath } from 'node:url';

import grpc from '@grpc/grpc-js';
import protoLoader from '@grpc/proto-loader';

export type EdgeCollector = {
  id: string;
  displayName: string;
  isActive: boolean;
  lastHeartbeatAtUnixMs: string;
  createdAtUnixMs: string;
  updatedAtUnixMs: string;
  collectorVersion: string;
  hostname: string;
};

export const edgeDeviceSourceStatuses = ['pending', 'linked', 'ignored'] as const;
export type EdgeDeviceSourceStatus = (typeof edgeDeviceSourceStatuses)[number];

export type EdgeDeviceSource = {
  id: string;
  collectorId: string;
  provider: string;
  transport: string;
  providerDeviceId: string;
  displayName: string;
  model: string;
  status: EdgeDeviceSourceStatus;
  linkedDeviceId: string;
  rssiDbm: number;
  lastSeenAtUnixMs: string;
  createdAtUnixMs: string;
  updatedAtUnixMs: string;
  metadata?: Record<string, unknown>;
};

type RequestContext = {
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
};

export type CreateCollectorInput = RequestContext & {
  userSubject: string;
  displayName?: string;
};

export type ListCollectorsInput = RequestContext & {
  userSubject: string;
};

export type EnrollCollectorInput = RequestContext & {
  setupToken: string;
  collectorVersion?: string;
  hostname?: string;
};

export type CollectorSecretInput = RequestContext & {
  collectorSecret: string;
  collectorVersion?: string;
  hostname?: string;
};

export type EdgeDiscoveryInput = {
  provider: string;
  transport: string;
  providerDeviceId: string;
  displayName?: string;
  model?: string;
  address?: string;
  rssiDbm?: number;
  observedAtUnixMs?: number;
  metadata?: Record<string, unknown>;
};

export type UploadDiscoveryInput = RequestContext & {
  collectorSecret: string;
  discoveries: EdgeDiscoveryInput[];
};

export type ListDeviceSourcesInput = RequestContext & {
  userSubject: string;
  collectorId?: string;
  status?: EdgeDeviceSourceStatus;
};

export type ApproveDeviceSourceInput = RequestContext & {
  userSubject: string;
  sourceId: string;
  deviceId?: string;
  productName?: string;
  model?: string;
};

export type EdgeTelemetrySampleInput = {
  provider: string;
  transport: string;
  providerDeviceId: string;
  observedAtUnixMs?: number;
  clientSampleId?: string;
  metrics: Record<string, unknown>;
};

export type UploadTelemetryInput = RequestContext & {
  collectorSecret: string;
  samples: EdgeTelemetrySampleInput[];
};

export interface EdgeClient {
  createCollector(input: CreateCollectorInput): Promise<{ collector: EdgeCollector; setupToken: string }>;
  listCollectors(input: ListCollectorsInput): Promise<EdgeCollector[]>;
  enrollCollector(input: EnrollCollectorInput): Promise<{
    collector: EdgeCollector;
    collectorSecret: string;
    collectorEnv: Record<string, string>;
  }>;
  heartbeat(input: CollectorSecretInput): Promise<EdgeCollector>;
  uploadDiscovery(input: UploadDiscoveryInput): Promise<{ acceptedCount: number }>;
  listDeviceSources(input: ListDeviceSourcesInput): Promise<EdgeDeviceSource[]>;
  approveDeviceSource(input: ApproveDeviceSourceInput): Promise<{ source: EdgeDeviceSource; deviceId: string }>;
  uploadTelemetry(input: UploadTelemetryInput): Promise<{ acceptedCount: number; droppedCount: number }>;
  close(): void;
}

type GrpcUnaryMethod = (
  request: Record<string, unknown>,
  metadata: grpc.Metadata,
  options: grpc.CallOptions,
  callback: (error: grpc.ServiceError | null, response?: unknown) => void
) => void;

type GrpcEdgeClient = {
  CreateCollector: GrpcUnaryMethod;
  ListCollectors: GrpcUnaryMethod;
  EnrollCollector: GrpcUnaryMethod;
  Heartbeat: GrpcUnaryMethod;
  UploadDiscovery: GrpcUnaryMethod;
  ListDeviceSources: GrpcUnaryMethod;
  ApproveDeviceSource: GrpcUnaryMethod;
  UploadTelemetryBatch: GrpcUnaryMethod;
  close: () => void;
};

type EdgeProto = {
  pulse: {
    edge: {
      v1: {
        EdgeIngestService: new (
          address: string,
          credentials: grpc.ChannelCredentials,
          options?: Record<string, unknown>
        ) => GrpcEdgeClient;
      };
    };
  };
};

type RawCollector = Partial<Record<keyof EdgeCollector, unknown>>;
type RawDeviceSource = Partial<Record<keyof EdgeDeviceSource, unknown>>;

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, '../../../../');
const protoRoot = path.join(projectRoot, 'proto');
const edgeProtoPath = path.join(protoRoot, 'pulse/edge/v1/edge.proto');

const packageDefinition = protoLoader.loadSync(edgeProtoPath, {
  keepCase: false,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
  includeDirs: [protoRoot]
});
const edgeProto = grpc.loadPackageDefinition(packageDefinition) as unknown as EdgeProto;

export function createEdgeClient(address: string): EdgeClient {
  const client = new edgeProto.pulse.edge.v1.EdgeIngestService(
    address,
    grpc.credentials.createInsecure()
  );
  return {
    async createCollector(input) {
      const response = await unaryCall<{ collector?: unknown; setupToken?: unknown }>(
        client.CreateCollector.bind(client),
        { userSubject: input.userSubject, displayName: input.displayName ?? '' },
        input
      );
      return {
        collector: normalizeCollector((response.collector ?? {}) as RawCollector),
        setupToken: normalizeString(response.setupToken)
      };
    },
    async listCollectors(input) {
      const response = await unaryCall<{ collectors?: unknown }>(
        client.ListCollectors.bind(client),
        { userSubject: input.userSubject },
        input
      );
      return Array.isArray(response.collectors)
        ? response.collectors.map((row) => normalizeCollector(row as RawCollector))
        : [];
    },
    async enrollCollector(input) {
      const response = await unaryCall<{ collector?: unknown; collectorSecret?: unknown; collectorEnv?: unknown }>(
        client.EnrollCollector.bind(client),
        {
          setupToken: input.setupToken,
          collectorVersion: input.collectorVersion ?? '',
          hostname: input.hostname ?? ''
        },
        input
      );
      return {
        collector: normalizeCollector((response.collector ?? {}) as RawCollector),
        collectorSecret: normalizeString(response.collectorSecret),
        collectorEnv: normalizeStringRecord(response.collectorEnv)
      };
    },
    async heartbeat(input) {
      const response = await unaryCall<{ collector?: unknown }>(
        client.Heartbeat.bind(client),
        {
          collectorSecret: input.collectorSecret,
          collectorVersion: input.collectorVersion ?? '',
          hostname: input.hostname ?? ''
        },
        input
      );
      return normalizeCollector((response.collector ?? {}) as RawCollector);
    },
    async uploadDiscovery(input) {
      const response = await unaryCall<{ acceptedCount?: unknown }>(
        client.UploadDiscovery.bind(client),
        {
          collectorSecret: input.collectorSecret,
          discoveries: input.discoveries
        },
        input
      );
      return { acceptedCount: normalizeInteger(response.acceptedCount) };
    },
    async listDeviceSources(input) {
      const response = await unaryCall<{ sources?: unknown }>(
        client.ListDeviceSources.bind(client),
        {
          userSubject: input.userSubject,
          collectorId: input.collectorId ?? '',
          status: input.status ?? ''
        },
        input
      );
      return Array.isArray(response.sources)
        ? response.sources.map((row) => normalizeDeviceSource(row as RawDeviceSource))
        : [];
    },
    async approveDeviceSource(input) {
      const response = await unaryCall<{ source?: unknown; deviceId?: unknown }>(
        client.ApproveDeviceSource.bind(client),
        {
          userSubject: input.userSubject,
          sourceId: input.sourceId,
          deviceId: input.deviceId ?? '',
          productName: input.productName ?? '',
          model: input.model ?? ''
        },
        input
      );
      return {
        source: normalizeDeviceSource((response.source ?? {}) as RawDeviceSource),
        deviceId: normalizeString(response.deviceId)
      };
    },
    async uploadTelemetry(input) {
      const response = await unaryCall<{ acceptedCount?: unknown; droppedCount?: unknown }>(
        client.UploadTelemetryBatch.bind(client),
        {
          collectorSecret: input.collectorSecret,
          samples: input.samples
        },
        input
      );
      return {
        acceptedCount: normalizeInteger(response.acceptedCount),
        droppedCount: normalizeInteger(response.droppedCount)
      };
    },
    close() {
      client.close();
    }
  };
}

function unaryCall<T>(
  method: GrpcUnaryMethod,
  request: Record<string, unknown>,
  input: RequestContext
): Promise<T> {
  const metadata = new grpc.Metadata();
  if (input.authHeader) {
    metadata.set('authorization', input.authHeader);
  }
  if (input.requestID) {
    metadata.set('x-request-id', input.requestID);
  }
  return new Promise<T>((resolve, reject) => {
    method(
      request,
      metadata,
      { deadline: new Date(Date.now() + input.deadlineMs) },
      (error, response) => {
        if (error) {
          reject(error);
          return;
        }
        resolve(response as T);
      }
    );
  });
}

function normalizeCollector(row: RawCollector): EdgeCollector {
  return {
    id: normalizeString(row.id),
    displayName: normalizeString(row.displayName),
    isActive: Boolean(row.isActive),
    lastHeartbeatAtUnixMs: normalizeString(row.lastHeartbeatAtUnixMs),
    createdAtUnixMs: normalizeString(row.createdAtUnixMs),
    updatedAtUnixMs: normalizeString(row.updatedAtUnixMs),
    collectorVersion: normalizeString(row.collectorVersion),
    hostname: normalizeString(row.hostname)
  };
}

function normalizeDeviceSource(row: RawDeviceSource): EdgeDeviceSource {
  return {
    id: normalizeString(row.id),
    collectorId: normalizeString(row.collectorId),
    provider: normalizeString(row.provider),
    transport: normalizeString(row.transport),
    providerDeviceId: normalizeString(row.providerDeviceId),
    displayName: normalizeString(row.displayName),
    model: normalizeString(row.model),
    status: normalizeDeviceSourceStatus(row.status),
    linkedDeviceId: normalizeString(row.linkedDeviceId),
    rssiDbm: normalizeInteger(row.rssiDbm),
    lastSeenAtUnixMs: normalizeString(row.lastSeenAtUnixMs),
    createdAtUnixMs: normalizeString(row.createdAtUnixMs),
    updatedAtUnixMs: normalizeString(row.updatedAtUnixMs),
    metadata: normalizeRecord(row.metadata)
  };
}

function normalizeString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function normalizeDeviceSourceStatus(value: unknown): EdgeDeviceSourceStatus {
  const status = normalizeString(value);
  if (status === 'linked' || status === 'ignored') {
    return status;
  }
  return 'pending';
}

function normalizeInteger(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.trunc(value);
  }
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return Math.trunc(parsed);
    }
  }
  return 0;
}

function normalizeRecord(value: unknown): Record<string, unknown> | undefined {
  const normalized = normalizeProtoValue(value);
  if (!normalized || typeof normalized !== 'object' || Array.isArray(normalized)) {
    return undefined;
  }
  return normalized as Record<string, unknown>;
}

function normalizeStringRecord(value: unknown): Record<string, string> {
  const record = normalizeRecord(value);
  if (!record) {
    return {};
  }
  const out: Record<string, string> = {};
  for (const [key, raw] of Object.entries(record)) {
    const normalizedKey = key.trim();
    const normalizedValue = normalizeString(raw).trim();
    if (normalizedKey && normalizedValue) {
      out[normalizedKey] = normalizedValue;
    }
  }
  return out;
}

function normalizeProtoValue(value: unknown): unknown {
  if (value === null || value === undefined) {
    return undefined;
  }
  if (Array.isArray(value)) {
    return value.map((item) => normalizeProtoValue(item));
  }
  if (typeof value !== 'object') {
    return value;
  }

  const record = value as Record<string, unknown>;
  if ('fields' in record && typeof record.fields === 'object' && record.fields !== null && !Array.isArray(record.fields)) {
    const out: Record<string, unknown> = {};
    for (const [key, fieldValue] of Object.entries(record.fields as Record<string, unknown>)) {
      out[key] = normalizeProtoValue(fieldValue);
    }
    return out;
  }
  if ('kind' in record && typeof record.kind === 'string') {
    return normalizeProtoValue(record[record.kind]);
  }
  return record;
}
