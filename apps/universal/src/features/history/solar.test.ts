import { describe, expect, it } from 'vitest';
import {
  buildSolarHistoryBounds,
  buildTodayBounds,
  historyRefreshIntervalMs,
  msUntilNextLocalDay,
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
