import { z } from 'zod';
import { requestJson } from '@/shared/api/restClient';
import { getWeatherCodeIcon, getWeatherCodeLabel } from '@/features/weather/model';
import type {
  SolarOutlook,
  WeatherDailyPoint,
  WeatherForecast,
  WeatherPoint,
  WeatherYesterdayHour,
  WeatherYesterdayVerification
} from '@/features/weather/model';

const WeatherMetricValueSchema = z.object({
  raw: z.number().nullable().optional(),
  corrected: z.number().nullable().optional(),
  unit: z.string().optional()
}).passthrough();

type WeatherMetricValueApiPayload = {
  raw?: number | null;
  corrected?: number | null;
  unit?: string;
};

const WeatherPointSchema = z.object({
  timestampIso: z.string(),
  weatherCode: z.number().nullable().optional(),
  weatherLabel: z.string().nullable().optional(),
  weatherIcon: z.string().optional(),
  temperature2m: WeatherMetricValueSchema.optional(),
  windSpeed10m: WeatherMetricValueSchema.optional(),
  windDirection10mDegrees: z.number().nullable().optional(),
  windDirectionErrorDegrees: z.number().nullable().optional(),
  precipitation: WeatherMetricValueSchema.optional(),
  cloudCover: WeatherMetricValueSchema.optional(),
  visibility: WeatherMetricValueSchema.optional(),
  sunshineDurationSeconds: z.number().nullable().optional(),
  shortwaveRadiation: WeatherMetricValueSchema.optional(),
  uvIndex: WeatherMetricValueSchema.optional(),
  globalTiltedIrradiance: WeatherMetricValueSchema.optional()
}).passthrough();

type WeatherPointApiPayload = {
  timestampIso: string;
  weatherCode?: number | null;
  weatherLabel?: string | null;
  weatherIcon?: string;
  temperature2m?: WeatherMetricValueApiPayload;
  windSpeed10m?: WeatherMetricValueApiPayload;
  windDirection10mDegrees?: number | null;
  windDirectionErrorDegrees?: number | null;
  precipitation?: WeatherMetricValueApiPayload;
  cloudCover?: WeatherMetricValueApiPayload;
  visibility?: WeatherMetricValueApiPayload;
  sunshineDurationSeconds?: number | null;
  shortwaveRadiation?: WeatherMetricValueApiPayload;
  uvIndex?: WeatherMetricValueApiPayload;
  globalTiltedIrradiance?: WeatherMetricValueApiPayload;
};

const WeatherDailyPointSchema = z.object({
  dateIso: z.string(),
  weatherCode: z.number().nullable().optional(),
  weatherLabel: z.string().nullable().optional(),
  weatherIcon: z.string().optional(),
  sunriseIso: z.string().nullable().optional(),
  sunsetIso: z.string().nullable().optional(),
  daylightDurationSeconds: z.number().nullable().optional(),
  sunshineDurationSeconds: z.number().nullable().optional(),
  shortwaveRadiationSum: WeatherMetricValueSchema.optional(),
  uvIndexMax: WeatherMetricValueSchema.optional()
}).passthrough();

type WeatherDailyPointApiPayload = {
  dateIso: string;
  weatherCode?: number | null;
  weatherLabel?: string | null;
  weatherIcon?: string;
  sunriseIso?: string | null;
  sunsetIso?: string | null;
  daylightDurationSeconds?: number | null;
  sunshineDurationSeconds?: number | null;
  shortwaveRadiationSum?: WeatherMetricValueApiPayload;
  uvIndexMax?: WeatherMetricValueApiPayload;
};

const WeatherForecastSchema = z.object({
  issuedAtUnixMs: z.string(),
  timezone: z.string(),
  unitSystem: z.enum(['metric', 'imperial']),
  panelTiltDegrees: z.number().nullable().optional(),
  panelAzimuthDegrees: z.number().nullable().optional(),
  provenance: z.object({
    source: z.literal('open_meteo'),
    modelSelection: z.literal('best_match'),
    actualSource: z.literal('past_days').optional()
  }),
  current: WeatherPointSchema,
  hourly: z.array(WeatherPointSchema),
  daily: z.array(WeatherDailyPointSchema)
}).passthrough();

type WeatherForecastApiPayload = {
  issuedAtUnixMs: string;
  timezone: string;
  unitSystem: 'metric' | 'imperial';
  panelTiltDegrees?: number | null;
  panelAzimuthDegrees?: number | null;
  provenance: {
    source: 'open_meteo';
    modelSelection: 'best_match';
    actualSource?: 'past_days';
  };
  current: WeatherPointApiPayload;
  hourly: WeatherPointApiPayload[];
  daily: WeatherDailyPointApiPayload[];
};

