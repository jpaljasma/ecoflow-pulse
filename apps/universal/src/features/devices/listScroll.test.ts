import { describe, expect, it } from 'vitest';

import { findDeviceScrollIndex } from '@/features/devices/listScroll';
import type { DeviceSummary } from '@/features/devices/schema';

function makeDevice(id: string): DeviceSummary {
  return {
    id,
    serialNumber: `SN-${id}`,
    name: `Device ${id}`,
    model: 'Test Model',
    online: true,
    batteryPct: 50,
    state: 'idle',
    etaMinutes: 0
  };
}

describe('findDeviceScrollIndex', () => {
  it('finds the newly enabled device in the updated configured-device list', () => {
    const devices = [makeDevice('alpha'), makeDevice('enabled-device'), makeDevice('omega')];

    expect(findDeviceScrollIndex(devices, 'enabled-device')).toBe(1);
  });

  it('falls back to the top of the list when the highlighted device is not present yet', () => {
    expect(findDeviceScrollIndex([makeDevice('alpha')], 'enabled-device')).toBe(0);
  });
});
