import { describe, expect, it, vi } from 'vitest';
import {
  convertDistanceM,
  convertPrecipitationMm,
  convertTemperatureC,
  convertWindSpeedMps,
  circularWindDirectionError,
  formatWeatherValue,
  getWeatherCodeIcon,
  getWeatherCodeLabel
} from '@/features/weather/model';
import { fetchSolarOutlook, fetchWeatherForecast } from '@/features/weather/api';

const restClientMock = vi.hoisted(() => ({
  requestJson: vi.fn()
}));

vi.mock('@/shared/api/restClient', () => restClientMock);

describe('weather model helpers', () => {
  it('converts temperature, wind speed, distance, and precipitation locally', () => {
    expect(convertTemperatureC(0, 'imperial')).toBe(32);
    expect(convertWindSpeedMps(10, 'imperial')).toBeCloseTo(22.36936, 5);
    expect(convertDistanceM(1000, 'metric')).toBe(1);
    expect(convertDistanceM(1609.344, 'imperial')).toBeCloseTo(1, 6);
    expect(convertPrecipitationMm(25.4, 'imperial')).toBeCloseTo(1, 6);
  });

  it('maps weather codes to readable labels and icons', () => {
    expect(getWeatherCodeLabel(0)).toBe('Clear sky');
    expect(getWeatherCodeLabel(95)).toBe('Thunderstorm');
    expect(getWeatherCodeIcon(2)).toBe('weather-partly-cloudy');
    expect(getWeatherCodeIcon(99)).toBe('weather-lightning-rainy');
  });

  it('computes circular wind-direction error without wraparound spikes', () => {
    expect(circularWindDirectionError(10, 350)).toBe(20);
    expect(circularWindDirectionError(350, 10)).toBe(-20);
    expect(circularWindDirectionError(180, 0)).toBe(180);
  });

  it('formats weather values with the active unit system', () => {
    expect(formatWeatherValue({ raw: 20, corrected: 21 }, 'imperial', 'temperature2m')).toBe('69.8°F');
    expect(formatWeatherValue({ raw: 7.5 }, 'metric', 'windSpeed10m')).toBe('7.5 m/s');
  });
});

