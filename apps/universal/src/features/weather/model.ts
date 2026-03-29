import type { ComponentProps } from 'react';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import type { DeviceSummary } from '@/features/devices/api';

export type WeatherUnitSystem = 'metric' | 'imperial';

export type WeatherMetricName =
  | 'temperature2m'
  | 'windSpeed10m'
  | 'cloudCover'
  | 'visibility'
  | 'uvIndex'
  | 'shortwaveRadiation'
  | 'globalTiltedIrradiance';

export type WeatherMetricValue = {
  raw?: number | null;
  corrected?: number | null;
  unit?: string;
};

export type WeatherPoint = {
  timestampIso: string;
  weatherCode?: number | null;
  weatherLabel?: string | null;
  weatherIcon?: ComponentProps<typeof MaterialCommunityIcons>['name'];
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
  weatherCode?: number | null;
  weatherLabel?: string | null;
  weatherIcon?: ComponentProps<typeof MaterialCommunityIcons>['name'];
  sunriseIso?: string | null;
  sunsetIso?: string | null;
  daylightDurationSeconds?: number | null;
  sunshineDurationSeconds?: number | null;
  shortwaveRadiationSum?: WeatherMetricValue;
  uvIndexMax?: WeatherMetricValue;
};

export type WeatherForecast = {
  issuedAtUnixMs: string;
  timezone: string;
  unitSystem: WeatherUnitSystem;
  panelTiltDegrees?: number | null;
  panelAzimuthDegrees?: number | null;
  provenance: {
    source: 'open_meteo';
    modelSelection: 'best_match';
    actualSource?: 'past_days';
  };
  current: WeatherPoint;
  hourly: WeatherPoint[];
  daily: WeatherDailyPoint[];
};

export type WeatherLocationPreference = {
  timezone?: string | null;
  weatherLocationEnabled?: boolean;
  weatherLocation?: {
    label?: string;
    latitude: number;
    longitude: number;
  } | null;
};

export type WeatherDaypart = {
  key: 'morning' | 'day' | 'afternoon' | 'night';
  label: string;
  point?: WeatherPoint;
};

export type WeatherYesterdayHour = {
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
};

export type WeatherYesterdayVerification = {
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
  hours: WeatherYesterdayHour[];
};

export type SolarCapacityEstimate = {
  estimatedPeakWatts?: number;
  observedPvWatts?: number;
  inputCeilingWatts?: number;
  method:
    | 'rolling_observed_p95'
    | 'rolling_observed_p95_and_irradiance'
    | 'rolling_observed_p95_device_share'
    | 'rolling_observed_p95_and_irradiance_device_share'
    | 'input_ceiling'
    | 'input_ceiling_device_share'
    | 'live_pv_and_irradiance'
    | 'live_pv_and_irradiance_device_share'
    | 'live_pv_only'
    | 'live_pv_only_device_share'
    | 'unavailable';
};

export type SolarDayOutlook = {
  dateIso: string;
  peakWatts?: number;
  energyKwh?: number;
  actualSoFarKwh?: number;
  forecastRemainingKwh?: number;
  peakHourIso?: string;
  irradianceSource: 'gti' | 'shortwave_radiation' | 'unavailable';
};

export type SolarOutlook = {
  capacity: SolarCapacityEstimate;
  scope?: {
    mode: string;
    deviceId?: string;
    resolvedDeviceIds: string[];
  };
  provenance?: {
    forecastSource: string;
    forecastModel: string;
    servedVariant: string;
    baselineModel: string;
    calibrationApplied: boolean;
    calibrationSampleCount: number;
    calibrationUpdatedAtUnixMs?: string;
    sameDayCurtailmentApplied: boolean;
    sameDayCurtailmentReason?: string;
    actualsSource: string;
    weatherSource: string;
    weatherModelSelection: string;
    timezone: string;
    canonicalLocationKey: string;
    issuedAtUnixMs: string;
    refreshedAtUnixMs: string;
  };
  today?: SolarDayOutlook;
  daily: SolarDayOutlook[];
  next24Hours?: Array<{
    timestampIso: string;
    actualGeneratedWh?: number;
    forecastGeneratedWh?: number;
    estimatedPeakWatts?: number;
    shortwaveRadiation?: number;
    globalTiltedIrradiance?: number;
    cloudCover?: number;
    confidence: 'low' | 'medium' | 'high';
  }>;
};

