import { describe, expect, it } from 'vitest';
import {
  buildCompareSolarHistoryView,
  combineSolarHistoryViews,
  emptySolarHistoryView
} from '../src/history/solarView.js';
import type { CompareRollupSeries, RollupPoint, RollupSeries } from '../src/grpc/telemetryClient.js';

function point({
  bucketStartUnixMs,
  bucketEndUnixMs,
  solarGeneratedWh,
  pvAvgW = 0
}: {
  bucketStartUnixMs: number;
  bucketEndUnixMs: number;
  solarGeneratedWh: number;
  pvAvgW?: number;
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
      acOutputAvgW: 0,
      acOutputMaxW: 0,
      pvAvgW,
      pvMaxW: 0,
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
      solarGeneratedWh,
      acInputEnergyWh: 0,
      acOutputEnergyWh: 0,
      dcOutputEnergyWh: 0,
      loadEnergyWh: 0,
      batteryChargeEnergyWh: 0,
      batteryDischargeEnergyWh: 0
    }
  };
}

function series(points: RollupPoint[], fromUnixMs: number): RollupSeries {
  return {
    deviceId: '22222222-2222-7222-8222-222222222222',
    resolution: 'minute',
    fromUnixMs: String(fromUnixMs),
    toUnixMs: points.at(-1)?.bucketEndUnixMs ?? String(fromUnixMs),
    points
  };
}

describe('backend solar history view', () => {
  it('builds 10-minute buckets relative to the request window start', () => {
    const fromUnixMs = Date.UTC(2026, 2, 6, 5, 0, 0);
    const current = series(
      [
        point({
          bucketStartUnixMs: fromUnixMs + 6 * 60 * 60 * 1000,
          bucketEndUnixMs: fromUnixMs + 6 * 60 * 60 * 1000 + 60_000,
          solarGeneratedWh: 2
        }),
        point({
          bucketStartUnixMs: fromUnixMs + 6 * 60 * 60 * 1000 + 10 * 60_000,
          bucketEndUnixMs: fromUnixMs + 6 * 60 * 60 * 1000 + 11 * 60_000,
          solarGeneratedWh: 3
        })
      ],
      fromUnixMs
    );
    const previous = series(
      [
        point({
          bucketStartUnixMs: fromUnixMs - 24 * 60 * 60 * 1000 + 6 * 60 * 60 * 1000,
          bucketEndUnixMs: fromUnixMs - 24 * 60 * 60 * 1000 + 6 * 60 * 60 * 1000 + 60_000,
          solarGeneratedWh: 1
        })
      ],
      fromUnixMs - 24 * 60 * 60 * 1000
    );

    const view = buildCompareSolarHistoryView({ current, previous } satisfies CompareRollupSeries);

    expect(view.todayWh).toBe(5);
    expect(view.yesterdayWh).toBe(1);
    expect(view.deltaPct).toBe(400);
    expect(view.seriesWh[0]).toBe(2);
    expect(view.seriesWh[1]).toBe(3);
    expect(view.yesterdaySeriesWh[0]).toBe(1);
  });

  it('covers the 6am-8pm window and excludes 8pm+', () => {
    const fromUnixMs = Date.UTC(2026, 2, 6, 5, 0, 0);
    const current = series(
      [
        point({
          bucketStartUnixMs: fromUnixMs + 19 * 60 * 60 * 1000 + 50 * 60_000,
          bucketEndUnixMs: fromUnixMs + 19 * 60 * 60 * 1000 + 51 * 60_000,
          solarGeneratedWh: 4
        }),
        point({
          bucketStartUnixMs: fromUnixMs + 20 * 60 * 60 * 1000,
          bucketEndUnixMs: fromUnixMs + 20 * 60 * 60 * 1000 + 60_000,
          solarGeneratedWh: 9
        })
      ],
      fromUnixMs
    );

    const view = buildCompareSolarHistoryView({
      current,
      previous: series([], fromUnixMs - 24 * 60 * 60 * 1000)
    } satisfies CompareRollupSeries);

    expect(view.seriesWh).toHaveLength(84);
    expect(view.seriesWh[83]).toBe(4);
    expect(view.seriesWh).not.toContain(9);
  });

  it('combines fleet views server-side', () => {
    const combined = combineSolarHistoryViews([
      {
        todayWh: 100,
        yesterdayWh: 80,
        deltaPct: 25,
        seriesWh: [100, 0, 0],
        yesterdaySeriesWh: [60, 20, 0]
      },
      {
        todayWh: 50,
        yesterdayWh: 20,
        deltaPct: 150,
        seriesWh: [0, 50, 0],
        yesterdaySeriesWh: [0, 20, 0]
      }
    ]);

    expect(combined.todayWh).toBe(150);
    expect(combined.yesterdayWh).toBe(100);
    expect(combined.deltaPct).toBe(50);
    expect(combined.seriesWh.slice(0, 3)).toEqual([100, 50, 0]);
    expect(combined.yesterdaySeriesWh.slice(0, 3)).toEqual([60, 40, 0]);
  });

  it('returns an empty default view', () => {
    expect(emptySolarHistoryView()).toEqual({
      todayWh: 0,
      yesterdayWh: 0,
      deltaPct: null,
      seriesWh: Array.from({ length: 84 }, () => 0),
      yesterdaySeriesWh: Array.from({ length: 84 }, () => 0)
    });
  });

  it('falls back to bucket power when stored solar energy undercounts sparse samples', () => {
    const fromUnixMs = Date.UTC(2026, 2, 6, 5, 0, 0);
    const current = series(
      [
        point({
          bucketStartUnixMs: fromUnixMs + 13 * 60 * 60 * 1000,
          bucketEndUnixMs: fromUnixMs + 13 * 60 * 60 * 1000 + 60_000,
          solarGeneratedWh: 2,
          pvAvgW: 360
        })
      ],
      fromUnixMs
    );

    const view = buildCompareSolarHistoryView({
      current,
      previous: series([], fromUnixMs - 24 * 60 * 60 * 1000)
    } satisfies CompareRollupSeries);

    expect(view.todayWh).toBeCloseTo(6, 5);
    expect(view.seriesWh[42]).toBeCloseTo(6, 5);
  });
});
