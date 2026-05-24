import { describe, expect, it } from 'vitest';
import { resolveDeviceInventoryGrid } from '@/features/devices/inventoryGrid';

describe('resolveDeviceInventoryGrid', () => {
  it('uses one equal-width card on phones', () => {
    expect(resolveDeviceInventoryGrid(640, 16)).toEqual({
      columns: 1,
      cardWidth: 608,
      compactCard: false
    });
  });

  it('uses two equal-width cards on tablets', () => {
    expect(resolveDeviceInventoryGrid(900, 20)).toEqual({
      columns: 2,
      cardWidth: 422,
      compactCard: true
    });
  });

  it('uses three equal-width cards on desktop', () => {
    expect(resolveDeviceInventoryGrid(1280, 24)).toEqual({
      columns: 3,
      cardWidth: 400,
      compactCard: true
    });
  });
});
