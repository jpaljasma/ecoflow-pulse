import { describe, expect, it } from 'vitest';

import { normalizeSolarOutlookResponse } from '../src/grpc/solarForecastClient.js';

describe('solarForecastClient', () => {
  it('normalizes protobufjs camel-cased next_7_days fields', () => {
    const outlook = normalizeSolarOutlookResponse({
      scope: {
        mode: 'all',
        resolvedDeviceIds: ['device-1']
      },
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
        timezone: 'America/New_York',
        canonicalLocationKey: 'grid-key',
        issuedAtUnixMs: '1773430800000',
        refreshedAtUnixMs: '1773430860000'
      },
      capacity: {
        estimatedPeakWatts: { value: 1680 },
        observedPvWatts: { value: 1230 },
        method: 'live_pv_and_irradiance'
      },
      today: {
        dateUnixMs: String(Date.UTC(2026, 2, 21)),
        actualGeneratedKwh: { value: 2.5 },
        forecastTotalKwh: { value: 2.5 },
        estimatedPeakWatts: { value: 1680 },
        confidence: 'SOLAR_FORECAST_CONFIDENCE_HIGH'
      },
      next_7Days: [
        {
          dateUnixMs: String(Date.UTC(2026, 2, 22)),
          forecastTotalKwh: { value: 1.8 },
          estimatedPeakWatts: { value: 1490 },
          confidence: 'SOLAR_FORECAST_CONFIDENCE_MEDIUM'
        },
        {
          dateUnixMs: String(Date.UTC(2026, 2, 23)),
          forecastTotalKwh: { value: 3.1 },
          estimatedPeakWatts: { value: 2010 },
          confidence: 'SOLAR_FORECAST_CONFIDENCE_MEDIUM'
        }
      ],
      next_24Hours: [
        {
          timeUnixMs: String(Date.UTC(2026, 2, 22, 12)),
          forecastGeneratedWh: { value: 240 },
          confidence: 'SOLAR_FORECAST_CONFIDENCE_MEDIUM'
        }
      ]
    });

    expect(outlook.daily).toEqual([
      expect.objectContaining({
        dateIso: '2026-03-22',
        forecastTotalKwh: 1.8,
        estimatedPeakWatts: 1490
      }),
      expect.objectContaining({
        dateIso: '2026-03-23',
        forecastTotalKwh: 3.1,
        estimatedPeakWatts: 2010
      })
    ]);
    expect(outlook.next24Hours).toEqual([
      expect.objectContaining({
        timestampIso: '2026-03-22T12:00:00.000Z',
        forecastGeneratedWh: 240
      })
    ]);
  });
});