const WeatherYesterdayHourSchema = z.object({
  timestampIso: z.string(),
  forecast: WeatherPointSchema,
  actual: WeatherPointSchema,
  error: z.object({
    temperature2m: z.number().nullable().optional(),
    windSpeed10m: z.number().nullable().optional(),
    cloudCover: z.number().nullable().optional(),
    visibility: z.number().nullable().optional(),
    uvIndex: z.number().nullable().optional(),
    shortwaveRadiation: z.number().nullable().optional(),
    windDirection: z.number().nullable().optional()
  }).passthrough()
}).passthrough();

type WeatherYesterdayHourApiPayload = {
  timestampIso: string;
  forecast: WeatherPointApiPayload;
  actual: WeatherPointApiPayload;
  error: {
    temperature2m?: number | null;
    windSpeed10m?: number | null;
    cloudCover?: number | null;
    visibility?: number | null;
    uvIndex?: number | null;
    shortwaveRadiation?: number | null;
    windDirection?: number | null;
  };
};

const WeatherYesterdayVerificationSchema = z.object({
  issuedAtUnixMs: z.string(),
  timezone: z.string(),
  verificationSource: z.enum(['snapshot', 'previous_runs']),
  provenance: z.object({
    source: z.literal('open_meteo'),
    modelSelection: z.literal('best_match'),
    actualSource: z.literal('past_days'),
    verificationSource: z.enum(['snapshot', 'previous_runs'])
  }),
  summary: z.object({
    comparedHours: z.number().int().nonnegative(),
    matchedHours: z.number().int().nonnegative(),
    meanAbsoluteTemperatureError: z.number().nullable().optional(),
    meanAbsoluteWindSpeedError: z.number().nullable().optional(),
    meanAbsoluteCloudCoverError: z.number().nullable().optional(),
    meanAbsoluteVisibilityError: z.number().nullable().optional(),
    meanAbsoluteUvIndexError: z.number().nullable().optional(),
    meanAbsoluteRadiationError: z.number().nullable().optional()
  }),
  hours: z.array(WeatherYesterdayHourSchema)
}).passthrough();

type WeatherYesterdayVerificationApiPayload = {
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
  hours: WeatherYesterdayHourApiPayload[];
};

export type WeatherForecastResponse = {
  forecast: WeatherForecast;
};

export type WeatherYesterdayVerificationResponse = {
  verification: WeatherYesterdayVerification;
};

export type SolarOutlookResponse = {
  outlook: SolarOutlook;
};

export async function fetchWeatherForecast(token?: string): Promise<WeatherForecastResponse> {
  const data = await requestJson<unknown>('/api/v1/weather/forecast', { token });
  const parsed = z.object({ forecast: WeatherForecastSchema }).parse(data);
  return {
    forecast: normalizeForecast(parsed.forecast)
  };
}

export async function fetchWeatherYesterdayVerification(
  token?: string
): Promise<WeatherYesterdayVerificationResponse> {
  const data = await requestJson<unknown>('/api/v1/weather/yesterday', { token });
  const parsed = z.object({ verification: WeatherYesterdayVerificationSchema }).parse(data);
  return {
    verification: normalizeYesterdayVerification(parsed.verification)
  };
}

