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
  acOutputAvgW: number;
  acOutputMaxW: number;
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
  acInputEnergyWh: number;
  acOutputEnergyWh: number;
  dcOutputEnergyWh: number;
  loadEnergyWh: number;
  batteryChargeEnergyWh: number;
  batteryDischargeEnergyWh: number;
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

export type EnergyValueComparison = {
  current: number;
  previous: number;
  delta: number;
  deltaPct: number | null;
};

export type EnergyScope = {
  mode: string;
  deviceId: string;
  resolvedDeviceIds: string[];
};

export type EnergyWindow = {
  preset: string;
  timezone: string;
  fromUnixMs: string;
  toUnixMs: string;
  previousFromUnixMs: string;
  previousToUnixMs: string;
};

export type EnergySummary = {
  solarGeneratedKwh: EnergyValueComparison;
  loadConsumedKwh: EnergyValueComparison;
  selfSufficiencyPct: EnergyValueComparison;
  batteryNetKwh: EnergyValueComparison;
  estimatedValue: EnergyValueComparison;
  estimatedAcInputCost: EnergyValueComparison;
  currency: string;
};

export type BatterySummary = {
  chargeKwh: number;
  dischargeKwh: number;
  netKwh: number;
  socStartPct: number;
  socEndPct: number;
  socMinPct: number;
  socMaxPct: number;
};

export type EnergyDashboard = {
  scope: EnergyScope;
  window: EnergyWindow;
  summary: EnergySummary;
  battery: BatterySummary;
  currentEnergyPoints: RollupPoint[];
  previousEnergyPoints: RollupPoint[];
  currentPowerPoints: RollupPoint[];
  previousPowerPoints: RollupPoint[];
  pvPortHistory: EnergyPVPortHistory[];
};

export type EnergyPVPortHistory = {
  deviceId: string;
  portId: string;
  portLabel: string;
  maxObservedVolts: number;
  maxObservedAmps: number;
  maxObservedWatts: number;
  lastObservedVolts: number;
  lastObservedAmps: number;
  lastObservedWatts: number;
  lastObservedUnixMs: string;
  sampleCount: number;
};

export type SnapshotRequestInput = {
  deviceId: string;
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
};

export type SnapshotCursor = {
  seq: string;
  tsUnixMs: string;
};

export type Snapshot = {
  deviceId: string;
  cursor: SnapshotCursor;
  metrics: Record<string, number>;
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
  compareFromUnixMs?: string;
  compareToUnixMs?: string;
};

export type GetEnergyDashboardInput = {
  deviceId?: string;
  useAllDevices: boolean;
  preset: string;
  timezone: string;
  includeComparison: boolean;
  gridPricePerKwh?: number;
  currency?: string;
  authHeader?: string;
  userSubject?: string;
  requestID?: string;
  deadlineMs: number;
};

export interface TelemetryHistoryClient {
  getSnapshot(input: SnapshotRequestInput): Promise<{ snapshot: Snapshot }>;
  queryRollupRange(input: QueryRollupRangeInput): Promise<RollupSeries>;
  compareRollupRange(input: CompareRollupRangeInput): Promise<CompareRollupSeries>;
  getEnergyDashboard(input: GetEnergyDashboardInput): Promise<EnergyDashboard>;
  close(): void;
}

export interface TelemetrySnapshotClient {
  getSnapshot(input: SnapshotRequestInput): Promise<{ snapshot: Snapshot }>;
  close(): void;
}

type GrpcUnaryMethod = (
  request: Record<string, unknown>,
  metadata: grpc.Metadata,
  options: grpc.CallOptions,
  callback: (error: grpc.ServiceError | null, response?: unknown) => void
) => void;

