import { describe, expect, it } from 'vitest';
import {
  buildSolarOutlook,
  buildWeatherLocationKey,
  circularWindDirectionError,
  formatMiniSolarOutlookSummary,
  formatSolarModelSummary,
  formatSolarOutlookSummary,
  formatRelativeWeatherDayLabel,
  formatWeatherDayLabel,
  formatWeatherValue,
  formatWindRange,
  formatWindSummary,
  getForecastDayparts,
  inferSolarCapacityEstimate,
  resolveProfileWeatherState,
  summarizeDayFromHourly
} from '@/features/weather/model';

describe('weather model helpers', () => {
  it('builds stable query keys from weather location inputs', () => {
    expect(buildWeatherLocationKey(42.6159, -77.4014, 'America/New_York')).toBe(
      '42.616:-77.401:America/New_York'
    );
    expect(buildWeatherLocationKey(42.6159, -77.4014, ' ')).toBe('42.616:-77.401:auto');
  });

  it('resolves enabled weather state from profile preferences', () => {
    expect(
      resolveProfileWeatherState({
        timezone: 'America/New_York',
        weatherLocationEnabled: true,
        weatherLocation: {
          latitude: 42.6159,
          longitude: -77.4014
        }
      })
    ).toEqual({
      enabled: true,
      timezone: 'America/New_York',
      location: {
        latitude: 42.6159,
        longitude: -77.4014
      },
      locationKey: '42.616:-77.401:America/New_York'
    });
  });

  it('formats weather days using the forecast timezone', () => {
    expect(formatWeatherDayLabel('2026-03-04', 'America/New_York', 'en-US')).toBe('Wed, Mar 4');
  });

  it('formats relative forecast day labels for near-term dates', () => {
    const now = new Date('2026-03-18T12:00:00Z');
    expect(formatRelativeWeatherDayLabel('2026-03-18', 'UTC', now)).toBe('Today');
    expect(formatRelativeWeatherDayLabel('2026-03-19', 'UTC', now)).toBe('Tomorrow');
    expect(formatRelativeWeatherDayLabel('2026-03-20', 'UTC', now)).toBe('Friday');
    expect(formatRelativeWeatherDayLabel('2026-03-28', 'UTC', now)).toBe('3/28');
  });

  it('keeps circular wind direction errors in the shortest signed range', () => {
    expect(circularWindDirectionError(5, 355)).toBe(10);
    expect(circularWindDirectionError(355, 5)).toBe(-10);
  });

  it('prefers corrected values when formatting weather metrics', () => {
    expect(formatWeatherValue({ raw: 4, corrected: 5 }, 'metric', 'windSpeed10m')).toBe('5.0 m/s');
  });

  it('summarizes low/high weather values from hourly points', () => {
    const summary = summarizeDayFromHourly(
      '2026-03-18',
      [
        { timestampIso: '2026-03-18T09:00:00Z', temperature2m: { corrected: 3 }, windSpeed10m: { corrected: 4 }, windDirection10mDegrees: 45 },
        { timestampIso: '2026-03-18T15:00:00Z', temperature2m: { corrected: 12 }, windSpeed10m: { corrected: 8 }, windDirection10mDegrees: 90 },
        { timestampIso: '2026-03-18T21:00:00Z', temperature2m: { corrected: 7 }, windSpeed10m: { corrected: 6 }, windDirection10mDegrees: 180 }
      ],
      'UTC'
    );

    expect(formatWeatherValue(summary.lowTemperature, 'metric', 'temperature2m')).toBe('3.0°C');
    expect(formatWeatherValue(summary.highTemperature, 'metric', 'temperature2m')).toBe('12.0°C');
    expect(formatWindRange(summary.lowWindSpeed, summary.highWindSpeed, 'metric')).toBe('4.0-8.0 m/s');
  });

  it('builds dayparts from the forecast hourly series', () => {
    const dayparts = getForecastDayparts(
      [
        { timestampIso: '2026-03-18T09:00:00Z', weatherCode: 1, temperature2m: { corrected: 5 } },
        { timestampIso: '2026-03-18T12:00:00Z', weatherCode: 2, temperature2m: { corrected: 9 } },
        { timestampIso: '2026-03-18T15:00:00Z', weatherCode: 61, temperature2m: { corrected: 11 } },
        { timestampIso: '2026-03-18T21:00:00Z', weatherCode: 3, temperature2m: { corrected: 6 } }
      ],
      'UTC',
      new Date('2026-03-18T12:00:00Z')
    );

    expect(dayparts.map((item) => item.label)).toEqual(['Morning', 'Day', 'Afternoon', 'Night']);
    expect(formatWeatherValue(dayparts[2]?.point?.temperature2m, 'metric', 'temperature2m')).toBe('11.0°C');
  });

  it('formats wind summaries with direction and speed', () => {
    expect(formatWindSummary({ corrected: 5 }, 'metric')).toBe('5.0 m/s');
  });

  it('infers solar capacity from live PV and current irradiance', () => {
    const estimate = inferSolarCapacityEstimate(
      [
        {
          id: 'device-1',
          serialNumber: 'sn-1',
          name: 'Delta',
          model: 'delta-pro',
          online: true,
          batteryPct: 84,
          state: 'charging',
          etaMinutes: 0,
          pvW: 640,
          details: {
            solarPorts: [
              { id: 'pv-1', name: 'PV 1', maxWatts: 800 },
              { id: 'pv-2', name: 'PV 2', maxWatts: 800 }
            ]
          }
        }
      ],
      {
        current: {
          timestampIso: '2026-03-18T16:00:00Z',
          globalTiltedIrradiance: { corrected: 400 }
        }
      }
    );

    expect(estimate.method).toBe('live_pv_and_irradiance');
    expect(estimate.estimatedPeakWatts).toBe(960);
  });

  it('falls back to device solar input ceilings when live PV cannot infer peak potential', () => {
    const estimate = inferSolarCapacityEstimate(
      [
        {
          id: 'device-1',
          serialNumber: 'sn-1',
          name: 'Delta',
          model: 'delta-pro',
          online: true,
          batteryPct: 84,
          state: 'idle',
          etaMinutes: 0,
          details: {
            solarPorts: [{ id: 'pv-1', name: 'PV 1', maxWatts: 400 }]
          }
        }
      ],
      {
        current: {
          timestampIso: '2026-03-18T01:00:00Z'
        }
      }
    );

    expect(estimate.method).toBe('input_ceiling');
    expect(estimate.estimatedPeakWatts).toBe(180);
  });

  it('builds a today and 7-day solar outlook from actual-so-far solar plus remaining irradiance', () => {
    const outlook = buildSolarOutlook(
      {
        issuedAtUnixMs: '1760000000000',
        timezone: 'UTC',
        unitSystem: 'metric',
        provenance: {
          source: 'open_meteo',
          modelSelection: 'best_match'
        },
        current: {
          timestampIso: '2026-03-18T12:00:00Z',
          globalTiltedIrradiance: { corrected: 500 }
        },
        hourly: [
          {
            timestampIso: '2026-03-18T10:00:00Z',
            temperature2m: { corrected: 8 },
            globalTiltedIrradiance: { corrected: 200 }
          },
          {
            timestampIso: '2026-03-18T12:00:00Z',
            temperature2m: { corrected: 10 },
            globalTiltedIrradiance: { corrected: 500 }
          },
          {
            timestampIso: '2026-03-18T14:00:00Z',
            temperature2m: { corrected: 11 },
            globalTiltedIrradiance: { corrected: 800 }
          },
          {
            timestampIso: '2026-03-19T12:00:00Z',
            temperature2m: { corrected: 9 },
            globalTiltedIrradiance: { corrected: 700 }
          }
        ],
        daily: [
          { dateIso: '2026-03-18' },
          { dateIso: '2026-03-19' }
        ]
      },
      [
        {
          id: 'device-1',
          serialNumber: 'sn-1',
          name: 'Delta',
          model: 'delta-pro',
          online: true,
          batteryPct: 84,
          state: 'charging',
          etaMinutes: 0,
          pvW: 700
        }
      ],
      {
        todayWh: 8660,
        seriesWh: [20, 80, 140, 190, 260, 300, 280, 220]
      },
      new Date('2026-03-18T12:00:00Z')
    );

    expect(outlook?.today?.dateIso).toBe('2026-03-18');
    expect(outlook?.today?.peakWatts).toBe(1730);
    expect(outlook?.today?.actualSoFarKwh).toBe(8.7);
    expect(outlook?.today?.forecastRemainingKwh).toBe(1.7);
    expect(outlook?.today?.energyKwh).toBe(10.4);
    expect(outlook?.daily).toHaveLength(2);
    expect(formatSolarOutlookSummary(outlook?.daily[0])).toBe('Solar 8.7 kWh so far · 10 kWh est total');
    expect(formatMiniSolarOutlookSummary(outlook?.daily[0])).toBe('Solar 8.7 kWh + 1.7 kWh est');
    expect(formatSolarOutlookSummary(outlook?.daily[1])).toBe('Solar est 1.5 kWh · peak 1.5 kW');
  });

  it('omits solar summary copy when no outlook values are available', () => {
    expect(
      formatSolarOutlookSummary({
        dateIso: '2026-03-18',
        irradianceSource: 'unavailable'
      })
    ).toBe('');
  });

  it('describes whether the served solar forecast is site calibrated', () => {
    expect(
      formatSolarModelSummary({
        capacity: { method: 'unavailable' },
        daily: [],
        provenance: {
          forecastSource: 'solarforecastd',
          forecastModel: 'deterministic_baseline_v1',
          servedVariant: 'site_calibrated',
          baselineModel: 'deterministic_baseline_v1',
          calibrationApplied: true,
          calibrationSampleCount: 24,
          actualsSource: 'telemetry_rollups',
          weatherSource: 'open_meteo',
          weatherModelSelection: 'best_match',
          timezone: 'UTC',
          canonicalLocationKey: 'grid-key',
          issuedAtUnixMs: '1',
          refreshedAtUnixMs: '2'
        }
      })
    ).toBe('Site-calibrated solar forecast from 24 verified site-hours.');
    expect(formatSolarModelSummary(undefined)).toBe('Baseline solar forecast.');
  });
});