export type SolarHistorySnapshot = {
  todayWh: number;
  seriesWh: number[];
};

const MPS_TO_MPH = 2.2369362920544;
const KM_TO_MI = 0.621371192237334;
const MM_TO_IN = 0.03937007874015748;

export function getWeatherCodeLabel(code: number | null | undefined): string {
  switch (code) {
    case 0:
      return 'Clear sky';
    case 1:
      return 'Mainly clear';
    case 2:
      return 'Partly cloudy';
    case 3:
      return 'Overcast';
    case 45:
    case 48:
      return 'Fog';
    case 51:
    case 53:
    case 55:
      return 'Drizzle';
    case 56:
    case 57:
      return 'Freezing drizzle';
    case 61:
    case 63:
    case 65:
      return 'Rain';
    case 66:
    case 67:
      return 'Freezing rain';
    case 71:
    case 73:
    case 75:
    case 77:
      return 'Snow';
    case 80:
    case 81:
    case 82:
      return 'Rain showers';
    case 85:
    case 86:
      return 'Snow showers';
    case 95:
      return 'Thunderstorm';
    case 96:
    case 99:
      return 'Thunderstorm with hail';
    default:
      return code === null || code === undefined ? 'Weather unavailable' : `Weather code ${code}`;
  }
}

export function getWeatherCodeIcon(code: number | null | undefined): ComponentProps<
  typeof MaterialCommunityIcons
>['name'] {
  switch (code) {
    case 0:
      return 'weather-sunny';
    case 1:
    case 2:
      return 'weather-partly-cloudy';
    case 3:
      return 'weather-cloudy';
    case 45:
    case 48:
      return 'weather-fog';
    case 51:
    case 53:
    case 55:
    case 61:
    case 63:
    case 65:
    case 80:
    case 81:
    case 82:
      return 'weather-rainy';
    case 56:
    case 57:
    case 66:
    case 67:
      return 'weather-pouring';
    case 71:
    case 73:
    case 75:
    case 77:
    case 85:
    case 86:
      return 'weather-snowy';
    case 95:
    case 96:
    case 99:
      return 'weather-lightning-rainy';
    default:
      return 'weather-cloudy';
  }
}

export function convertTemperatureC(valueC: number, unitSystem: WeatherUnitSystem): number {
  return unitSystem === 'imperial' ? (valueC * 9) / 5 + 32 : valueC;
}

export function convertWindSpeedMps(valueMps: number, unitSystem: WeatherUnitSystem): number {
  return unitSystem === 'imperial' ? valueMps * MPS_TO_MPH : valueMps;
}

export function convertDistanceM(valueMeters: number, unitSystem: WeatherUnitSystem): number {
  return unitSystem === 'imperial' ? (valueMeters / 1000) * KM_TO_MI : valueMeters / 1000;
}

export function convertPrecipitationMm(valueMm: number, unitSystem: WeatherUnitSystem): number {
  return unitSystem === 'imperial' ? valueMm * MM_TO_IN : valueMm;
}

export function formatSignedDegrees(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  const normalized = ((value % 360) + 360) % 360;
  return `${Math.round(normalized)}°`;
}

export function formatWeatherDayLabel(
  dateIso: string,
  timezone?: string | null,
  locale = 'en-US'
): string {
  const resolvedTimezone = timezone?.trim() || 'UTC';
  try {
    return new Intl.DateTimeFormat(locale, {
      weekday: 'short',
      month: 'short',
      day: 'numeric',
      timeZone: resolvedTimezone
    }).format(new Date(`${dateIso}T12:00:00Z`));
  } catch {
    return dateIso;
  }
}

export function formatRelativeWeatherDayLabel(
  dateIso: string,
  timezone?: string | null,
  now: Date = new Date(),
  locale = 'en-US'
): string {
  const todayIso = getDateIsoInTimezone(now, timezone);
  const diffDays = diffIsoDays(dateIso, todayIso);
  if (diffDays === 0) {
    return 'Today';
  }
  if (diffDays === 1) {
    return 'Tomorrow';
  }
  if (diffDays > 1 && diffDays < 7) {
    try {
      return new Intl.DateTimeFormat(locale, {
        weekday: 'long',
        timeZone: timezone?.trim() || 'UTC'
      }).format(new Date(`${dateIso}T12:00:00Z`));
    } catch {
      return dateIso;
    }
  }
  try {
    return new Intl.DateTimeFormat(locale, {
      month: 'numeric',
      day: 'numeric',
      timeZone: timezone?.trim() || 'UTC'
    }).format(new Date(`${dateIso}T12:00:00Z`));
  } catch {
    return dateIso;
  }
}

