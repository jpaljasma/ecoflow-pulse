import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import grpc from '@grpc/grpc-js';
import protoLoader from '@grpc/proto-loader';

export type WeatherUnitSystem = 'metric' | 'imperial';

export type WeatherRequest = {
  latitude: number;
  longitude: number;
  unitSystem: WeatherUnitSystem;
  panelTiltDegrees?: number;
  panelAzimuthDegrees?: number;
  timezone: string;
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
};

export type WeatherMetricValue = {
  raw: number | null;
  corrected: number | null;
  unit?: string;
};

export type WeatherPoint = {
  timestampIso: string;
  weatherCode: number | null;
  weatherLabel: string | null;
  temperature2m?: WeatherMetricValue;
  windSpeed10m?: WeatherMetricValue;
  windDirection10mDegrees?: number | null;
  windDirectionErrorDegrees?: number | null;
  precipitation?: WeatherMetricValue;
  cloudCover?: WeatherMetricValue;
  visibility?: WeatherMetricValue;
  sunshineDurationSeconds?: number | null;
  shortwaveRadiation?: WeatherMetricValue;
  uvIndex?: WeatherMetricValue;
  globalTiltedIrradiance?: WeatherMetricValue;
};

export type WeatherDailyPoint = {
  dateIso: string;
  weatherCode: number | null;
  weatherLabel: string | null;
  sunriseIso?: string | null;
  sunsetIso?: string | null;
  daylightDurationSeconds?: number | null;
  sunshineDurationSeconds?: number | null;
  shortwaveRadiationSum?: WeatherMetricValue;
  uvIndexMax?: WeatherMetricValue;
};

export type WeatherForecastResponse = {
  forecast: {
    issuedAtUnixMs: string;
    timezone: string;
    unitSystem: WeatherUnitSystem;
    panelTiltDegrees?: number | null;
    panelAzimuthDegrees?: number | null;
    provenance: {
      source: 'open_meteo';
      modelSelection: 'best_match';
      actualSource: 'past_days';
    };
    current: WeatherPoint;
    hourly: WeatherPoint[];
    daily: WeatherDailyPoint[];
  };
};

export type WeatherYesterdayVerificationResponse = {
  verification: {
    issuedAtUnixMs: string;
    timezone: string;
    verificationSource: 'snapshot' | 'previous_runs';
    provenance: {
      source: 'open_meteo';
      modelSelection: 'best_match';
      actualSource: 'past_days';
      verificationSource: 'snapshot' | 'previous_runs';
    };
    summary: {
      comparedHours: number;
      matchedHours: number;
      meanAbsoluteTemperatureError?: number | null;
      meanAbsoluteWindSpeedError?: number | null;
      meanAbsoluteCloudCoverError?: number | null;
      meanAbsoluteVisibilityError?: number | null;
      meanAbsoluteUvIndexError?: number | null;
      meanAbsoluteRadiationError?: number | null;
    };
    hours: Array<{
      timestampIso: string;
      forecast: WeatherPoint;
      actual: WeatherPoint;
      error: {
        temperature2m?: number | null;
        windSpeed10m?: number | null;
        cloudCover?: number | null;
        visibility?: number | null;
        uvIndex?: number | null;
        shortwaveRadiation?: number | null;
        windDirection?: number | null;
      };
    }>;
  };
};

export interface WeatherClient {
  get7DayForecast(input: WeatherRequest): Promise<WeatherForecastResponse>;
  getYesterdayVerification(input: WeatherRequest): Promise<WeatherYesterdayVerificationResponse>;
  close(): void;
}

type GrpcUnaryMethod = (
  request: Record<string, unknown>,
  metadata: grpc.Metadata,
  options: grpc.CallOptions,
  callback: (error: grpc.ServiceError | null, response?: unknown) => void
) => void;

type GrpcWeatherClient = {
  Get7DayForecast: GrpcUnaryMethod;
  GetYesterdayVerification: GrpcUnaryMethod;
  close: () => void;
};

type WeatherProto = {
  pulse: {
    weather: {
      v1: {
        WeatherService: new (
          address: string,
          credentials: grpc.ChannelCredentials,
          options?: Record<string, unknown>
        ) => GrpcWeatherClient;
      };
    };
  };
};

