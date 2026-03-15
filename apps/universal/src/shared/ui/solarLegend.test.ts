import { describe, expect, it } from 'vitest';

import { formatSolarLegendDelta } from '@/shared/ui/solarLegend';

describe('formatSolarLegendDelta', () => {
  it('shows percentage deltas when yesterday clears the baseline floor', () => {
    expect(formatSolarLegendDelta(5610, 1000, 461)).toBe(' (+461%)');
  });

  it('shows absolute change when yesterday is below the baseline floor', () => {
    expect(formatSolarLegendDelta(5610, 1, 1003228)).toBe(' (+5.61 kWh)');
  });

  it('shows new activity text when yesterday is zero', () => {
    expect(formatSolarLegendDelta(5610, 0, 1003228)).toBe(' (new activity today)');
  });
});