export function circularWindDirectionError(actualDegrees: number, expectedDegrees: number): number {
  const raw = ((actualDegrees - expectedDegrees + 540) % 360) - 180;
  return raw === -180 ? 180 : raw;
}

export function formatWeatherValue(
  value: WeatherMetricValue | undefined,
  unitSystem: WeatherUnitSystem,
  metric: WeatherMetricName
): string {
  if (!value) {
    return '—';
  }
  const raw = value.corrected ?? value.raw;
  if (raw === null || raw === undefined || Number.isNaN(raw)) {
    return '—';
  }
  switch (metric) {
    case 'temperature2m':
      return `${convertTemperatureC(raw, unitSystem).toFixed(1)}°${unitSystem === 'imperial' ? 'F' : 'C'}`;
    case 'windSpeed10m':
      return `${convertWindSpeedMps(raw, unitSystem).toFixed(1)} ${unitSystem === 'imperial' ? 'mph' : 'm/s'}`;
    case 'cloudCover':
      return `${Math.round(raw)}%`;
    case 'visibility':
      return `${convertDistanceM(raw, unitSystem).toFixed(1)} ${unitSystem === 'imperial' ? 'mi' : 'km'}`;
    case 'uvIndex':
      return raw.toFixed(1);
    case 'shortwaveRadiation':
    case 'globalTiltedIrradiance':
      return `${Math.round(raw)} W/m2`;
    default:
      return String(raw);
  }
}

export function formatDailyDuration(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined || Number.isNaN(seconds)) {
    return '—';
  }
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.round((seconds % 3600) / 60);
  if (hours <= 0) {
    return `${minutes}m`;
  }
  if (minutes <= 0) {
    return `${hours}h`;
  }
  return `${hours}h ${minutes}m`;
}

export function formatWindSummary(
  speed: WeatherMetricValue | undefined,
  unitSystem: WeatherUnitSystem
): string {
  const speedLabel = formatWeatherValue(speed, unitSystem, 'windSpeed10m');
  if (speedLabel === '—') {
    return 'Wind unavailable';
  }
  return speedLabel;
}

export function formatVisibilityKilometers(value: WeatherMetricValue | undefined): string {
  if (!value) {
    return '—';
  }
  const raw = value.corrected ?? value.raw;
  if (raw === null || raw === undefined || Number.isNaN(raw)) {
    return '—';
  }
  return `${convertDistanceM(raw, 'metric').toFixed(1)} km`;
}

export function getForecastDayparts(
  hourly: WeatherPoint[],
  timezone?: string | null,
  now: Date = new Date()
): WeatherDaypart[] {
  const targetDateIso = getDateIsoInTimezone(now, timezone);
  const dayHours = hourly.filter((point) => getDateIsoFromTimestamp(point.timestampIso, timezone) === targetDateIso);
  const periods: Array<{ key: WeatherDaypart['key']; label: string; hour: number }> = [
    { key: 'morning', label: 'Morning', hour: 9 },
    { key: 'day', label: 'Day', hour: 12 },
    { key: 'afternoon', label: 'Afternoon', hour: 15 },
    { key: 'night', label: 'Night', hour: 21 }
  ];

  return periods.map((period) => ({
    key: period.key,
    label: period.label,
    point: pickNearestHour(dayHours, period.hour, timezone)
  }));
}