type WrappedNumber = { value?: unknown } | number | null | undefined;
type RawCondition = { weatherCode?: unknown; weatherText?: unknown } | null | undefined;
type RawForecastValueSet = {
  temperature?: WrappedNumber;
  windSpeed?: WrappedNumber;
  windDirectionDegrees?: WrappedNumber;
  precipitation?: WrappedNumber;
  cloudCover?: WrappedNumber;
  visibility?: WrappedNumber;
  sunshineDurationSeconds?: WrappedNumber;
  shortwaveRadiation?: WrappedNumber;
  uvIndex?: WrappedNumber;
  globalTiltedIrradiance?: WrappedNumber;
} | null | undefined;
type RawDailyValueSet = {
  sunshineDurationSeconds?: WrappedNumber;
  shortwaveRadiationSum?: WrappedNumber;
  uvIndexMax?: WrappedNumber;
} | null | undefined;
type RawHourlyForecastPoint = {
  timeUnixMs?: unknown;
  condition?: RawCondition;
  raw?: RawForecastValueSet;
  corrected?: RawForecastValueSet;
};
type RawDailyForecastPoint = {
  dateUnixMs?: unknown;
  condition?: RawCondition;
  sunriseUnixMs?: unknown;
  sunsetUnixMs?: unknown;
  daylightDurationSeconds?: WrappedNumber;
  raw?: RawDailyValueSet;
  corrected?: RawDailyValueSet;
};
type RawForecastResponse = {
  provenance?: {
    source?: unknown;
    modelSelection?: unknown;
    actualSource?: unknown;
    timezone?: unknown;
    issuedAtUnixMs?: unknown;
  };
  unitSystem?: unknown;
  hourly?: unknown;
  daily?: unknown;
};
type RawVerificationHour = {
  timeUnixMs?: unknown;
  forecastCondition?: RawCondition;
  actualCondition?: RawCondition;
  forecastRaw?: RawForecastValueSet;
  forecastCorrected?: RawForecastValueSet;
  actual?: RawForecastValueSet;
};
type RawVerificationSummary = {
  temperature?: { meanAbsoluteError?: WrappedNumber };
  windSpeed?: { meanAbsoluteError?: WrappedNumber };
  cloudCover?: { meanAbsoluteError?: WrappedNumber };
  visibility?: { meanAbsoluteError?: WrappedNumber };
  uvIndex?: { meanAbsoluteError?: WrappedNumber };
  shortwaveRadiation?: { meanAbsoluteError?: WrappedNumber };
};
type RawVerificationResponse = {
  provenance?: {
    source?: unknown;
    modelSelection?: unknown;
    actualSource?: unknown;
    verificationSource?: unknown;
    timezone?: unknown;
    issuedAtUnixMs?: unknown;
  };
  hourly?: unknown;
  summary?: RawVerificationSummary;
};

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, '../../../../');
const protoRoot = path.join(projectRoot, 'proto');
const weatherProtoPath = path.join(protoRoot, 'pulse/weather/v1/weather.proto');

