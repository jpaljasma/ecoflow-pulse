import { describe, expect, it } from 'vitest';
import type { RollupSeries } from '@/features/history/api';
import {
  buildPowerTrendBounds,
  buildPowerTrendView,
  mergeTrendPrefill,
  POWER_TREND_POINTS,
  sparklineCoveragePoints
} from '@/features/history/powerTrend';

function buildSeries(points: RollupSeries['points']): RollupSeries {
  return {
    deviceId: 'device-1',
    resolution: 'minute',
    fromUnixMs: '0',
    toUnixMs: '0',
    points
  };
}

describe('power trend helpers', () => {
  it('builds a 5 minute prefill window aligned to minute history buckets', () => {
    const now = new Date('2026-03-07T13:04:55.000Z');
    const bounds = buildPowerTrendBounds(now);

    expect(bounds.queryFrom.toISOString()).toBe('2026-03-07T13:00:00.000Z');
    expect(bounds.queryTo.toISOString()).toBe('2026-03-07T13:05:00.000Z');
  });

  it('expands minute rollups into 5 second trend points', () => {
    const now = new Date('2026-03-07T13:04:55.000Z');
    const series = buildSeries([
      {
        bucketStartUnixMs: String(Date.parse('2026-03-07T13:02:00.000Z')),
        bucketEndUnixMs: String(Date.parse('2026-03-07T13:03:00.000Z')),
        sampleCount: 12,
        firstTsUnixMs: '0',
        lastTsUnixMs: '0',
        metrics: {
          socAvgPct: 0,
          socMinPct: 0,
          socMaxPct: 0,
          acInAvgW: 40,
          acInMaxW: 0,
          pvAvgW: 120,
          pvMaxW: 0,
          dcAvgW: 10,
          dcMaxW: 0,
          loadAvgW: 80,
          loadMaxW: 0,
          netAvgW: 0,
          netMinW: 0,
          netMaxW: 0,
          batteryAvgW: 0,
          batteryMinW: 0,
          batteryMaxW: 0,
          tempAvgC: 0,
          tempMinC: 0,
          tempMaxC: 0,
          solarGeneratedWh: 0,
          acInputEnergyWh: 0,
          dcOutputEnergyWh: 0,
          loadEnergyWh: 0,
          batteryChargeEnergyWh: 0,
          batteryDischargeEnergyWh: 0
        }
      }
    ]);

    const trend = buildPowerTrendView(series, now);

    expect(trend.solar).toHaveLength(POWER_TREND_POINTS);
    expect(trend.solar.slice(24, 36)).toEqual(Array.from({ length: 12 }, () => 120));
    expect(trend.ac.slice(24, 36)).toEqual(Array.from({ length: 12 }, () => 40));
    expect(trend.dc.slice(24, 36)).toEqual(Array.from({ length: 12 }, () => 10));
    expect(trend.load.slice(24, 36)).toEqual(Array.from({ length: 12 }, () => 80));
    expect(trend.solar.slice(0, 24).every((value) => value === 0)).toBe(true);
  });

  it('keeps history on the left and overlays live coverage on the right', () => {
    const prefill = Array.from({ length: POWER_TREND_POINTS }, (_, idx) => idx + 1);
    const live = Array.from({ length: 20 }, () => 999);

    const merged = mergeTrendPrefill(prefill, live, 20);

    expect(merged).toHaveLength(POWER_TREND_POINTS);
    expect(merged.slice(0, 40)).toEqual(prefill.slice(0, 40));
    expect(merged.slice(40)).toEqual(Array.from({ length: 20 }, () => 999));
  });

  it('derives coverage buckets from sparkline timestamps instead of raw point count', () => {
    const coverage = sparklineCoveragePoints([
      { ts: Date.parse('2026-03-07T13:00:00.000Z'), value: 10 },
      { ts: Date.parse('2026-03-07T13:00:04.000Z'), value: 15 },
      { ts: Date.parse('2026-03-07T13:00:08.000Z'), value: 20 },
      { ts: Date.parse('2026-03-07T13:00:12.000Z'), value: 25 }
    ]);

    expect(coverage).toBe(4);
  });
});