export function summarizeDayFromHourly(
  dateIso: string,
  hourly: WeatherPoint[],
  timezone?: string | null
): {
  lowTemperature?: WeatherMetricValue;
  highTemperature?: WeatherMetricValue;
  lowWindSpeed?: WeatherMetricValue;
  highWindSpeed?: WeatherMetricValue;
  representativeWindDirectionDegrees?: number | null;
  representativeVisibility?: WeatherMetricValue;
} {
  const dayHours = hourly.filter((point) => getDateIsoFromTimestamp(point.timestampIso, timezone) === dateIso);
  const lowTemperature = summarizeMetric(dayHours, 'temperature2m', 'min');
  const highTemperature = summarizeMetric(dayHours, 'temperature2m', 'max');
  const lowWindSpeed = summarizeMetric(dayHours, 'windSpeed10m', 'min');
  const highWindSpeed = summarizeMetric(dayHours, 'windSpeed10m', 'max');
  const windPoint = pickNearestHour(dayHours, 15, timezone) ?? dayHours.find((point) => point.windDirection10mDegrees !== null && point.windDirection10mDegrees !== undefined);
  const visibilityPoint =
    pickNearestHour(
      dayHours.filter((point) => {
        const visibility = point.visibility?.corrected ?? point.visibility?.raw;
        return visibility !== null && visibility !== undefined && !Number.isNaN(visibility);
      }),
      12,
      timezone
    ) ?? dayHours.find((point) => {
      const visibility = point.visibility?.corrected ?? point.visibility?.raw;
      return visibility !== null && visibility !== undefined && !Number.isNaN(visibility);
    });

  return {
    lowTemperature,
    highTemperature,
    lowWindSpeed,
    highWindSpeed,
    representativeWindDirectionDegrees: windPoint?.windDirection10mDegrees,
    representativeVisibility: visibilityPoint?.visibility
  };
}

export function formatTemperatureRange(
  low: WeatherMetricValue | undefined,
  high: WeatherMetricValue | undefined,
  unitSystem: WeatherUnitSystem
): string {
  const lowLabel = formatWeatherValue(low, unitSystem, 'temperature2m');
  const highLabel = formatWeatherValue(high, unitSystem, 'temperature2m');
  if (lowLabel === '—' && highLabel === '—') {
    return 'Temps unavailable';
  }
  if (lowLabel === highLabel) {
    return highLabel;
  }
  return `${lowLabel} / ${highLabel}`;
}

export function formatWindRange(
  low: WeatherMetricValue | undefined,
  high: WeatherMetricValue | undefined,
  unitSystem: WeatherUnitSystem
): string {
  const lowValue = low?.corrected ?? low?.raw;
  const highValue = high?.corrected ?? high?.raw;
  if (
    (lowValue === null || lowValue === undefined || Number.isNaN(lowValue)) &&
    (highValue === null || highValue === undefined || Number.isNaN(highValue))
  ) {
    return 'Wind unavailable';
  }
  const unit = unitSystem === 'imperial' ? 'mph' : 'm/s';
  const lowDisplay =
    lowValue === null || lowValue === undefined || Number.isNaN(lowValue)
      ? null
      : convertWindSpeedMps(lowValue, unitSystem).toFixed(1);
  const highDisplay =
    highValue === null || highValue === undefined || Number.isNaN(highValue)
      ? null
      : convertWindSpeedMps(highValue, unitSystem).toFixed(1);
  const speedLabel =
    lowDisplay && highDisplay && lowDisplay !== highDisplay
      ? `${lowDisplay}-${highDisplay} ${unit}`
      : `${highDisplay ?? lowDisplay ?? '—'} ${unit}`;
  return speedLabel;
}

export function formatPowerWatts(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  if (Math.abs(value) >= 1000) {
    return `${(value / 1000).toFixed(1)} kW`;
  }
  return `${Math.round(value)} W`;
}

export function formatEnergyKwh(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  return `${value.toFixed(value >= 10 ? 0 : 1)} kWh`;
}

export function formatSolarOutlookSummary(outlook: SolarDayOutlook | undefined): string {
  if (!outlook) {
    return '';
  }
  if ((outlook.actualSoFarKwh ?? 0) > 0) {
    const soFar = formatEnergyKwh(outlook.actualSoFarKwh);
    const total = formatEnergyKwh(outlook.energyKwh);
    if (total !== '—') {
      return `Solar ${soFar} so far · ${total} est total`;
    }
    return `Solar ${soFar} so far`;
  }
  const peak = formatPowerWatts(outlook.peakWatts);
  const energy = formatEnergyKwh(outlook.energyKwh);
  if (peak === '—' && energy === '—') {
    return '';
  }
  if (peak === '—') {
    return `Solar est ${energy}`;
  }
  if (energy === '—') {
    return `Solar peak ${peak}`;
  }
  return `Solar est ${energy} · peak ${peak}`;
}

