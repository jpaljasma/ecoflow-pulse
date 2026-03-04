import { describe, expect, it } from 'vitest';
import {
  buildCompareSolarHistoryView,
  buildSolarHistoryView,
  combineSolarHistoryViews
} from '@/features/history/solar';
import type { CompareRollupSeries, RollupPoint, RollupSeries } from '@/features/history/api';

function point({
  bucketStartUnixMs,
  bucketEndUnixMs,
  pvAvgW,
  solarGeneratedWh
}: {
  bucketStartUnixMs: number;
  bucketEndUnixMs: number;
  pvAvgW: number;
  solarGeneratedWh?: number;
}): RollupPoint {
  return {
    bucketStartUnixMs: String(bucketStartUnixMs),
    bucketEndUnixMs: String(bucketEndUnixMs),
    sampleCount: 1,
    firstTsUnixMs: String(bucketStartUnixMs),
    lastTsUnixMs: String(bucketEndUnixMs),
    metrics: {
      socAvgPct: 0,
      socMinPct: 0,
      socMaxPct: 0,
      acInAvgW: 0,
      acInMaxW: 0,
      pvAvgW,
      pvMaxW: pvAvgW,
      dcAvgW: 0,
      dcMaxW: 0,
      loadAvgW: 0,
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
      solarGeneratedWh: solarGeneratedWh ?? 0
    }
  };
}

function series(points: RollupPoint[]): RollupSeries {
  return {
    deviceId: '019cab9d-bcab-75c0-9c02-db3ae1105d61',
    resolution: 'minute',
    fromUnixMs: points[0]?.bucketStartUnixMs ?? '0',
    toUnixMs: points.at(-1)?.bucketEndUnixMs ?? '0',
    points
  };
}

describe('solar history helpers', () => {
  it('falls back to pvAvgW when solarGeneratedWh is missing', () => {
    const view = buildSolarHistoryView([
      point({
        bucketStartUnixMs: Date.UTC(2026, 2, 3, 12, 0, 0),
        bucketEndUnixMs: Date.UTC(2026, 2, 3, 12, 1, 0),
        pvAvgW: 120
      })
    ]);

    expect(view.todayWh).toBeCloseTo(2, 6);
    expect(view.seriesWh.reduce((sum, value) => sum + value, 0)).toBeCloseTo(2, 6);
  });

  it('clamps fallback pvAvgW to the physical PV limit', () => {
    const view = buildSolarHistoryView(
      [
        point({
          bucketStartUnixMs: Date.UTC(2026, 2, 3, 12, 0, 0),
          bucketEndUnixMs: Date.UTC(2026, 2, 3, 12, 1, 0),
          pvAvgW: 12_000
        })
      ],
      { maxSolarWatts: 1000 }
    );

    expect(view.todayWh).toBeCloseTo(1000 / 60, 6);
  });

  it('prefers explicit solarGeneratedWh when present', () => {
    const view = buildSolarHistoryView([
      point({
        bucketStartUnixMs: Date.UTC(2026, 2, 3, 12, 0, 0),
        bucketEndUnixMs: Date.UTC(2026, 2, 3, 12, 1, 0),
        pvAvgW: 120,
        solarGeneratedWh: 5
      })
    ]);

    expect(view.todayWh).toBe(5);
    expect(view.seriesWh.reduce((sum, value) => sum + value, 0)).toBe(5);
  });

  it('computes compare delta from derived energy', () => {
    const current = series([
      point({
        bucketStartUnixMs: Date.UTC(2026, 2, 3, 12, 0, 0),
        bucketEndUnixMs: Date.UTC(2026, 2, 3, 12, 1, 0),
        pvAvgW: 120
      })
    ]);
    const previous = series([
      point({
        bucketStartUnixMs: Date.UTC(2026, 2, 2, 12, 0, 0),
        bucketEndUnixMs: Date.UTC(2026, 2, 2, 12, 1, 0),
        pvAvgW: 60
      })
    ]);

    const view = buildCompareSolarHistoryView({ current, previous } satisfies CompareRollupSeries);

    expect(view.todayWh).toBeCloseTo(2, 6);
    expect(view.yesterdayWh).toBeCloseTo(1, 6);
    expect(view.deltaPct).toBeCloseTo(100, 6);
  });

  it('combines today and yesterday energy across fleet views', () => {
    const combined = combineSolarHistoryViews([
      { todayWh: 100, yesterdayWh: 80, deltaPct: 25, seriesWh: [100, 0, 0] },
      { todayWh: 50, yesterdayWh: 20, deltaPct: 150, seriesWh: [0, 50, 0] }
    ]);

    expect(combined.todayWh).toBe(150);
    expect(combined.yesterdayWh).toBe(100);
    expect(combined.deltaPct).toBeCloseTo(50, 6);
    expect(combined.seriesWh.slice(0, 3)).toEqual([100, 50, 0]);
  });
});