export function createWeatherClient(address: string): WeatherClient {
  let client: GrpcWeatherClient | null = null;

  function ensureClient(): GrpcWeatherClient {
    if (client) {
      return client;
    }
    if (!fs.existsSync(weatherProtoPath)) {
      throw new Error(`weather proto not found at ${weatherProtoPath}`);
    }
    const packageDefinition = protoLoader.loadSync(weatherProtoPath, {
      keepCase: false,
      longs: String,
      enums: String,
      defaults: true,
      oneofs: true,
      includeDirs: [protoRoot]
    });
    const weatherProto = grpc.loadPackageDefinition(packageDefinition) as unknown as WeatherProto;
    client = new weatherProto.pulse.weather.v1.WeatherService(
      address,
      grpc.credentials.createInsecure()
    );
    return client;
  }

  async function unaryCall<T>(
    method: GrpcUnaryMethod,
    request: Record<string, unknown>,
    input: Pick<WeatherRequest, 'authHeader' | 'requestID' | 'deadlineMs'>
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

  return {
    async get7DayForecast(input) {
      const grpcClient = ensureClient();
      const response = await unaryCall<RawForecastResponse>(
        grpcClient.Get7DayForecast.bind(grpcClient),
        buildGrpcRequest(input),
        input
      );
      return normalizeForecastResponse(response, input);
    },
    async getYesterdayVerification(input) {
      const grpcClient = ensureClient();
      const response = await unaryCall<RawVerificationResponse>(
        grpcClient.GetYesterdayVerification.bind(grpcClient),
        buildGrpcRequest(input),
        input
      );
      return normalizeVerificationResponse(response, input);
    },
    close() {
      client?.close();
      client = null;
    }
  };
}

function buildGrpcRequest(input: WeatherRequest) {
  const location: Record<string, unknown> = {
    latitude: input.latitude,
    longitude: input.longitude,
    unitSystem: input.unitSystem === 'imperial' ? 2 : 1,
    timezone: input.timezone
  };
  if (typeof input.panelTiltDegrees === 'number') {
    location.panelTiltDegrees = { value: input.panelTiltDegrees };
  }
  if (typeof input.panelAzimuthDegrees === 'number') {
    location.panelAzimuthDegrees = { value: input.panelAzimuthDegrees };
  }
  return { location };
}

function normalizeForecastResponse(
  response: RawForecastResponse,
  input: WeatherRequest
): WeatherForecastResponse {
  const unitSystem = normalizeUnitSystem(response.unitSystem);
  const timezone = stringOrFallback(response.provenance?.timezone, input.timezone);
  const hourly = asArray<RawHourlyForecastPoint>(response.hourly).map((point) =>
    normalizeWeatherPoint(point, unitSystem)
  );
  const current = selectCurrentPoint(hourly);
  return {
    forecast: {
      issuedAtUnixMs: longString(response.provenance?.issuedAtUnixMs),
      timezone,
      unitSystem,
      panelTiltDegrees: input.panelTiltDegrees ?? null,
      panelAzimuthDegrees: input.panelAzimuthDegrees ?? null,
      provenance: {
        source: 'open_meteo',
        modelSelection: 'best_match',
        actualSource: 'past_days'
      },
      current,
      hourly,
      daily: asArray<RawDailyForecastPoint>(response.daily).map((point) =>
        normalizeDailyPoint(point)
      )
    }
  };
}

function normalizeVerificationResponse(
  response: RawVerificationResponse,
  input: WeatherRequest
): WeatherYesterdayVerificationResponse {
  const timezone = stringOrFallback(response.provenance?.timezone, input.timezone);
  const verificationSource = normalizeVerificationSource(response.provenance?.verificationSource);
  const hours = asArray<RawVerificationHour>(response.hourly).map((point) =>
    normalizeVerificationHour(point, input.unitSystem)
  );
  return {
    verification: {
      issuedAtUnixMs: longString(response.provenance?.issuedAtUnixMs),
      timezone,
      verificationSource,
      provenance: {
        source: 'open_meteo',
        modelSelection: 'best_match',
        actualSource: 'past_days',
        verificationSource
      },
      summary: {
        comparedHours: hours.length,
        matchedHours: hours.length,
        meanAbsoluteTemperatureError: unwrapNumber(response.summary?.temperature?.meanAbsoluteError),
        meanAbsoluteWindSpeedError: unwrapNumber(response.summary?.windSpeed?.meanAbsoluteError),
        meanAbsoluteCloudCoverError: unwrapNumber(response.summary?.cloudCover?.meanAbsoluteError),
        meanAbsoluteVisibilityError: unwrapNumber(response.summary?.visibility?.meanAbsoluteError),
        meanAbsoluteUvIndexError: unwrapNumber(response.summary?.uvIndex?.meanAbsoluteError),
        meanAbsoluteRadiationError: unwrapNumber(response.summary?.shortwaveRadiation?.meanAbsoluteError)
      },
      hours
    }
  };
}

function normalizeVerificationHour(
  point: RawVerificationHour,
  unitSystem: WeatherUnitSystem
): WeatherYesterdayVerificationResponse['verification']['hours'][number] {
  const forecast = normalizeVerificationPoint(
    point.timeUnixMs,
    point.forecastCondition,
    point.forecastRaw,
    point.forecastCorrected,
    unitSystem
  );
  const actual = normalizeActualPoint(point.timeUnixMs, point.actualCondition, point.actual, unitSystem);
  return {
    timestampIso: forecast.timestampIso,
    forecast,
    actual,
    error: {
      temperature2m: difference(actual.temperature2m?.raw, forecast.temperature2m?.raw),
      windSpeed10m: difference(actual.windSpeed10m?.raw, forecast.windSpeed10m?.raw),
      cloudCover: difference(actual.cloudCover?.raw, forecast.cloudCover?.raw),
      visibility: difference(actual.visibility?.raw, forecast.visibility?.raw),
      uvIndex: difference(actual.uvIndex?.raw, forecast.uvIndex?.raw),
      shortwaveRadiation: difference(actual.shortwaveRadiation?.raw, forecast.shortwaveRadiation?.raw),
      windDirection: circularDifference(actual.windDirection10mDegrees, forecast.windDirection10mDegrees)
    }
  };
}

function normalizeWeatherPoint(
  point: RawHourlyForecastPoint,
  unitSystem: WeatherUnitSystem
): WeatherPoint {
  return normalizeVerificationPoint(
    point.timeUnixMs,
    point.condition,
    point.raw,
    point.corrected,
    unitSystem
  );
}

function normalizeVerificationPoint(
  timeUnixMs: unknown,
  condition: RawCondition,
  raw: RawForecastValueSet,
  corrected: RawForecastValueSet,
  unitSystem: WeatherUnitSystem
): WeatherPoint {
  return {
    timestampIso: unixMsToISO(timeUnixMs),
    weatherCode: numberOrNull(condition?.weatherCode),
    weatherLabel: stringOrNull(condition?.weatherText),
    temperature2m: metricValue(raw?.temperature, corrected?.temperature, temperatureUnit(unitSystem)),
    windSpeed10m: metricValue(raw?.windSpeed, corrected?.windSpeed, windSpeedUnit(unitSystem)),
    windDirection10mDegrees: unwrapNumber(corrected?.windDirectionDegrees ?? raw?.windDirectionDegrees),
    precipitation: metricValue(raw?.precipitation, corrected?.precipitation, precipitationUnit(unitSystem)),
    cloudCover: metricValue(raw?.cloudCover, corrected?.cloudCover, '%'),
    visibility: metricValue(raw?.visibility, corrected?.visibility, visibilityUnit(unitSystem)),
    sunshineDurationSeconds: unwrapNumber(corrected?.sunshineDurationSeconds ?? raw?.sunshineDurationSeconds),
    shortwaveRadiation: metricValue(raw?.shortwaveRadiation, corrected?.shortwaveRadiation, 'W/m²'),
    uvIndex: metricValue(raw?.uvIndex, corrected?.uvIndex, 'UV'),
    globalTiltedIrradiance: metricValue(
      raw?.globalTiltedIrradiance,
      corrected?.globalTiltedIrradiance,
      'W/m²'
    )
  };
}

function normalizeActualPoint(
  timeUnixMs: unknown,
  condition: RawCondition,
  actual: RawForecastValueSet,
  unitSystem: WeatherUnitSystem
): WeatherPoint {
  return {
    timestampIso: unixMsToISO(timeUnixMs),
    weatherCode: numberOrNull(condition?.weatherCode),
    weatherLabel: stringOrNull(condition?.weatherText),
    temperature2m: metricValue(actual?.temperature, actual?.temperature, temperatureUnit(unitSystem)),
    windSpeed10m: metricValue(actual?.windSpeed, actual?.windSpeed, windSpeedUnit(unitSystem)),
    windDirection10mDegrees: unwrapNumber(actual?.windDirectionDegrees),
    precipitation: metricValue(actual?.precipitation, actual?.precipitation, precipitationUnit(unitSystem)),
    cloudCover: metricValue(actual?.cloudCover, actual?.cloudCover, '%'),
    visibility: metricValue(actual?.visibility, actual?.visibility, visibilityUnit(unitSystem)),
    sunshineDurationSeconds: unwrapNumber(actual?.sunshineDurationSeconds),
    shortwaveRadiation: metricValue(actual?.shortwaveRadiation, actual?.shortwaveRadiation, 'W/m²'),
    uvIndex: metricValue(actual?.uvIndex, actual?.uvIndex, 'UV'),
    globalTiltedIrradiance: metricValue(actual?.globalTiltedIrradiance, actual?.globalTiltedIrradiance, 'W/m²')
  };
}

function normalizeDailyPoint(point: RawDailyForecastPoint): WeatherDailyPoint {
  return {
    dateIso: unixMsToDateISO(point.dateUnixMs),
    weatherCode: numberOrNull(point.condition?.weatherCode),
    weatherLabel: stringOrNull(point.condition?.weatherText),
    sunriseIso: unixMsToISO(point.sunriseUnixMs),
    sunsetIso: unixMsToISO(point.sunsetUnixMs),
    daylightDurationSeconds: unwrapNumber(point.daylightDurationSeconds),
    sunshineDurationSeconds: unwrapNumber(point.corrected?.sunshineDurationSeconds ?? point.raw?.sunshineDurationSeconds),
    shortwaveRadiationSum: metricValue(
      point.raw?.shortwaveRadiationSum,
      point.corrected?.shortwaveRadiationSum,
      'MJ/m²'
    ),
    uvIndexMax: metricValue(point.raw?.uvIndexMax, point.corrected?.uvIndexMax, 'UV')
  };
}

function metricValue(raw: WrappedNumber, corrected: WrappedNumber, unit?: string): WeatherMetricValue {
  return {
    raw: unwrapNumber(raw),
    corrected: unwrapNumber(corrected),
    unit
  };
}

function selectCurrentPoint(hourly: WeatherPoint[]): WeatherPoint {
  if (hourly.length === 0) {
    return {
      timestampIso: new Date(0).toISOString(),
      weatherCode: null,
      weatherLabel: null
    };
  }
  const now = Date.now();
  return hourly.reduce((best, candidate) => {
    const bestDelta = Math.abs(Date.parse(best.timestampIso) - now);
    const candidateDelta = Math.abs(Date.parse(candidate.timestampIso) - now);
    return candidateDelta < bestDelta ? candidate : best;
  });
}

function normalizeUnitSystem(value: unknown): WeatherUnitSystem {
  return String(value) === 'UNIT_SYSTEM_IMPERIAL' ? 'imperial' : 'metric';
}

function normalizeVerificationSource(value: unknown): 'snapshot' | 'previous_runs' {
  return String(value) === 'previous_runs' ? 'previous_runs' : 'snapshot';
}

function unwrapNumber(value: WrappedNumber): number | null {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : null;
  }
  if (!value || typeof value !== 'object') {
    return null;
  }
  const inner = 'value' in value ? (value as { value?: unknown }).value : undefined;
  if (typeof inner === 'number') {
    return Number.isFinite(inner) ? inner : null;
  }
  return null;
}

