import path from 'node:path';
import { fileURLToPath } from 'node:url';

import grpc from '@grpc/grpc-js';
import protoLoader from '@grpc/proto-loader';

export type InsightKind =
  | 'unspecified'
  | 'battery_expansion'
  | 'solar_add_on'
  | 'solar_upgrade'
  | 'energy_shift'
  | 'maintenance'
  | 'energy_comparison';

export type InsightStatus = 'pending' | 'ready' | 'stale' | 'unavailable' | 'unspecified';

export type InsightActionKind = 'internal_route' | 'external_url' | 'learn_more' | 'dismiss' | 'unspecified';

export type InsightEvidenceSource =
  | 'live_snapshot'
  | 'rollup_history'
  | 'device_capabilities'
  | 'provider_metadata'
  | 'model_output'
  | 'rule_engine'
  | 'user_context'
  | 'unspecified';

export type InsightAction = {
  kind: InsightActionKind;
  label: string;
  target: string;
  params?: Record<string, unknown>;
};

export type InsightEvidence = {
  source: InsightEvidenceSource;
  summary: string;
  metrics?: Record<string, unknown>;
};

export type DeviceInsight = {
  id: string;
  deviceId: string;
  kind: InsightKind;
  title: string;
  summary: string;
  score: number;
  rank: number;
  modelKey: string;
  modelVersion: string;
  generatedAtUnixMs: string;
  expiresAtUnixMs: string;
  tags: string[];
  evidence: InsightEvidence[];
  actions: InsightAction[];
  attributes?: Record<string, unknown>;
};

export type DeviceInsights = {
  deviceId: string;
  status: InsightStatus;
  statusDetail: string;
  refreshedAtUnixMs: string;
  insights: DeviceInsight[];
};

export type EnergyComparisonCardCategory =
  | 'unspecified'
  | 'self_sufficiency'
  | 'solar'
  | 'load'
  | 'battery'
  | 'grid'
  | 'value';

export type EnergyComparisonCard = {
  category: EnergyComparisonCardCategory;
  title: string;
  summary: string;
  recommendation: string;
  score: number;
  confidence: number;
  evidence: InsightEvidence[];
  attributes?: Record<string, unknown>;
};

export type EnergyComparisonInsight = {
  id: string;
  scope: {
    mode: string;
    deviceId: string;
    resolvedDeviceIds: string[];
  };
  preset: string;
  timezone: string;
  verdictClass: string;
  headline: string;
  summary: string;
  score: number;
  confidence: number;
  modelKey: string;
  modelVersion: string;
  generatedAtUnixMs: string;
  expiresAtUnixMs: string;
  tags: string[];
  cards: EnergyComparisonCard[];
  evidence: InsightEvidence[];
  attributes?: Record<string, unknown>;
};

export type EnergyComparisonInsightResponse = {
  status: InsightStatus;
  statusDetail: string;
  insight?: EnergyComparisonInsight;
};

export type GetDeviceInsightsInput = {
  deviceId: string;
  kinds?: InsightKind[];
  maxItems?: number;
  authHeader?: string;
  userSubject?: string;
  requestID?: string;
  deadlineMs: number;
};

export type GetEnergyComparisonInsightInput = {
  deviceId?: string;
  useAllDevices: boolean;
  preset: string;
  timezone: string;
  gridPricePerKwh?: number;
  currency?: string;
  authHeader?: string;
  userSubject?: string;
  requestID?: string;
  deadlineMs: number;
};

export interface InferenceClient {
  getDeviceInsights(input: GetDeviceInsightsInput): Promise<DeviceInsights>;
  getEnergyComparisonInsight(input: GetEnergyComparisonInsightInput): Promise<EnergyComparisonInsightResponse>;
  close(): void;
}

type GrpcUnaryMethod = (
  request: Record<string, unknown>,
  metadata: grpc.Metadata,
  options: grpc.CallOptions,
  callback: (error: grpc.ServiceError | null, response?: unknown) => void
) => void;

type GrpcInferenceClient = {
  GetDeviceInsights: GrpcUnaryMethod;
  GetEnergyComparisonInsight: GrpcUnaryMethod;
  close: () => void;
};

