import path from 'node:path';
import { fileURLToPath } from 'node:url';

import grpc from '@grpc/grpc-js';
import protoLoader from '@grpc/proto-loader';

export type RollupResolution = 'minute' | 'hour' | 'day';

export type RollupMetrics = {
  socAvgPct: number;
  socMinPct: number;
  socMaxPct: number;
  acInAvgW: number;
  acInMaxW: number;
  pvAvgW: number;
  pvMaxW: number;
  dcAvgW: number;
  dcMaxW: number;
  loadAvgW: number;
  loadMaxW: number;
  netAvgW: number;
  netMinW: number;
  netMaxW: number;
  batteryAvgW: number;
  batteryMinW: number;
  batteryMaxW: number;
  tempAvgC: number;
  tempMinC: number;
  tempMaxC: number;
  solarGeneratedWh: number;
};

export type RollupPoint = {
  bucketStartUnixMs: string;
  bucketEndUnixMs: string;
  sampleCount: number;
  firstTsUnixMs: string;
  lastTsUnixMs: string;
  metrics: RollupMetrics;
};

export type RollupSeries = {
  deviceId: string;
  resolution: RollupResolution;
  fromUnixMs: string;
  toUnixMs: string;
  points: RollupPoint[];
};

export type CompareRollupSeries = {
  current: RollupSeries;
  previous: RollupSeries;
};

export type QueryRollupRangeInput = {
  deviceId: string;
  resolution: RollupResolution;
  fromUnixMs: string;
  toUnixMs: string;
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
};

export type CompareRollupRangeInput = QueryRollupRangeInput & {
  usePreviousPeriod: boolean;
};

export interface TelemetryHistoryClient {
  queryRollupRange(input: QueryRollupRangeInput): Promise<RollupSeries>;
  compareRollupRange(input: CompareRollupRangeInput): Promise<CompareRollupSeries>;
  close(): void;
}

type GrpcUnaryMethod = (
  request: Record<string, unknown>,
  metadata: grpc.Metadata,
  options: grpc.CallOptions,
  callback: (error: grpc.ServiceError | null, response?: unknown) => void
) => void;

type GrpcTelemetryClient = {
  QueryRollupRange: GrpcUnaryMethod;
  CompareRollupRange: GrpcUnaryMethod;
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

type RawRollupMetrics = Partial<Record<keyof RollupMetrics, unknown>>;

type RawRollupPoint = {
  bucketStartUnixMs?: unknown;
  bucketEndUnixMs?: unknown;
  sampleCount?: unknown;
  firstTsUnixMs?: unknown;
  lastTsUnixMs?: unknown;
  metrics?: RawRollupMetrics;
};

type RawRollupSeries = {
  deviceId?: unknown;
  resolution?: unknown;
  fromUnixMs?: unknown;
  toUnixMs?: unknown;
  points?: unknown;
};

type RawQueryRollupRangeResponse = {
  series?: RawRollupSeries;
};

type RawCompareRollupRangeResponse = {
  current?: RawRollupSeries;
  previous?: RawRollupSeries;
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

const resolutionMap: Record<RollupResolution, string> = {
  minute: 'ROLLUP_RESOLUTION_MINUTE',
  hour: 'ROLLUP_RESOLUTION_HOUR',
  day: 'ROLLUP_RESOLUTION_DAY'
};

const metricsKeys: (keyof RollupMetrics)[] = [
  'socAvgPct',
  'socMinPct',
  'socMaxPct',
  'acInAvgW',
  'acInMaxW',
  'pvAvgW',
  'pvMaxW',
  'dcAvgW',
  'dcMaxW',
  'loadAvgW',
  'loadMaxW',
  'netAvgW',
  'netMinW',
  'netMaxW',
  'batteryAvgW',
  'batteryMinW',
  'batteryMaxW',
  'tempAvgC',
  'tempMinC',
  'tempMaxC',
  'solarGeneratedWh'
];

export function createTelemetryHistoryClient(address: string): TelemetryHistoryClient {
  const client = new telemetryProto.pulse.telemetry.v1.TelemetryService(
    address,
    grpc.credentials.createInsecure()
  );
  return {
    async queryRollupRange(input) {
      const response = await unaryCall<RawQueryRollupRangeResponse>(
        client.QueryRollupRange.bind(client),
        {
          deviceId: input.deviceId,
          resolution: resolutionMap[input.resolution],
          fromUnixMs: input.fromUnixMs,
          toUnixMs: input.toUnixMs
        },
        input
      );
      return normalizeSeries(response.series);
    },
    async compareRollupRange(input) {
      const response = await unaryCall<RawCompareRollupRangeResponse>(
        client.CompareRollupRange.bind(client),
        {
          deviceId: input.deviceId,
          resolution: resolutionMap[input.resolution],
          fromUnixMs: input.fromUnixMs,
          toUnixMs: input.toUnixMs,
          usePreviousPeriod: input.usePreviousPeriod
        },
        input
      );
      return {
        current: normalizeSeries(response.current),
        previous: normalizeSeries(response.previous)
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
  input: { authHeader?: string; requestID?: string; deadlineMs: number }
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

function normalizeSeries(series: RawRollupSeries | undefined): RollupSeries {
  const resolution = normalizeResolution(series?.resolution);
  return {
    deviceId: normalizeString(series?.deviceId),
    resolution,
    fromUnixMs: normalizeString(series?.fromUnixMs),
    toUnixMs: normalizeString(series?.toUnixMs),
    points: Array.isArray(series?.points) ? series.points.map((point) => normalizePoint(point as RawRollupPoint)) : []
  };
}

function normalizePoint(point: RawRollupPoint): RollupPoint {
  return {
    bucketStartUnixMs: normalizeString(point.bucketStartUnixMs),
    bucketEndUnixMs: normalizeString(point.bucketEndUnixMs),
    sampleCount: normalizeInt(point.sampleCount),
    firstTsUnixMs: normalizeString(point.firstTsUnixMs),
    lastTsUnixMs: normalizeString(point.lastTsUnixMs),
    metrics: normalizeMetrics(point.metrics)
  };
}

function normalizeMetrics(metrics: RawRollupMetrics | undefined): RollupMetrics {
  const normalized = {} as RollupMetrics;
  for (const key of metricsKeys) {
    normalized[key] = normalizeNumber(metrics?.[key]);
  }
  return normalized;
}

function normalizeResolution(value: unknown): RollupResolution {
  switch (String(value)) {
    case 'ROLLUP_RESOLUTION_MINUTE':
      return 'minute';
    case 'ROLLUP_RESOLUTION_DAY':
      return 'day';
    default:
      return 'hour';
  }
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
  if (typeof value === 'number') {
    return Number.isFinite(value) ? Math.trunc(value) : 0;
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

function normalizeNumber(value: unknown): number {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : 0;
  }
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number.parseFloat(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}
