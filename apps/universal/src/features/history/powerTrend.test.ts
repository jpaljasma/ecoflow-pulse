import { describe, expect, it } from 'vitest';
import type { RollupSeries } from '@/features/history/api';
import {
  buildPowerTrendBounds,
  buildPowerTrendView,
  mergePowerTrendPrefill,
  mergePowerTrendPrefillWithLivePoints,
  mergeTrendPrefill,
  mergeTrendPrefillWithLivePoints,
  POWER_TREND_POINTS,
  repairPowerTrendDropouts,
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

  it('repairs final fleet trend payloads after live overlays introduce a short zero gap', () => {
    const prefill = {
      solar: Array.from({ length: POWER_TREND_POINTS }, () => 200),
      ac: Array.from({ length: POWER_TREND_POINTS }, () => 80),
      dc: Array.from({ length: POWER_TREND_POINTS }, () => 20),
      load: Array.from({ length: POWER_TREND_POINTS }, () => 120)
    };
    const live = {
      solar: [200, 0, 220],
      ac: [80, 0, 100],
      dc: [20, 0, 40],
      load: [120, 0, 140]
    };

    const merged = mergePowerTrendPrefill(prefill, live, 3);

    expect(merged.solar.slice(-3)).toEqual([200, 210, 220]);
    expect(merged.ac.slice(-3)).toEqual([80, 90, 100]);
    expect(merged.dc.slice(-3)).toEqual([20, 30, 40]);
    expect(merged.load.slice(-3)).toEqual([120, 130, 140]);
  });

  it('repairs short all-zero gaps between non-zero power samples', () => {
    const repaired = repairPowerTrendDropouts({
      solar: [100, 120, 0, 160, 180],
      ac: [40, 60, 0, 100, 120],
      dc: [10, 20, 0, 40, 50],
      load: [80, 100, 0, 140, 160]
    });

    expect(repaired.solar).toEqual([100, 120, 140, 160, 180]);
    expect(repaired.ac).toEqual([40, 60, 80, 100, 120]);
    expect(repaired.dc).toEqual([10, 20, 30, 40, 50]);
    expect(repaired.load).toEqual([80, 100, 120, 140, 160]);
  });

  it('preserves real idle windows and edge zeros in power trends', () => {
    const repaired = repairPowerTrendDropouts({
      solar: [0, 120, 0, 0, 0, 160, 0],
      ac: [0, 60, 0, 0, 0, 100, 0],
      dc: [0, 20, 0, 0, 0, 40, 0],
      load: [0, 100, 0, 0, 0, 140, 0]
    });

    expect(repaired.solar).toEqual([0, 120, 0, 0, 0, 160, 0]);
    expect(repaired.ac).toEqual([0, 60, 0, 0, 0, 100, 0]);
    expect(repaired.dc).toEqual([0, 20, 0, 0, 0, 40, 0]);
    expect(repaired.load).toEqual([0, 100, 0, 0, 0, 140, 0]);
  });

  it('overlays realtime points by timestamp without creating a gap before live data starts', () => {
    const now = new Date('2026-03-07T13:04:55.000Z');
    const prefill = Array.from({ length: POWER_TREND_POINTS }, () => 262);

    const merged = mergeTrendPrefillWithLivePoints(
      prefill,
      [
        { ts: Date.parse('2026-03-07T13:04:45.000Z'), value: 262 },
        { ts: Date.parse('2026-03-07T13:04:50.000Z'), value: 265 },
        { ts: Date.parse('2026-03-07T13:04:55.000Z'), value: 262 }
      ],
      now
    );

    expect(merged.slice(0, 57).every((value) => value === 262)).toBe(true);
    expect(merged.slice(57)).toEqual([262, 265, 262]);
  });

  it('repairs final detail trend payloads after timestamped live overlays introduce a short zero gap', () => {
    const now = new Date('2026-03-07T13:04:55.000Z');
    const prefill = {
      solar: Array.from({ length: POWER_TREND_POINTS }, () => 220),
      ac: Array.from({ length: POWER_TREND_POINTS }, () => 90),
      dc: Array.from({ length: POWER_TREND_POINTS }, () => 30),
      load: Array.from({ length: POWER_TREND_POINTS }, () => 130)
    };

    const merged = mergePowerTrendPrefillWithLivePoints(
      prefill,
      {
        solar: [
          { ts: Date.parse('2026-03-07T13:04:45.000Z'), value: 220 },
          { ts: Date.parse('2026-03-07T13:04:50.000Z'), value: 0 },
          { ts: Date.parse('2026-03-07T13:04:55.000Z'), value: 240 }
        ],
        ac: [
          { ts: Date.parse('2026-03-07T13:04:45.000Z'), value: 90 },
          { ts: Date.parse('2026-03-07T13:04:50.000Z'), value: 0 },
          { ts: Date.parse('2026-03-07T13:04:55.000Z'), value: 110 }
        ],
        dc: [
          { ts: Date.parse('2026-03-07T13:04:45.000Z'), value: 30 },
          { ts: Date.parse('2026-03-07T13:04:50.000Z'), value: 0 },
          { ts: Date.parse('2026-03-07T13:04:55.000Z'), value: 50 }
        ],
        load: [
          { ts: Date.parse('2026-03-07T13:04:45.000Z'), value: 130 },
          { ts: Date.parse('2026-03-07T13:04:50.000Z'), value: 0 },
          { ts: Date.parse('2026-03-07T13:04:55.000Z'), value: 150 }
        ]
      },
      now
    );

    expect(merged.solar.slice(-3)).toEqual([220, 230, 240]);
    expect(merged.ac.slice(-3)).toEqual([90, 100, 110]);
    expect(merged.dc.slice(-3)).toEqual([30, 40, 50]);
    expect(merged.load.slice(-3)).toEqual([130, 140, 150]);
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