export function formatMiniSolarOutlookSummary(outlook: SolarDayOutlook | undefined): string {
  if (!outlook) {
    return '';
  }
  const soFar = formatEnergyKwh(outlook.actualSoFarKwh);
  const remaining = formatEnergyKwh(outlook.forecastRemainingKwh);
  const total = formatEnergyKwh(outlook.energyKwh);
  if (soFar !== '—' && remaining !== '—') {
    return `Solar ${soFar} + ${remaining} est`;
  }
  if (soFar !== '—' && total !== '—') {
    return `Solar ${soFar} · ${total} total`;
  }
  if (soFar !== '—') {
    return `Solar ${soFar} today`;
  }
  if (total !== '—') {
    return `Solar est ${total} today`;
  }
  return '';
}

export function formatSolarCapacitySummary(capacity: SolarCapacityEstimate | undefined): string {
	if (!capacity?.estimatedPeakWatts) {
		return 'Solar potential estimates improve once live PV data is available.';
	}
  const peak = formatPowerWatts(capacity.estimatedPeakWatts);
  switch (capacity.method) {
    case 'rolling_observed_p95':
      return `Observed site potential ${peak}, learned from recent solar production.`;
    case 'rolling_observed_p95_and_irradiance':
      return `Observed site potential ${peak}, learned from recent solar production and current irradiance.`;
    case 'rolling_observed_p95_device_share':
    case 'rolling_observed_p95_and_irradiance_device_share':
      return `Allocated device potential ${peak}, derived from site calibration and recent device share.`;
    case 'live_pv_and_irradiance':
      return `Heuristic peak potential ${peak}, calibrated from observed solar generation and forecast irradiance.`;
    case 'live_pv_and_irradiance_device_share':
      return `Allocated device potential ${peak}, derived from site irradiance and recent device share.`;
    case 'input_ceiling':
      return `Conservative solar potential ${peak}, estimated from device solar input limits.`;
    case 'input_ceiling_device_share':
      return `Allocated device potential ${peak}, constrained by device solar input limits.`;
    case 'live_pv_only':
      return `Heuristic peak potential ${peak}, inferred from observed solar generation.`;
    case 'live_pv_only_device_share':
      return `Allocated device potential ${peak}, inferred from recent device share of observed solar generation.`;
    default:
      return 'Solar potential estimates improve once live PV data is available.';
	}
}

export function formatSolarModelSummary(outlook: SolarOutlook | undefined): string {
  const provenance = outlook?.provenance;
  const deviceAllocated = outlook?.capacity?.method?.includes('device_share') ?? false;
  const scopeLabel = deviceAllocated ? 'Site-allocated device' : outlook?.scope?.mode === 'device' ? 'Device' : 'Site';
  if (!provenance) {
    return 'Baseline solar forecast.';
  }
  if (provenance.calibrationApplied) {
    const sampleLabel =
      provenance.calibrationSampleCount > 0
        ? `${provenance.calibrationSampleCount} verified ${scopeLabel.toLowerCase()}-hours`
        : `verified ${scopeLabel.toLowerCase()} history`;
    return `${scopeLabel}-calibrated solar forecast from ${sampleLabel}.`;
  }
  return `Baseline solar forecast while ${scopeLabel.toLowerCase()} calibration warms up.`;
}

export function formatSolarProvenanceSummary(outlook: SolarOutlook | undefined): string {
  if (!outlook) {
    return '';
  }
  const parts: string[] = [];
  const provenance = outlook.provenance;
  const deviceAllocated = outlook.capacity.method.includes('device_share');
  if (provenance?.calibrationApplied) {
    parts.push(deviceAllocated ? 'Site-calibrated device allocation' : outlook.scope?.mode === 'device' ? 'Device-calibrated' : 'Site-calibrated');
  } else {
    parts.push('Baseline');
  }
  switch (outlook.capacity.method) {
    case 'rolling_observed_p95':
    case 'rolling_observed_p95_and_irradiance':
    case 'rolling_observed_p95_device_share':
    case 'rolling_observed_p95_and_irradiance_device_share':
      parts.push('rolling P95 capacity');
      break;
    case 'live_pv_and_irradiance':
    case 'live_pv_only':
    case 'live_pv_and_irradiance_device_share':
    case 'live_pv_only_device_share':
      parts.push('observed PV capacity');
      break;
    case 'input_ceiling':
    case 'input_ceiling_device_share':
      parts.push('input-limit fallback');
      break;
    default:
      break;
  }
  if (provenance?.sameDayCurtailmentApplied && provenance.sameDayCurtailmentReason === 'battery_near_full') {
    parts.push('battery near full');
  }
  return parts.join(' · ');
}

