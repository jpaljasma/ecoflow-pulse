import { describe, expect, it } from 'vitest';
import {
  buildTodayBounds,
  historyRefreshIntervalMs,
  msUntilNextLocalDay,
  SOLAR_HISTORY_POINTS
} from '@/features/history/solar';

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
    expect(SOLAR_HISTORY_POINTS).toBe(72);
  });
});