type InferenceProto = {
  pulse: {
    inference: {
      v1: {
        InferenceService: new (
          address: string,
          credentials: grpc.ChannelCredentials,
          options?: Record<string, unknown>
        ) => GrpcInferenceClient;
      };
    };
  };
};

type RawInsightAction = Partial<Record<keyof InsightAction, unknown>>;
type RawInsightEvidence = Partial<Record<keyof InsightEvidence, unknown>>;
type RawDeviceInsight = Partial<Record<keyof DeviceInsight, unknown>>;
type RawDeviceInsights = Partial<Record<keyof DeviceInsights, unknown>>;
type RawGetDeviceInsightsResponse = {
  insights?: RawDeviceInsights;
};
type RawEnergyComparisonCard = Partial<Record<keyof EnergyComparisonCard, unknown>>;
type RawEnergyComparisonInsight = Partial<Record<keyof EnergyComparisonInsight, unknown>> & {
  scope?: { mode?: unknown; deviceId?: unknown; resolvedDeviceIds?: unknown };
  cards?: unknown;
  evidence?: unknown;
};
type RawGetEnergyComparisonInsightResponse = {
  status?: unknown;
  statusDetail?: unknown;
  insight?: RawEnergyComparisonInsight;
};

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, '../../../../');
const protoRoot = path.join(projectRoot, 'proto');
const inferenceProtoPath = path.join(protoRoot, 'pulse/inference/v1/inference.proto');

const packageDefinition = protoLoader.loadSync(inferenceProtoPath, {
  keepCase: false,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
  includeDirs: [protoRoot]
});
const inferenceProto = grpc.loadPackageDefinition(packageDefinition) as unknown as InferenceProto;

const kindToProtoValue: Partial<Record<InsightKind, string>> = {
  battery_expansion: 'INSIGHT_KIND_BATTERY_EXPANSION',
  solar_add_on: 'INSIGHT_KIND_SOLAR_ADD_ON',
  solar_upgrade: 'INSIGHT_KIND_SOLAR_UPGRADE',
  energy_shift: 'INSIGHT_KIND_ENERGY_SHIFT',
  maintenance: 'INSIGHT_KIND_MAINTENANCE'
  ,
  energy_comparison: 'INSIGHT_KIND_ENERGY_COMPARISON'
};

