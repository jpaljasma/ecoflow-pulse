import { describe, expect, it } from 'vitest';
import { buildInventoryMetricLayout, buildStatusDotHoverLabel } from '@/features/devices/deviceCardLayout';

describe('buildInventoryMetricLayout', () => {
  it('keeps live metrics in a compact two-row matrix', () => {
    expect(buildInventoryMetricLayout()).toEqual({
      columns: 6,
      items: [
        { key: 'ac', span: 2 },
        { key: 'dc', span: 2 },
        { key: 'pv', span: 2 },
        { key: 'load', span: 3 },
        { key: 'net', span: 3 }
      ]
    });
  });

  it('keeps last-seen copy available through the status dot hover label', () => {
    expect(buildStatusDotHoverLabel('Last seen 7s ago', null)).toBe('Last seen 7s ago');
    expect(buildStatusDotHoverLabel('Last seen 2m ago', 'Offline')).toBe('Offline · Last seen 2m ago');
  });
});
