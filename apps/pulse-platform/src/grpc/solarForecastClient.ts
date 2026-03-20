import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import grpc from '@grpc/grpc-js';
import protoLoader from '@grpc/proto-loader';

export type SolarOutlookRequest = {
  latitude: number;
  longitude: number;
  timezone: string;
  panelTiltDegrees?: number;
  panelAzimuthDegrees?: number;
  deviceId?: string;
  useAllDevices: boolean;
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
};

export type SolarCapacityEstimate = {
  estimatedPeakWatts?: number;
  observedPvWatts?: number;
  method: 'live_pv_and_irradiance' | 'live_pv_only' | 'input_ceiling' | 'unavailable';
};

export type SolarGenerationDay = {
  dateIso: string;
  actualGeneratedKwh?: number;
  forecastRemainingKwh?: number;
  forecastTotalKwh?: number;
  estimatedPeakWatts?: number;
  peakTimeIso?: string;
  confidence: 'low' | 'medium' | 'high';
};

export type SolarGenerationPoint = {
  timestampIso: string;
  actualGeneratedWh?: number;
  forecastGeneratedWh?: number;
  estimatedPeakWatts?: number;
  shortwaveRadiation?: number;
  globalTiltedIrradiance?: number;
  cloudCover?: number;
  confidence: 'low' | 'medium' | 'high';
};

export type SolarOutlookResponse = {
  outlook: {
    scope: {
      mode: string;
      deviceId?: string;
      resolvedDeviceIds: string[];
    };
    provenance: {
      forecastSource: string;
      forecastModel: string;
      servedVariant: string;
      baselineModel: string;
      calibrationApplied: boolean;
      calibrationSampleCount: number;
      calibrationUpdatedAtUnixMs?: string;
      actualsSource: string;
      weatherSource: string;
      weatherModelSelection: string;
      timezone: string;
      canonicalLocationKey: string;
      issuedAtUnixMs: string;
      refreshedAtUnixMs: string;
    };
    capacity: SolarCapacityEstimate;
    today?: SolarGenerationDay;
    daily: SolarGenerationDay[];
    next24Hours: SolarGenerationPoint[];
  };
};

export interface SolarForecastClient {
  getSolarOutlook(input: SolarOutlookRequest): Promise<SolarOutlookResponse>;
  close(): void;
}

type GrpcUnaryMethod = (
  request: Record<string, unknown>,
  metadata: grpc.Metadata,
  options: grpc.CallOptions,
  callback: (error: grpc.ServiceError | null, response?: unknown) => void
) => void;

type GrpcSolarForecastClient = {
  GetSolarOutlook: GrpcUnaryMethod;
  close: () => void;
};

type SolarForecastProto = {
  pulse: {
    solarforecast: {
      v1: {
        SolarForecastService: new (
          address: string,
          credentials: grpc.ChannelCredentials,
          options?: Record<string, unknown>
        ) => GrpcSolarForecastClient;
      };
    };
  };
};

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, '../../../../');
const protoRoot = path.join(projectRoot, 'proto');
const protoPath = path.join(protoRoot, 'pulse/solarforecast/v1/solar_forecast.proto');

export function createSolarForecastClient(address: string): SolarForecastClient {
  let client: GrpcSolarForecastClient | null = null;

  function ensureClient(): GrpcSolarForecastClient {
    if (client) {
      return client;
    }
    if (!fs.existsSync(protoPath)) {
      throw new Error(`solar forecast proto not found at ${protoPath}`);
    }
    const packageDefinition = protoLoader.loadSync(protoPath, {
      keepCase: false,
      longs: String,
      enums: String,
      defaults: true,
      oneofs: true,
      includeDirs: [protoRoot]
    });
    const proto = grpc.loadPackageDefinition(packageDefinition) as unknown as SolarForecastProto;
    client = new proto.pulse.solarforecast.v1.SolarForecastService(
      address,
      grpc.credentials.createInsecure()
    );
    return client;
  }

  return {
    async getSolarOutlook(input) {
      const response = await unaryCall<Record<string, unknown>>(
        ensureClient().GetSolarOutlook.bind(ensureClient()),
        {
          location: {
            latitude: input.latitude,
            longitude: input.longitude,
            timezone: input.timezone,
            panelTiltDegrees:
              input.panelTiltDegrees === undefined ? undefined : { value: input.panelTiltDegrees },
            panelAzimuthDegrees:
              input.panelAzimuthDegrees === undefined
                ? undefined
                : { value: input.panelAzimuthDegrees }
          },
          deviceId: input.deviceId ?? '',
          useAllDevices: input.useAllDevices
        },
        input
      );
      return {
        outlook: normalizeSolarOutlookResponse(response)
      };
    },
    close() {
      client?.close();
      client = null;
    }
  };
}

async function unaryCall<T>(
  method: GrpcUnaryMethod,
  request: Record<string, unknown>,
  input: Pick<SolarOutlookRequest, 'authHeader' | 'requestID' | 'deadlineMs'>
): Promise<T> {
  const metadata = new grpc.Metadata();
  if (input.authHeader) {
    metadata.set('authorization', input.authHeader);
  }
  if (input.requestID) {
    metadata.set('x-request-id', input.requestID);
  }
  return await new Promise<T>((resolve, reject) => {
    method(
      request,
      metadata,
      {
        deadline: new Date(Date.now() + input.deadlineMs)
      },
      (error, response) => {
        if (error) {
          reject(error);
          return;
        }
        resolve((response ?? {}) as T);
      }
    );
  });
}