export function createInferenceClient(address: string): InferenceClient {
  const client = new inferenceProto.pulse.inference.v1.InferenceService(
    address,
    grpc.credentials.createInsecure()
  );
  return {
    async getDeviceInsights(input) {
      const request: Record<string, unknown> = {
        deviceId: input.deviceId
      };
      if ((input.kinds?.length ?? 0) > 0 || (input.maxItems ?? 0) > 0) {
        request.filter = {
          kinds: (input.kinds ?? []).flatMap((kind) => {
            const mapped = kindToProtoValue[kind];
            return mapped ? [mapped] : [];
          }),
          maxItems: input.maxItems ?? 0
        };
      }
      const response = await unaryCall<RawGetDeviceInsightsResponse>(
        client.GetDeviceInsights.bind(client),
        request,
        input
      );
      return normalizeDeviceInsights(response.insights);
    },
    async getEnergyComparisonInsight(input) {
      const request: Record<string, unknown> = {
        useAllDevices: input.useAllDevices,
        preset: input.preset,
        timezone: input.timezone,
        gridPricePerKwh: input.gridPricePerKwh ?? 0,
        currency: input.currency ?? ''
      };
      if (input.deviceId) {
        request.deviceId = input.deviceId;
      }
      const response = await unaryCall<RawGetEnergyComparisonInsightResponse>(
        client.GetEnergyComparisonInsight.bind(client),
        request,
        input
      );
      return normalizeEnergyComparisonInsightResponse(response);
    },
    close() {
      client.close();
    }
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

function normalizeDeviceInsights(value: RawDeviceInsights | undefined): DeviceInsights {
  return {
    deviceId: normalizeString(value?.deviceId),
    status: normalizeInsightStatus(value?.status),
    statusDetail: normalizeString(value?.statusDetail),
    refreshedAtUnixMs: normalizeString(value?.refreshedAtUnixMs),
    insights: Array.isArray(value?.insights)
      ? value.insights.map((entry) => normalizeDeviceInsight(entry as RawDeviceInsight))
      : []
  };
}

function normalizeDeviceInsight(value: RawDeviceInsight): DeviceInsight {
  return {
    id: normalizeString(value.id),
    deviceId: normalizeString(value.deviceId),
    kind: normalizeInsightKind(value.kind),
    title: normalizeString(value.title),
    summary: normalizeString(value.summary),
    score: normalizeNumber(value.score),
    rank: normalizeInt(value.rank),
    modelKey: normalizeString(value.modelKey),
    modelVersion: normalizeString(value.modelVersion),
    generatedAtUnixMs: normalizeString(value.generatedAtUnixMs),
    expiresAtUnixMs: normalizeString(value.expiresAtUnixMs),
    tags: Array.isArray(value.tags) ? value.tags.filter((entry): entry is string => typeof entry === 'string') : [],
    evidence: Array.isArray(value.evidence)
      ? value.evidence.map((entry) => normalizeInsightEvidence(entry as RawInsightEvidence))
      : [],
    actions: Array.isArray(value.actions)
      ? value.actions.map((entry) => normalizeInsightAction(entry as RawInsightAction))
      : [],
    attributes: normalizeRecord(value.attributes)
  };
}

function normalizeInsightAction(value: RawInsightAction): InsightAction {
  return {
    kind: normalizeInsightActionKind(value.kind),
    label: normalizeString(value.label),
    target: normalizeString(value.target),
    params: normalizeRecord(value.params)
  };
}

function normalizeInsightEvidence(value: RawInsightEvidence): InsightEvidence {
  return {
    source: normalizeInsightEvidenceSource(value.source),
    summary: normalizeString(value.summary),
    metrics: normalizeRecord(value.metrics)
  };
}

function normalizeInsightKind(value: unknown): InsightKind {
  switch (value) {
    case 'INSIGHT_KIND_BATTERY_EXPANSION':
      return 'battery_expansion';
    case 'INSIGHT_KIND_SOLAR_ADD_ON':
      return 'solar_add_on';
    case 'INSIGHT_KIND_SOLAR_UPGRADE':
      return 'solar_upgrade';
    case 'INSIGHT_KIND_ENERGY_SHIFT':
      return 'energy_shift';
    case 'INSIGHT_KIND_MAINTENANCE':
      return 'maintenance';
    case 'INSIGHT_KIND_ENERGY_COMPARISON':
      return 'energy_comparison';
    default:
      return 'unspecified';
  }
}

function normalizeEnergyComparisonInsightResponse(
  value: RawGetEnergyComparisonInsightResponse | undefined
): EnergyComparisonInsightResponse {
  return {
    status: normalizeInsightStatus(value?.status),
    statusDetail: normalizeString(value?.statusDetail),
    insight: value?.insight ? normalizeEnergyComparisonInsight(value.insight) : undefined
  };
}

function normalizeEnergyComparisonInsight(value: RawEnergyComparisonInsight): EnergyComparisonInsight {
  return {
    id: normalizeString(value.id),
    scope: {
      mode: normalizeString(value.scope?.mode),
      deviceId: normalizeString(value.scope?.deviceId),
      resolvedDeviceIds: Array.isArray(value.scope?.resolvedDeviceIds)
        ? value.scope!.resolvedDeviceIds.filter((entry): entry is string => typeof entry === 'string')
        : []
    },
    preset: normalizeString(value.preset),
    timezone: normalizeString(value.timezone),
    verdictClass: normalizeString(value.verdictClass),
    headline: normalizeString(value.headline),
    summary: normalizeString(value.summary),
    score: normalizeNumber(value.score),
    confidence: normalizeNumber(value.confidence),
    modelKey: normalizeString(value.modelKey),
    modelVersion: normalizeString(value.modelVersion),
    generatedAtUnixMs: normalizeString(value.generatedAtUnixMs),
    expiresAtUnixMs: normalizeString(value.expiresAtUnixMs),
    tags: Array.isArray(value.tags) ? value.tags.filter((entry): entry is string => typeof entry === 'string') : [],
    cards: Array.isArray(value.cards)
      ? value.cards.map((entry) => normalizeEnergyComparisonCard(entry as RawEnergyComparisonCard))
      : [],
    evidence: Array.isArray(value.evidence)
      ? value.evidence.map((entry) => normalizeInsightEvidence(entry as RawInsightEvidence))
      : [],
    attributes: normalizeRecord(value.attributes)
  };
}

function normalizeEnergyComparisonCard(value: RawEnergyComparisonCard): EnergyComparisonCard {
  return {
    category: normalizeEnergyComparisonCardCategory(value.category),
    title: normalizeString(value.title),
    summary: normalizeString(value.summary),
    recommendation: normalizeString(value.recommendation),
    score: normalizeNumber(value.score),
    confidence: normalizeNumber(value.confidence),
    evidence: Array.isArray(value.evidence)
      ? value.evidence.map((entry) => normalizeInsightEvidence(entry as RawInsightEvidence))
      : [],
    attributes: normalizeRecord(value.attributes)
  };
}

function normalizeEnergyComparisonCardCategory(value: unknown): EnergyComparisonCardCategory {
  switch (value) {
    case 'ENERGY_COMPARISON_CARD_CATEGORY_SELF_SUFFICIENCY':
      return 'self_sufficiency';
    case 'ENERGY_COMPARISON_CARD_CATEGORY_SOLAR':
      return 'solar';
    case 'ENERGY_COMPARISON_CARD_CATEGORY_LOAD':
      return 'load';
    case 'ENERGY_COMPARISON_CARD_CATEGORY_BATTERY':
      return 'battery';
    case 'ENERGY_COMPARISON_CARD_CATEGORY_GRID':
      return 'grid';
    case 'ENERGY_COMPARISON_CARD_CATEGORY_VALUE':
      return 'value';
    default:
      return 'unspecified';
  }
}

function normalizeInsightStatus(value: unknown): InsightStatus {
  switch (value) {
    case 'INSIGHT_STATUS_PENDING':
      return 'pending';
    case 'INSIGHT_STATUS_READY':
      return 'ready';
    case 'INSIGHT_STATUS_STALE':
      return 'stale';
    case 'INSIGHT_STATUS_UNAVAILABLE':
      return 'unavailable';
    default:
      return 'unspecified';
  }
}

function normalizeInsightActionKind(value: unknown): InsightActionKind {
  switch (value) {
    case 'INSIGHT_ACTION_KIND_INTERNAL_ROUTE':
      return 'internal_route';
    case 'INSIGHT_ACTION_KIND_EXTERNAL_URL':
      return 'external_url';
    case 'INSIGHT_ACTION_KIND_LEARN_MORE':
      return 'learn_more';
    case 'INSIGHT_ACTION_KIND_DISMISS':
      return 'dismiss';
    default:
      return 'unspecified';
  }
}

function normalizeInsightEvidenceSource(value: unknown): InsightEvidenceSource {
  switch (value) {
    case 'INSIGHT_EVIDENCE_SOURCE_LIVE_SNAPSHOT':
      return 'live_snapshot';
    case 'INSIGHT_EVIDENCE_SOURCE_ROLLUP_HISTORY':
      return 'rollup_history';
    case 'INSIGHT_EVIDENCE_SOURCE_DEVICE_CAPABILITIES':
      return 'device_capabilities';
    case 'INSIGHT_EVIDENCE_SOURCE_PROVIDER_METADATA':
      return 'provider_metadata';
    case 'INSIGHT_EVIDENCE_SOURCE_MODEL_OUTPUT':
      return 'model_output';
    case 'INSIGHT_EVIDENCE_SOURCE_RULE_ENGINE':
      return 'rule_engine';
    case 'INSIGHT_EVIDENCE_SOURCE_USER_CONTEXT':
      return 'user_context';
    default:
      return 'unspecified';
  }
}

function normalizeString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function normalizeNumber(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function normalizeInt(value: unknown): number {
  return Number.isInteger(value) ? Number(value) : 0;
}

function normalizeRecord(value: unknown): Record<string, unknown> | undefined {
  const normalized = normalizeProtoValue(value);
  if (!normalized || typeof normalized !== 'object' || Array.isArray(normalized)) {
    return undefined;
  }
  return normalized as Record<string, unknown>;
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
    const kind = record.kind;
    return normalizeProtoValue(record[kind]);
  }

  const out: Record<string, unknown> = {};
  for (const [key, nested] of Object.entries(record)) {
    out[key] = normalizeProtoValue(nested);
  }
  return out;
}
