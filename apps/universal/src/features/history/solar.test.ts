import { describe, expect, it } from 'vitest';
import { buildTodayBounds, historyRefreshIntervalMs, SOLAR_HISTORY_POINTS } from '@/features/history/solar';

describe('solar history helpers', () => {
  it('builds a local-day window from midnight to now', () => {
    const now = new Date('2026-03-06T15:23:53-05:00');
    const { from, to } = buildTodayBounds(now);

    expect(from.toISOString()).toBe('2026-03-06T05:00:00.000Z');
    expect(to.toISOString()).toBe('2026-03-06T20:23:53.000Z');
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

  it('keeps the chart bucket count fixed', () => {
    expect(SOLAR_HISTORY_POINTS).toBe(72);
  });
});
