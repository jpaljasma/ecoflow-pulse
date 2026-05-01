import { describe, expect, it } from 'vitest';
import type { DeviceSummary } from '@/features/devices/api';
import {
  buildFleetDevicePreview,
  resolveFleetDevicePreviewLimit
} from '@/features/devices/summaryPreview';

function device(overrides: Partial<DeviceSummary> & Pick<DeviceSummary, 'id' | 'name' | 'state'>): DeviceSummary {
  return {
    serialNumber: `SN-${overrides.id}`,
    model: 'DELTA Pro Ultra',
    online: true,
    batteryPct: 50,
    etaMinutes: 0,
    ...overrides
  };
}

describe('fleet summary device preview', () => {
  it('shows charging and discharging devices before idle devices without shuffling each group', () => {
    const preview = buildFleetDevicePreview(
      [
        device({ id: 'idle-1', name: 'Idle first', state: 'idle', batteryPct: 18 }),
        device({ id: 'charging-1', name: 'Charging first', state: 'charging', batteryPct: 63 }),
        device({ id: 'discharging-1', name: 'Discharging first', state: 'discharging', batteryPct: 41 }),
        device({ id: 'idle-2', name: 'Idle second', state: 'idle', batteryPct: 92 }),
        device({ id: 'charging-2', name: 'Charging second', state: 'charging', batteryPct: 77 })
      ],
      { maxItems: 4 }
    );

    expect(preview.map((item) => item.id)).toEqual([
      'charging-1',
      'discharging-1',
      'charging-2',
      'idle-1'
    ]);
    expect(preview.map((item) => item.chargeState)).toEqual([
      'charging',
      'discharging',
      'charging',
      'neutral'
    ]);
  });

  it('limits mobile previews to one row and tablet/desktop previews to two rows', () => {
    expect(resolveFleetDevicePreviewLimit(430)).toBe(2);
    expect(resolveFleetDevicePreviewLimit(768)).toBe(4);
    expect(resolveFleetDevicePreviewLimit(1280)).toBe(4);
  });
});
