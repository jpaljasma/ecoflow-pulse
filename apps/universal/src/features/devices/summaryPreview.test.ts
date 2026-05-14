import { describe, expect, it } from 'vitest';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';
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

  it('keeps authoritative device detail soc when live snapshot soc is stale', () => {
    const preview = buildFleetDevicePreview(
      [
        device({
          id: 'dpu-1',
          name: 'DPU',
          state: 'idle',
          batteryPct: 71,
          details: {
            overallSocPct: 71
          }
        })
      ],
      {
        maxItems: 1,
        snapshotsById: {
          'dpu-1': {
            deviceId: 'dpu-1',
            stale: false,
            inactive: false,
            online: true,
            lastSeenAt: 1,
            metrics: {
              ts: 1,
              online: true,
              soc: 98.8,
              pvW: 0,
              loadW: 0,
              batteryW: 0,
              tempC: 20,
              acW: 0,
              dcW: 0
            },
            status: 'idle',
            sparkline: { loadW: [], pvW: [], batteryW: [], soc: [], acW: [], dcW: [] }
          } satisfies DeviceSnapshot
        }
      }
    );

    expect(preview[0]?.batteryPct).toBe(71);
  });

  it('limits mobile previews to one row and tablet/desktop previews to two rows', () => {
    expect(resolveFleetDevicePreviewLimit(430)).toBe(2);
    expect(resolveFleetDevicePreviewLimit(768)).toBe(4);
    expect(resolveFleetDevicePreviewLimit(1280)).toBe(4);
  });
});