type GrpcTelemetryClient = {
  GetSnapshot: GrpcUnaryMethod;
  QueryRollupRange: GrpcUnaryMethod;
  CompareRollupRange: GrpcUnaryMethod;
  GetEnergyDashboard: GrpcUnaryMethod;
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

type RawSnapshot = {
  deviceId?: unknown;
  cursor?: {
    seq?: unknown;
    tsUnixMs?: unknown;
  };
  metrics?: unknown;
};

type RawGetSnapshotResponse = {
  snapshot?: RawSnapshot;
};

type RawCompareRollupRangeResponse = {
  current?: RawRollupSeries;
  previous?: RawRollupSeries;
};

type RawEnergyValueComparison = {
  current?: unknown;
  previous?: unknown;
  delta?: unknown;
  deltaPct?: unknown;
};

type RawEnergySummary = {
  solarGeneratedKwh?: RawEnergyValueComparison;
  loadConsumedKwh?: RawEnergyValueComparison;
  selfSufficiencyPct?: RawEnergyValueComparison;
  batteryNetKwh?: RawEnergyValueComparison;
  estimatedValue?: RawEnergyValueComparison;
  estimatedAcInputCost?: RawEnergyValueComparison;
  currency?: unknown;
};

type RawBatterySummary = {
  chargeKwh?: unknown;
  dischargeKwh?: unknown;
  netKwh?: unknown;
  socStartPct?: unknown;
  socEndPct?: unknown;
  socMinPct?: unknown;
  socMaxPct?: unknown;
};

type RawEnergyScope = {
  mode?: unknown;
  deviceId?: unknown;
  resolvedDeviceIds?: unknown;
};

type RawEnergyWindow = {
  preset?: unknown;
  timezone?: unknown;
  fromUnixMs?: unknown;
  toUnixMs?: unknown;
  previousFromUnixMs?: unknown;
  previousToUnixMs?: unknown;
};

type RawEnergyPVPortHistory = {
  deviceId?: unknown;
  portId?: unknown;
  portLabel?: unknown;
  maxObservedVolts?: unknown;
  maxObservedAmps?: unknown;
  maxObservedWatts?: unknown;
  lastObservedVolts?: unknown;
  lastObservedAmps?: unknown;
  lastObservedWatts?: unknown;
  lastObservedUnixMs?: unknown;
  sampleCount?: unknown;
};

type RawGetEnergyDashboardResponse = {
  scope?: RawEnergyScope;
  window?: RawEnergyWindow;
  summary?: RawEnergySummary;
  battery?: RawBatterySummary;
  currentEnergyPoints?: unknown;
  previousEnergyPoints?: unknown;
  currentPowerPoints?: unknown;
  previousPowerPoints?: unknown;
  pvPortHistory?: unknown;
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
  'acOutputAvgW',
  'acOutputMaxW',
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
  'solarGeneratedWh',
  'acInputEnergyWh',
  'acOutputEnergyWh',
  'dcOutputEnergyWh',
  'loadEnergyWh',
  'batteryChargeEnergyWh',
  'batteryDischargeEnergyWh'
];

export function createTelemetryHistoryClient(address: string): TelemetryHistoryClient {
  const client = new telemetryProto.pulse.telemetry.v1.TelemetryService(
    address,
    grpc.credentials.createInsecure()
  );
  return {
    async getSnapshot(input) {
      const response = await unaryCall<RawGetSnapshotResponse>(
        client.GetSnapshot.bind(client),
        {
          deviceId: input.deviceId
        },
        input
      );
      return {
        snapshot: normalizeSnapshot(response.snapshot)
      };
    },
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
      const request: Record<string, unknown> = {
        deviceId: input.deviceId,
        resolution: resolutionMap[input.resolution],
        fromUnixMs: input.fromUnixMs,
        toUnixMs: input.toUnixMs,
        usePreviousPeriod: input.usePreviousPeriod
      };
      if (input.compareFromUnixMs) {
        request.compareFromUnixMs = input.compareFromUnixMs;
      }
      if (input.compareToUnixMs) {
        request.compareToUnixMs = input.compareToUnixMs;
      }
      const response = await unaryCall<RawCompareRollupRangeResponse>(
        client.CompareRollupRange.bind(client),
        request,
        input
      );
      return {
        current: normalizeSeries(response.current),
        previous: normalizeSeries(response.previous)
      };
    },
    async getEnergyDashboard(input) {
      const response = await unaryCall<RawGetEnergyDashboardResponse>(
        client.GetEnergyDashboard.bind(client),
        {
          deviceId: input.deviceId ?? '',
          useAllDevices: input.useAllDevices,
          preset: input.preset,
          timezone: input.timezone,
          includeComparison: input.includeComparison,
          gridPricePerKwh: input.gridPricePerKwh ?? 0,
          currency: input.currency ?? ''
        },
        input
      );
      return normalizeEnergyDashboard(response);
    },
    close() {
      client.close();
    }
  };
}

export function createTelemetrySnapshotClient(address: string): TelemetrySnapshotClient {
  const client = createTelemetryHistoryClient(address);
  return {
    getSnapshot: client.getSnapshot,
    close: client.close
  };
}

function unaryCall<T>(
  method: GrpcUnaryMethod,
  request: Record<string, unknown>,
  input: { authHeader?: string; userSubject?: string; requestID?: string; deadlineMs: number }
): Promise<T> {
  const metadata = new grpc.Metadata();
  if (input.authHeader) {
    metadata.set('authorization', input.authHeader);
  }
  if (input.userSubject) {
    metadata.set('x-user-subject', input.userSubject);
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

function normalizeSnapshot(snapshot: RawSnapshot | undefined): Snapshot {
  return {
    deviceId: normalizeString(snapshot?.deviceId),
    cursor: {
      seq: normalizeString(snapshot?.cursor?.seq),
      tsUnixMs: normalizeString(snapshot?.cursor?.tsUnixMs)
    },
    metrics: normalizeNumericMap(snapshot?.metrics)
  };
}

function normalizeNumericMap(value: unknown): Record<string, number> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return {};
  }
  const out: Record<string, number> = {};
  for (const [key, entry] of Object.entries(value)) {
    if (typeof entry === 'number' && Number.isFinite(entry)) {
      out[key] = entry;
    }
  }
  return out;
}