function unixMsToISO(value: unknown): string {
  const parsed = longNumber(value);
  return Number.isFinite(parsed) ? new Date(parsed).toISOString() : new Date(0).toISOString();
}

function unixMsToDateISO(value: unknown): string {
  return unixMsToISO(value).slice(0, 10);
}

function longNumber(value: unknown): number {
  if (typeof value === 'number') {
    return value;
  }
  if (typeof value === 'string') {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : Number.NaN;
  }
  return Number.NaN;
}

function longString(value: unknown): string {
  if (typeof value === 'string' && value.trim()) {
    return value;
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return String(Math.trunc(value));
  }
  return '0';
}

function stringOrFallback(value: unknown, fallback: string): string {
  return typeof value === 'string' && value.trim() ? value.trim() : fallback;
}

function stringOrNull(value: unknown): string | null {
  return typeof value === 'string' && value.trim() ? value.trim() : null;
}

function numberOrNull(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
}

function difference(actual?: number | null, forecast?: number | null): number | null {
  if (typeof actual !== 'number' || typeof forecast !== 'number') {
    return null;
  }
  return actual - forecast;
}

function circularDifference(actual?: number | null, forecast?: number | null): number | null {
  if (typeof actual !== 'number' || typeof forecast !== 'number') {
    return null;
  }
  let diff = actual - forecast;
  while (diff <= -180) {
    diff += 360;
  }
  while (diff > 180) {
    diff -= 360;
  }
  return diff;
}

function asArray<T>(value: unknown): T[] {
  return Array.isArray(value) ? (value as T[]) : [];
}

function temperatureUnit(unitSystem: WeatherUnitSystem): string {
  return unitSystem === 'imperial' ? 'F' : 'C';
}

function windSpeedUnit(unitSystem: WeatherUnitSystem): string {
  return unitSystem === 'imperial' ? 'mph' : 'km/h';
}

function precipitationUnit(unitSystem: WeatherUnitSystem): string {
  return unitSystem === 'imperial' ? 'in' : 'mm';
}

function visibilityUnit(unitSystem: WeatherUnitSystem): string {
  return unitSystem === 'imperial' ? 'mi' : 'm';
}