describe('weather api parsing', () => {
  it('preserves raw and corrected forecast values', async () => {
    restClientMock.requestJson.mockResolvedValueOnce({
      forecast: {
        issuedAtUnixMs: '1770000000000',
        timezone: 'America/New_York',
        unitSystem: 'metric',
        provenance: {
          source: 'open_meteo',
          modelSelection: 'best_match',
          actualSource: 'past_days'
        },
        current: {
          timestampIso: '2026-03-04T15:00:00Z',
          weatherCode: 63,
          temperature2m: { raw: 12.3, corrected: 12.8, unit: 'celsius' },
          windSpeed10m: { raw: 4.1, corrected: 3.9, unit: 'm/s' }
        },
        hourly: [],
        daily: [
          {
            dateIso: '2026-03-04',
            weatherCode: 63,
            sunriseIso: '2026-03-04T11:00:00Z',
            sunsetIso: '2026-03-04T22:00:00Z'
          }
        ]
      }
    });

    const result = await fetchWeatherForecast();
    expect(result.forecast.current.temperature2m?.corrected).toBe(12.8);
    expect(result.forecast.current.weatherLabel).toBe('Rain');
    expect(result.forecast.daily[0]?.weatherIcon).toBeDefined();
  });

  it('normalizes timestamp-shaped daily dates into calendar dates', async () => {
    restClientMock.requestJson.mockResolvedValueOnce({
      forecast: {
        issuedAtUnixMs: '1770000000000',
        timezone: 'America/New_York',
        unitSystem: 'metric',
        provenance: {
          source: 'open_meteo',
          modelSelection: 'best_match',
          actualSource: 'past_days'
        },
        current: {
          timestampIso: '2026-03-04T15:00:00Z',
          weatherCode: 63
        },
        hourly: [],
        daily: [
          {
            dateIso: '2026-03-17T04:00:00.000Z',
            weatherCode: 63
          }
        ]
      }
    });

    const result = await fetchWeatherForecast();
    expect(result.forecast.daily[0]?.dateIso).toBe('2026-03-17');
  });

  it('normalizes solar outlook payloads into the widget model shape', async () => {
    restClientMock.requestJson.mockResolvedValueOnce({
      outlook: {
        provenance: {
          forecastSource: 'solarforecastd',
          forecastModel: 'deterministic_baseline_v1',
          servedVariant: 'site_calibrated',
          baselineModel: 'deterministic_baseline_v1',
          calibrationApplied: true,
          calibrationSampleCount: 24,
          calibrationUpdatedAtUnixMs: '1770003600000',
          sameDayCurtailmentApplied: true,
          sameDayCurtailmentReason: 'battery_near_full',
          actualsSource: 'telemetry_rollups',
          weatherSource: 'open_meteo',
          weatherModelSelection: 'best_match',
          timezone: 'America/New_York',
          canonicalLocationKey: 'grid-key',
          issuedAtUnixMs: '1770000000000',
          refreshedAtUnixMs: '1770000300000'
        },
        capacity: {
          estimatedPeakWatts: 1680,
          observedPvWatts: 1230,
          method: 'rolling_observed_p95'
        },
        today: {
          dateIso: '2026-03-18T04:00:00.000Z',
          actualGeneratedKwh: 5.2,
          forecastRemainingKwh: 1.8,
          forecastTotalKwh: 7,
          estimatedPeakWatts: 1680,
          peakTimeIso: '2026-03-18T18:00:00.000Z',
          confidence: 'high'
        },
        daily: [
          {
            dateIso: '2026-03-19T04:00:00.000Z',
            forecastTotalKwh: 4.3,
            estimatedPeakWatts: 1250,
            confidence: 'medium'
          }
        ],
        next24Hours: []
      }
    });

    const result = await fetchSolarOutlook();
    expect(result.outlook.capacity.estimatedPeakWatts).toBe(1680);
    expect(result.outlook.today?.energyKwh).toBe(7);
    expect(result.outlook.today?.dateIso).toBe('2026-03-18');
    expect(result.outlook.daily[0]?.dateIso).toBe('2026-03-19');
    expect(result.outlook.today?.irradianceSource).toBe('unavailable');
    expect(result.outlook.provenance?.servedVariant).toBe('site_calibrated');
    expect(result.outlook.provenance?.calibrationSampleCount).toBe(24);
    expect(result.outlook.provenance?.sameDayCurtailmentApplied).toBe(true);
    expect(result.outlook.provenance?.sameDayCurtailmentReason).toBe('battery_near_full');
    expect(result.outlook.capacity.method).toBe('rolling_observed_p95');
  });

  it('requests device-scoped solar outlooks with query params', async () => {
    restClientMock.requestJson.mockResolvedValueOnce({
      outlook: {
        capacity: { method: 'unavailable' },
        daily: [],
        next24Hours: []
      }
    });

    await fetchSolarOutlook('token-123', {
      scope: 'device',
      deviceId: '019c9f0e-4521-775d-873e-e80039f16d75'
    });

    expect(restClientMock.requestJson).toHaveBeenCalledWith(
      '/api/v1/solar/outlook?scope=device&deviceId=019c9f0e-4521-775d-873e-e80039f16d75',
      { token: 'token-123' }
    );
  });

  it('parses site-allocated device solar capacity methods', async () => {
    restClientMock.requestJson.mockResolvedValueOnce({
      outlook: {
        capacity: {
          estimatedPeakWatts: 1480,
          observedPvWatts: 980,
          method: 'rolling_observed_p95_and_irradiance_device_share'
        },
        scope: {
          mode: 'device',
          deviceId: '019c9f0e-4521-775d-873e-e80039f16d75',
          resolvedDeviceIds: [
            '019c9f0e-4521-775d-873e-e80039f16d75'
          ]
        },
        daily: [],
        next24Hours: []
      }
    });

    const result = await fetchSolarOutlook('token-123', {
      scope: 'device',
      deviceId: '019c9f0e-4521-775d-873e-e80039f16d75'
    });

    expect(result.outlook.capacity.method).toBe('rolling_observed_p95_and_irradiance_device_share');
    expect(result.outlook.scope?.mode).toBe('device');
  });
});