export async function fetchSolarOutlook(token?: string): Promise<SolarOutlookResponse> {
  const data = await requestJson<unknown>('/api/v1/solar/outlook', { token });
  const parsed = z.object({
    outlook: z.object({
      scope: z
        .object({
          mode: z.string(),
          deviceId: z.string().optional(),
          resolvedDeviceIds: z.array(z.string()).optional().default([])
        })
        .optional(),
      provenance: z
        .object({
          forecastSource: z.string(),
          forecastModel: z.string(),
          servedVariant: z.string().optional().default('baseline'),
          baselineModel: z.string().optional().default(''),
          calibrationApplied: z.boolean().optional().default(false),
          calibrationSampleCount: z.number().int().nonnegative().optional().default(0),
          calibrationUpdatedAtUnixMs: z.string().optional(),
          sameDayCurtailmentApplied: z.boolean().optional().default(false),
          sameDayCurtailmentReason: z.string().optional(),
          actualsSource: z.string(),
          weatherSource: z.string(),
          weatherModelSelection: z.string(),
          timezone: z.string(),
          canonicalLocationKey: z.string(),
          issuedAtUnixMs: z.string(),
          refreshedAtUnixMs: z.string()
        })
        .optional(),
      capacity: z.object({
        estimatedPeakWatts: z.number().optional(),
        observedPvWatts: z.number().optional(),
        method: z.enum([
          'rolling_observed_p95',
          'rolling_observed_p95_and_irradiance',
          'live_pv_and_irradiance',
          'live_pv_only',
          'input_ceiling',
          'unavailable'
        ])
      }),
      today: z
        .object({
          dateIso: z.string(),
          actualGeneratedKwh: z.number().optional(),
          forecastRemainingKwh: z.number().optional(),
          forecastTotalKwh: z.number().optional(),
          estimatedPeakWatts: z.number().optional(),
          peakTimeIso: z.string().optional(),
          confidence: z.enum(['low', 'medium', 'high'])
        })
        .optional(),
      daily: z.array(
        z.object({
          dateIso: z.string(),
          actualGeneratedKwh: z.number().optional(),
          forecastRemainingKwh: z.number().optional(),
          forecastTotalKwh: z.number().optional(),
          estimatedPeakWatts: z.number().optional(),
          peakTimeIso: z.string().optional(),
          confidence: z.enum(['low', 'medium', 'high'])
        })
      ),
      next24Hours: z
        .array(
          z.object({
            timestampIso: z.string(),
            actualGeneratedWh: z.number().optional(),
            forecastGeneratedWh: z.number().optional(),
            estimatedPeakWatts: z.number().optional(),
            shortwaveRadiation: z.number().optional(),
            globalTiltedIrradiance: z.number().optional(),
            cloudCover: z.number().optional(),
            confidence: z.enum(['low', 'medium', 'high'])
          })
        )
        .optional()
        .default([])
    })
  }).parse(data);
  return {
    outlook: {
      ...parsed.outlook,
      provenance: parsed.outlook.provenance
        ? {
            ...parsed.outlook.provenance,
            servedVariant: parsed.outlook.provenance.servedVariant || 'baseline',
            baselineModel:
              parsed.outlook.provenance.baselineModel || parsed.outlook.provenance.forecastModel,
            calibrationApplied: parsed.outlook.provenance.calibrationApplied,
            calibrationSampleCount: parsed.outlook.provenance.calibrationSampleCount,
            calibrationUpdatedAtUnixMs: parsed.outlook.provenance.calibrationUpdatedAtUnixMs,
            sameDayCurtailmentApplied: parsed.outlook.provenance.sameDayCurtailmentApplied,
            sameDayCurtailmentReason: parsed.outlook.provenance.sameDayCurtailmentReason
          }
        : undefined,
      today: parsed.outlook.today
        ? {
            dateIso: normalizeDateIso(parsed.outlook.today.dateIso),
            peakWatts: parsed.outlook.today.estimatedPeakWatts,
            energyKwh: parsed.outlook.today.forecastTotalKwh,
            actualSoFarKwh: parsed.outlook.today.actualGeneratedKwh,
            forecastRemainingKwh: parsed.outlook.today.forecastRemainingKwh,
            peakHourIso: parsed.outlook.today.peakTimeIso,
            irradianceSource: 'unavailable'
          }
        : undefined,
      daily: parsed.outlook.daily.map((day) => ({
        dateIso: normalizeDateIso(day.dateIso),
        peakWatts: day.estimatedPeakWatts,
        energyKwh: day.forecastTotalKwh,
        actualSoFarKwh: day.actualGeneratedKwh,
        forecastRemainingKwh: day.forecastRemainingKwh,
        peakHourIso: day.peakTimeIso,
        irradianceSource: 'unavailable'
      }))
    }
  };
}

function normalizeForecast(forecast: WeatherForecastApiPayload): WeatherForecast {
  return {
    ...forecast,
    current: normalizeWeatherPoint(forecast.current),
    hourly: forecast.hourly.map(normalizeWeatherPoint),
    daily: forecast.daily.map(normalizeDailyPoint),
    provenance: {
      source: forecast.provenance.source,
      modelSelection: forecast.provenance.modelSelection,
      actualSource: forecast.provenance.actualSource
    }
  };
}

function normalizeYesterdayVerification(
  verification: WeatherYesterdayVerificationApiPayload
): WeatherYesterdayVerification {
  return {
    ...verification,
    hours: verification.hours.map(normalizeYesterdayHour)
  };
}

function normalizeDailyPoint(day: WeatherDailyPointApiPayload): WeatherDailyPoint {
  return {
    ...day,
    dateIso: normalizeDateIso(day.dateIso),
    weatherLabel: day.weatherLabel ?? getWeatherCodeLabel(day.weatherCode),
    weatherIcon: (day.weatherIcon ?? getWeatherCodeIcon(day.weatherCode)) as WeatherDailyPoint['weatherIcon']
  };
}

function normalizeYesterdayHour(hour: WeatherYesterdayHourApiPayload): WeatherYesterdayHour {
  return {
    ...hour,
    forecast: normalizeWeatherPoint(hour.forecast),
    actual: normalizeWeatherPoint(hour.actual)
  };
}

function normalizeWeatherPoint(point: WeatherPointApiPayload): WeatherPoint {
  return {
    ...point,
    weatherLabel: point.weatherLabel ?? getWeatherCodeLabel(point.weatherCode),
    weatherIcon: (point.weatherIcon ?? getWeatherCodeIcon(point.weatherCode)) as WeatherPoint['weatherIcon']
  };
}

function normalizeDateIso(value: string): string {
  return value.includes('T') ? value.slice(0, 10) : value;
}