export function buildSolarOutlook(
  forecast: WeatherForecast | undefined,
  devices: DeviceSummary[],
  history?: SolarHistorySnapshot,
  now: Date = new Date()
): SolarOutlook | undefined {
  if (!forecast) {
    return undefined;
  }
  const capacity = inferSolarCapacityEstimate(devices, forecast, history);
  const todayIso = getTodayIsoInTimezone(forecast.timezone, now);
  const daily = forecast.daily.map((day) =>
    summarizeSolarDay(
      day.dateIso,
      forecast.hourly,
      capacity.estimatedPeakWatts,
      forecast.timezone,
      day.dateIso === todayIso ? history : undefined,
      now
    )
  );
  const today = daily.find((day) => day.dateIso === todayIso);
  return {
    capacity,
    today,
    daily
  };
}

export function inferSolarCapacityEstimate(
  devices: DeviceSummary[],
  forecast: Pick<WeatherForecast, 'current'>,
  history?: SolarHistorySnapshot
): SolarCapacityEstimate {
  const inputCeilingWatts = devices.reduce((sum, device) => {
    const deviceCeiling =
      device.details?.solarPorts?.reduce((deviceSum, port) => deviceSum + Math.max(port.maxWatts ?? 0, 0), 0) ?? 0;
    return sum + deviceCeiling;
  }, 0);
  const observedPvWatts = devices.reduce((sum, device) => sum + Math.max(device.pvW ?? 0, 0), 0);
  const observedHistoryPeakWatts = history ? estimatePeakWattsFromHistory(history.seriesWh) : 0;
  const irradianceFactor = resolveIrradianceFactor(forecast.current);

  if ((observedPvWatts > 0 || observedHistoryPeakWatts > 0) && irradianceFactor >= 0.35) {
    const strongestObservedWatts = Math.max(observedPvWatts, observedHistoryPeakWatts);
    const inferredFromLive = strongestObservedWatts / irradianceFactor;
    const conservativeLiveEstimate = inferredFromLive * 0.6;
    const boundedEstimate =
      inputCeilingWatts > 0
        ? Math.min(Math.max(strongestObservedWatts, conservativeLiveEstimate), inputCeilingWatts * 0.65)
        : Math.max(strongestObservedWatts, conservativeLiveEstimate);
    return {
      estimatedPeakWatts: roundWatts(boundedEstimate),
      observedPvWatts: strongestObservedWatts > 0 ? roundWatts(strongestObservedWatts) : undefined,
      inputCeilingWatts: inputCeilingWatts > 0 ? roundWatts(inputCeilingWatts) : undefined,
      method: 'live_pv_and_irradiance'
    };
  }

  if (inputCeilingWatts > 0) {
    return {
      estimatedPeakWatts: roundWatts(inputCeilingWatts * 0.45),
      observedPvWatts:
        Math.max(observedPvWatts, observedHistoryPeakWatts) > 0
          ? roundWatts(Math.max(observedPvWatts, observedHistoryPeakWatts))
          : undefined,
      inputCeilingWatts: roundWatts(inputCeilingWatts),
      method: 'input_ceiling'
    };
  }

  if (observedPvWatts > 0 || observedHistoryPeakWatts > 0) {
    const strongestObservedWatts = Math.max(observedPvWatts, observedHistoryPeakWatts);
    return {
      estimatedPeakWatts: roundWatts(strongestObservedWatts * 1.1),
      observedPvWatts: roundWatts(strongestObservedWatts),
      method: 'live_pv_only'
    };
  }

  return {
    method: 'unavailable'
  };
}

export function buildWeatherLocationKey(
  latitude: number,
  longitude: number,
  timezone: string | undefined | null
): string {
  return `${latitude.toFixed(3)}:${longitude.toFixed(3)}:${(timezone || 'auto').trim() || 'auto'}`;
}