function normalizeSolarOutlookResponse(response: Record<string, unknown>): SolarOutlookResponse['outlook'] {
  const scope = toObject(response.scope);
  const provenance = toObject(response.provenance);
  const today = toGenerationDay(response.today);
  const daily = toArray(response.next7Days).map(toGenerationDay).filter(Boolean) as SolarGenerationDay[];
  const next24Hours = toArray(response.next24Hours)
    .map(toGenerationPoint)
    .filter(Boolean) as SolarGenerationPoint[];
  return {
    scope: {
      mode: stringValue(scope.mode),
      deviceId: optionalString(scope.deviceId),
      resolvedDeviceIds: toArray(scope.resolvedDeviceIds).map((value) => stringValue(value)).filter(Boolean)
    },
    provenance: {
      forecastSource: stringValue(provenance.forecastSource),
      forecastModel: stringValue(provenance.forecastModel),
      servedVariant: stringValue(provenance.servedVariant),
      baselineModel: stringValue(provenance.baselineModel),
      calibrationApplied: Boolean(provenance.calibrationApplied),
      calibrationSampleCount: numericValue(provenance.calibrationSampleCount) ?? 0,
      calibrationUpdatedAtUnixMs: optionalUnixMsString(provenance.calibrationUpdatedAtUnixMs),
      actualsSource: stringValue(provenance.actualsSource),
      weatherSource: stringValue(provenance.weatherSource),
      weatherModelSelection: stringValue(provenance.weatherModelSelection),
      timezone: stringValue(provenance.timezone),
      canonicalLocationKey: stringValue(provenance.canonicalLocationKey),
      issuedAtUnixMs: stringValue(provenance.issuedAtUnixMs),
      refreshedAtUnixMs: stringValue(provenance.refreshedAtUnixMs)
    },
    capacity: {
      estimatedPeakWatts: wrappedNumber(toObject(response.capacity).estimatedPeakWatts) ?? undefined,
      observedPvWatts: wrappedNumber(toObject(response.capacity).observedPvWatts) ?? undefined,
      method: (stringValue(toObject(response.capacity).method) ||
        'unavailable') as SolarCapacityEstimate['method']
    },
    today: today ?? undefined,
    daily,
    next24Hours
  };
}

function toGenerationDay(value: unknown): SolarGenerationDay | null {
  const raw = toObject(value);
  if (Object.keys(raw).length === 0) {
    return null;
  }
  return {
    dateIso: unixMsToDateIso(raw.dateUnixMs),
    actualGeneratedKwh: wrappedNumber(raw.actualGeneratedKwh) ?? undefined,
    forecastRemainingKwh: wrappedNumber(raw.forecastRemainingKwh) ?? undefined,
    forecastTotalKwh: wrappedNumber(raw.forecastTotalKwh) ?? undefined,
    estimatedPeakWatts: wrappedNumber(raw.estimatedPeakWatts) ?? undefined,
    peakTimeIso: unixMsToIso(raw.peakTimeUnixMs) ?? undefined,
    confidence: confidenceValue(raw.confidence)
  };
}

function toGenerationPoint(value: unknown): SolarGenerationPoint | null {
  const raw = toObject(value);
  if (Object.keys(raw).length === 0) {
    return null;
  }
  return {
    timestampIso: unixMsToIso(raw.timeUnixMs) ?? new Date(0).toISOString(),
    actualGeneratedWh: wrappedNumber(raw.actualGeneratedWh) ?? undefined,
    forecastGeneratedWh: wrappedNumber(raw.forecastGeneratedWh) ?? undefined,
    estimatedPeakWatts: wrappedNumber(raw.estimatedPeakWatts) ?? undefined,
    shortwaveRadiation: wrappedNumber(raw.shortwaveRadiation) ?? undefined,
    globalTiltedIrradiance: wrappedNumber(raw.globalTiltedIrradiance) ?? undefined,
    cloudCover: wrappedNumber(raw.cloudCover) ?? undefined,
    confidence: confidenceValue(raw.confidence)
  };
}

function confidenceValue(value: unknown): 'low' | 'medium' | 'high' {
  switch (stringValue(value)) {
    case 'SOLAR_FORECAST_CONFIDENCE_HIGH':
      return 'high';
    case 'SOLAR_FORECAST_CONFIDENCE_MEDIUM':
      return 'medium';
    default:
      return 'low';
  }
}

function unixMsToIso(value: unknown): string | null {
  const parsed = numericValue(value);
  if (parsed === null) {
    return null;
  }
  return new Date(parsed).toISOString();
}

function unixMsToDateIso(value: unknown): string {
  return unixMsToIso(value)?.slice(0, 10) ?? '';
}

function wrappedNumber(value: unknown): number | null {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : null;
  }
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  if (typeof value === 'object' && value !== null && 'value' in value) {
    return wrappedNumber((value as { value?: unknown }).value);
  }
  return null;
}

function numericValue(value: unknown): number | null {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : null;
  }
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function optionalString(value: unknown): string | undefined {
  const normalized = stringValue(value).trim();
  return normalized ? normalized : undefined;
}

function optionalUnixMsString(value: unknown): string | undefined {
  const parsed = numericValue(value);
  if (parsed === null || parsed <= 0) {
    return undefined;
  }
  return String(Math.trunc(parsed));
}

function toObject(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null ? (value as Record<string, unknown>) : {};
}

function toArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}
