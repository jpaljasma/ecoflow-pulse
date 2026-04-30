import { describe, expect, it } from 'vitest';
import {
  buildSolarHistoryBounds,
  buildTodayBounds,
  historyRefreshIntervalMs,
  msUntilNextLocalDay,
  resolveSolarHistoryWindow,
  SOLAR_HISTORY_POINTS
} from '@/features/history/solar';

function restoreTZ(value: string | undefined): void {
  if (value === undefined) {
    delete process.env.TZ;
    return;
  }
  process.env.TZ = value;
}

describe('solar history helpers', () => {
  it('builds a local-day window from midnight to now', () => {
    const now = new Date('2026-03-06T15:23:53-05:00');
    const { from, to } = buildTodayBounds(now);
    const expectedFrom = new Date(now);
    expectedFrom.setHours(0, 0, 0, 0);

    expect(from.getTime()).toBe(expectedFrom.getTime());
    expect(to.getTime()).toBe(now.getTime());
  });

  it('uses a stable refresh interval per key', () => {
    const first = historyRefreshIntervalMs('device-a');
    const second = historyRefreshIntervalMs('device-a');
    const other = historyRefreshIntervalMs('device-b');

    expect(first).toBe(second);
    expect(first).toBeGreaterThanOrEqual(60_000);
    expect(first).toBeLessThan(67_500);
    expect(other).not.toBe(first);
  });

  it('computes the delay until the next local day rollover', () => {
    const now = new Date(2026, 2, 6, 23, 59, 30);
    expect(msUntilNextLocalDay(now)).toBe(31_000);
  });

  it('keeps the chart bucket count fixed', () => {
    expect(SOLAR_HISTORY_POINTS).toBe(84);
  });

  it('rounds a known sunrise and sunset to surrounding half-hour chart bounds', () => {
    const window = resolveSolarHistoryWindow(
      {
        timezone: 'America/New_York',
        daily: [
          {
            dateIso: '2026-04-29',
            sunriseIso: '2026-04-29T10:16:00Z',
            sunsetIso: '2026-04-30T00:01:00Z'
          }
        ]
      },
      new Date('2026-04-29T15:00:00Z')
    );

    expect(window).toEqual({
      startMinutes: 6 * 60,
      endMinutes: 20 * 60 + 30,
      points: 87,
      title: 'Solar Generated (6am-8:30pm, 10m buckets)'
    });
  });

  it('falls back to the fixed legacy chart window when sun times are unavailable', () => {
    expect(resolveSolarHistoryWindow(undefined, new Date('2026-04-29T15:00:00Z'))).toEqual({
      startMinutes: 6 * 60,
      endMinutes: 20 * 60,
      points: SOLAR_HISTORY_POINTS,
      title: 'Solar Generated (6am-8pm, 10m buckets)'
    });
  });

  it('builds a full previous local day across spring-forward', () => {
    const originalTZ = process.env.TZ;
    try {
      process.env.TZ = 'America/New_York';
      const now = new Date('2026-03-09T12:00:00-04:00');
      const { from, compareFrom, compareTo } = buildSolarHistoryBounds(now);

      expect(from.toISOString()).toBe('2026-03-09T04:00:00.000Z');
      expect(compareFrom.toISOString()).toBe('2026-03-08T05:00:00.000Z');
      expect(compareTo.toISOString()).toBe('2026-03-09T04:00:00.000Z');
    } finally {
      restoreTZ(originalTZ);
    }
  });

  it('builds a full previous local day across fall-back', () => {
    const originalTZ = process.env.TZ;
    try {
      process.env.TZ = 'America/New_York';
      const now = new Date('2026-11-02T12:00:00-05:00');
      const { from, compareFrom, compareTo } = buildSolarHistoryBounds(now);

      expect(from.toISOString()).toBe('2026-11-02T05:00:00.000Z');
      expect(compareFrom.toISOString()).toBe('2026-11-01T04:00:00.000Z');
      expect(compareTo.toISOString()).toBe('2026-11-02T05:00:00.000Z');
    } finally {
      restoreTZ(originalTZ);
    }
  });
});