export function resolveProfileWeatherState(preference?: WeatherLocationPreference): {
  enabled: boolean;
  timezone: string;
  location?: WeatherLocationPreference['weatherLocation'];
  locationKey: string;
} {
  const timezone = (preference?.timezone || 'auto').trim() || 'auto';
  const location = preference?.weatherLocationEnabled ? preference.weatherLocation ?? undefined : undefined;
  return {
    enabled: Boolean(location),
    timezone,
    location,
    locationKey: location ? buildWeatherLocationKey(location.latitude, location.longitude, timezone) : 'none'
  };
}

export function getTodayIsoInTimezone(timezone?: string | null, now: Date = new Date()): string {
  return getDateIsoInTimezone(now, timezone);
}

export function formatLocalTimeLabel(
  timestampIso: string,
  timezone?: string | null,
  locale = 'en-US'
): string {
  try {
    return new Intl.DateTimeFormat(locale, {
      hour: 'numeric',
      minute: '2-digit',
      timeZone: timezone?.trim() || 'UTC'
    }).format(new Date(timestampIso));
  } catch {
    return timestampIso;
  }
}

function getDateIsoInTimezone(date: Date, timezone?: string | null): string {
  try {
    const formatter = new Intl.DateTimeFormat('en-CA', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      timeZone: timezone?.trim() || 'UTC'
    });
    const parts = formatter.formatToParts(date);
    const year = parts.find((part) => part.type === 'year')?.value;
    const month = parts.find((part) => part.type === 'month')?.value;
    const day = parts.find((part) => part.type === 'day')?.value;
    return `${year}-${month}-${day}`;
  } catch {
    return date.toISOString().slice(0, 10);
  }
}

function getDateIsoFromTimestamp(timestampIso: string, timezone?: string | null): string {
  return getDateIsoInTimezone(new Date(timestampIso), timezone);
}

function diffIsoDays(leftIso: string, rightIso: string): number {
  const left = Date.parse(`${leftIso}T00:00:00Z`);
  const right = Date.parse(`${rightIso}T00:00:00Z`);
  return Math.round((left - right) / 86_400_000);
}

function pickNearestHour(
  points: WeatherPoint[],
  targetHour: number,
  timezone?: string | null
): WeatherPoint | undefined {
  let best: WeatherPoint | undefined;
  let bestDistance = Number.POSITIVE_INFINITY;
  for (const point of points) {
    const hour = getHourInTimezone(point.timestampIso, timezone);
    const distance = Math.abs(hour - targetHour);
    if (distance < bestDistance) {
      best = point;
      bestDistance = distance;
    }
  }
  return best;
}

function getHourInTimezone(timestampIso: string, timezone?: string | null): number {
  try {
    const formatter = new Intl.DateTimeFormat('en-US', {
      hour: 'numeric',
      hour12: false,
      timeZone: timezone?.trim() || 'UTC'
    });
    const parts = formatter.formatToParts(new Date(timestampIso));
    const hour = parts.find((part) => part.type === 'hour')?.value;
    return Number(hour ?? 0);
  } catch {
    return new Date(timestampIso).getUTCHours();
  }
}

function summarizeMetric(
  points: WeatherPoint[],
  metric: 'temperature2m' | 'windSpeed10m',
  mode: 'min' | 'max'
): WeatherMetricValue | undefined {
  let selectedValue: number | undefined;
  let selectedCorrected: number | undefined;

  for (const point of points) {
    const candidate = point[metric];
    const normalized = candidate?.corrected ?? candidate?.raw;
    if (normalized === null || normalized === undefined || Number.isNaN(normalized)) {
      continue;
    }
    if (
      selectedValue === undefined ||
      (mode === 'min' ? normalized < selectedValue : normalized > selectedValue)
    ) {
      selectedValue = normalized;
      selectedCorrected = candidate?.corrected ?? undefined;
    }
  }

  if (selectedValue === undefined) {
    return undefined;
  }
  return {
    raw: selectedCorrected === undefined ? selectedValue : undefined,
    corrected: selectedCorrected !== undefined ? selectedValue : undefined
  };
}