function normalizeMetrics(metrics: RawRollupMetrics | undefined): RollupMetrics {
  const normalized = {} as RollupMetrics;
  for (const key of metricsKeys) {
    normalized[key] = normalizeNumber(metrics?.[key]);
  }
  return normalized;
}

function normalizeEnergyDashboard(response: RawGetEnergyDashboardResponse): EnergyDashboard {
  return {
    scope: {
      mode: normalizeString(response.scope?.mode),
      deviceId: normalizeString(response.scope?.deviceId),
      resolvedDeviceIds: Array.isArray(response.scope?.resolvedDeviceIds)
        ? response.scope.resolvedDeviceIds.map((value) => normalizeString(value))
        : []
    },
    window: {
      preset: normalizeString(response.window?.preset),
      timezone: normalizeString(response.window?.timezone),
      fromUnixMs: normalizeString(response.window?.fromUnixMs),
      toUnixMs: normalizeString(response.window?.toUnixMs),
      previousFromUnixMs: normalizeString(response.window?.previousFromUnixMs),
      previousToUnixMs: normalizeString(response.window?.previousToUnixMs)
    },
    summary: normalizeEnergySummary(response.summary),
    battery: normalizeBatterySummary(response.battery),
    currentEnergyPoints: normalizePoints(response.currentEnergyPoints),
    previousEnergyPoints: normalizePoints(response.previousEnergyPoints),
    currentPowerPoints: normalizePoints(response.currentPowerPoints),
    previousPowerPoints: normalizePoints(response.previousPowerPoints),
    pvPortHistory: normalizePVPortHistoryRows(response.pvPortHistory)
  };
}

function normalizeEnergySummary(summary: RawEnergySummary | undefined): EnergySummary {
  return {
    solarGeneratedKwh: normalizeEnergyValueComparison(summary?.solarGeneratedKwh),
    loadConsumedKwh: normalizeEnergyValueComparison(summary?.loadConsumedKwh),
    selfSufficiencyPct: normalizeEnergyValueComparison(summary?.selfSufficiencyPct),
    batteryNetKwh: normalizeEnergyValueComparison(summary?.batteryNetKwh),
    estimatedValue: normalizeEnergyValueComparison(summary?.estimatedValue),
    estimatedAcInputCost: normalizeEnergyValueComparison(summary?.estimatedAcInputCost),
    currency: normalizeString(summary?.currency)
  };
}

function normalizeEnergyValueComparison(value: RawEnergyValueComparison | undefined): EnergyValueComparison {
  return {
    current: normalizeNumber(value?.current),
    previous: normalizeNumber(value?.previous),
    delta: normalizeNumber(value?.delta),
    deltaPct: normalizeNullableNumber(value?.deltaPct)
  };
}

function normalizeBatterySummary(summary: RawBatterySummary | undefined): BatterySummary {
  return {
    chargeKwh: normalizeNumber(summary?.chargeKwh),
    dischargeKwh: normalizeNumber(summary?.dischargeKwh),
    netKwh: normalizeNumber(summary?.netKwh),
    socStartPct: normalizeNumber(summary?.socStartPct),
    socEndPct: normalizeNumber(summary?.socEndPct),
    socMinPct: normalizeNumber(summary?.socMinPct),
    socMaxPct: normalizeNumber(summary?.socMaxPct)
  };
}

function normalizePoints(points: unknown): RollupPoint[] {
  return Array.isArray(points) ? points.map((point) => normalizePoint(point as RawRollupPoint)) : [];
}

function normalizePVPortHistoryRows(rows: unknown): EnergyPVPortHistory[] {
  return Array.isArray(rows)
    ? rows.map((row) => normalizePVPortHistoryRow(row as RawEnergyPVPortHistory))
    : [];
}

function normalizePVPortHistoryRow(row: RawEnergyPVPortHistory): EnergyPVPortHistory {
  return {
    deviceId: normalizeString(row.deviceId),
    portId: normalizeString(row.portId),
    portLabel: normalizeString(row.portLabel),
    maxObservedVolts: normalizeNumber(row.maxObservedVolts),
    maxObservedAmps: normalizeNumber(row.maxObservedAmps),
    maxObservedWatts: normalizeNumber(row.maxObservedWatts),
    lastObservedVolts: normalizeNumber(row.lastObservedVolts),
    lastObservedAmps: normalizeNumber(row.lastObservedAmps),
    lastObservedWatts: normalizeNumber(row.lastObservedWatts),
    lastObservedUnixMs: normalizeString(row.lastObservedUnixMs),
    sampleCount: normalizeInt(row.sampleCount)
  };
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

function normalizeNullableNumber(value: unknown): number | null {
  if (value === null || value === undefined || value === '') {
    return null;
  }
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : null;
  }
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number.parseFloat(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}
