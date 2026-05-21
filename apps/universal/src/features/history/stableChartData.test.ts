import { describe, expect, it } from 'vitest';
import {
  hasNonZeroSeriesValue,
  selectStableChartData
} from '@/features/history/stableChartData';

describe('stable chart data selection', () => {
  const lastGood = { series: [0, 180, 220] };
  const empty = { series: [0, 0, 0] };
  const fresh = { series: [12, 24, 36] };

  it('keeps the last non-empty payload while the same chart scope is refetching', () => {
    const selected = selectStableChartData({
      data: empty,
      lastGoodData: lastGood,
      currentKey: 'fleet:today',
      lastGoodKey: 'fleet:today',
      isFetching: true,
      isError: false,
      isPlaceholderData: false,
      isUsable: (value) => hasNonZeroSeriesValue(value.series)
    });

    expect(selected).toBe(lastGood);
  });

  it('keeps the last non-empty payload after a transient error for the same chart scope', () => {
    const selected = selectStableChartData({
      data: undefined,
      lastGoodData: lastGood,
      currentKey: 'fleet:today',
      lastGoodKey: 'fleet:today',
      isFetching: false,
      isError: true,
      isPlaceholderData: false,
      isUsable: (value) => hasNonZeroSeriesValue(value.series)
    });

    expect(selected).toBe(lastGood);
  });

  it('uses fresh non-empty data immediately', () => {
    const selected = selectStableChartData({
      data: fresh,
      lastGoodData: lastGood,
      currentKey: 'fleet:today',
      lastGoodKey: 'fleet:today',
      isFetching: false,
      isError: false,
      isPlaceholderData: false,
      isUsable: (value) => hasNonZeroSeriesValue(value.series)
    });

    expect(selected).toBe(fresh);
  });

  it('does not carry cached data across a reset boundary', () => {
    const selected = selectStableChartData({
      data: empty,
      lastGoodData: lastGood,
      currentKey: 'fleet:tomorrow',
      lastGoodKey: 'fleet:today',
      isFetching: true,
      isError: false,
      isPlaceholderData: false,
      isUsable: (value) => hasNonZeroSeriesValue(value.series)
    });

    expect(selected).toBe(empty);
  });
});