function summarizeSolarDay(
  dateIso: string,
  hourly: WeatherPoint[],
  estimatedPeakWatts: number | undefined,
  timezone?: string | null,
  history?: SolarHistorySnapshot,
  now: Date = new Date()
): SolarDayOutlook {
  const dayHours = hourly.filter((point) => getDateIsoFromTimestamp(point.timestampIso, timezone) === dateIso);
  let peakWatts = 0;
  let peakHourIso: string | undefined;
  let forecastRemainingKwh = 0;
  let irradianceSource: SolarDayOutlook['irradianceSource'] = 'unavailable';
  const todayIso = getTodayIsoInTimezone(timezone, now);
  const nowMs = now.getTime();

  for (const point of dayHours) {
    const estimatedWatts = estimateSolarWattsForPoint(point, estimatedPeakWatts);
    const isFuturePoint = dateIso !== todayIso || Date.parse(point.timestampIso) > nowMs;
    if (estimatedWatts !== undefined && isFuturePoint) {
      forecastRemainingKwh += estimatedWatts / 1000;
      if (estimatedWatts > peakWatts) {
        peakWatts = estimatedWatts;
        peakHourIso = point.timestampIso;
      }
    }
    const source = resolveIrradianceSource(point);
    if (irradianceSource === 'unavailable' && source !== 'unavailable') {
      irradianceSource = source;
    }
  }

  const actualSoFarKwh =
    history && dateIso === todayIso && history.todayWh > 0 ? Number((history.todayWh / 1000).toFixed(1)) : undefined;
  const totalEnergyKwh = (actualSoFarKwh ?? 0) + forecastRemainingKwh;

  return {
    dateIso,
    peakWatts: peakWatts > 0 ? roundWatts(peakWatts) : undefined,
    energyKwh: totalEnergyKwh > 0 ? Number(totalEnergyKwh.toFixed(1)) : undefined,
    actualSoFarKwh,
    forecastRemainingKwh: forecastRemainingKwh > 0 ? Number(forecastRemainingKwh.toFixed(1)) : undefined,
    peakHourIso,
    irradianceSource
  };
}

function estimateSolarWattsForPoint(
  point: WeatherPoint,
  estimatedPeakWatts: number | undefined
): number | undefined {
  if (!estimatedPeakWatts || estimatedPeakWatts <= 0) {
    return undefined;
  }
  const irradianceFactor = resolveIrradianceFactor(point);
  if (irradianceFactor <= 0) {
    return undefined;
  }
  const temperatureRaw = point.temperature2m?.corrected ?? point.temperature2m?.raw;
  const temperatureFactor =
    temperatureRaw === null || temperatureRaw === undefined || Number.isNaN(temperatureRaw)
      ? 1
      : Math.max(0.82, 1 - Math.max(temperatureRaw - 25, 0) * 0.004);
  return Math.min(estimatedPeakWatts * 1.05, estimatedPeakWatts * irradianceFactor * temperatureFactor);
}

function resolveIrradianceFactor(point: WeatherPoint): number {
  const irradiance = resolveIrradianceValue(point);
  if (irradiance === undefined) {
    return 0;
  }
  return clamp(irradiance / 1000, 0, 1.1);
}

function resolveIrradianceValue(point: WeatherPoint): number | undefined {
  const gti = point.globalTiltedIrradiance?.corrected ?? point.globalTiltedIrradiance?.raw;
  if (gti !== null && gti !== undefined && !Number.isNaN(gti)) {
    return gti;
  }
  const shortwave = point.shortwaveRadiation?.corrected ?? point.shortwaveRadiation?.raw;
  if (shortwave !== null && shortwave !== undefined && !Number.isNaN(shortwave)) {
    return shortwave;
  }
  return undefined;
}

function resolveIrradianceSource(point: WeatherPoint): SolarDayOutlook['irradianceSource'] {
  const gti = point.globalTiltedIrradiance?.corrected ?? point.globalTiltedIrradiance?.raw;
  if (gti !== null && gti !== undefined && !Number.isNaN(gti)) {
    return 'gti';
  }
  const shortwave = point.shortwaveRadiation?.corrected ?? point.shortwaveRadiation?.raw;
  if (shortwave !== null && shortwave !== undefined && !Number.isNaN(shortwave)) {
    return 'shortwave_radiation';
  }
  return 'unavailable';
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function roundWatts(value: number): number {
  return Math.round(value / 10) * 10;
}

function estimatePeakWattsFromHistory(seriesWh: number[]): number {
  if (!seriesWh.length) {
    return 0;
  }
  const maxBucketWh = Math.max(...seriesWh.map((value) => Math.max(value, 0)));
  return maxBucketWh * 6;
}
