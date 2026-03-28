import { describe, expect, it } from 'vitest';
import { DeviceSchema } from '@/features/devices/schema';

describe('device api schema', () => {
  it('preserves extended system-signal and diagnostics detail fields', () => {
    const device = DeviceSchema.parse({
      id: 'device-1',
      serialNumber: 'delta-test',
      name: 'Bench Delta',
      model: 'Delta Pro Ultra',
      online: true,
      batteryPct: 78,
      state: 'idle',
      etaMinutes: 0,
      details: {
        xBoostOn: true,
        solarMode: 'Solar Only',
        passthroughMode: 'L14 Transfer Switch',
        acAutoOnMode: 'Always On',
        energyManagementOn: true,
        diagnostics: [
          {
            key: 'dpu-time-task-conflict',
            label: 'Time Task Conflict',
            value: 'No Conflict',
            tone: 'success'
          }
        ]
      }
    });

    expect(device.details).toEqual(
      expect.objectContaining({
        xBoostOn: true,
        solarMode: 'Solar Only',
        passthroughMode: 'L14 Transfer Switch',
        acAutoOnMode: 'Always On',
        energyManagementOn: true,
        diagnostics: [
          expect.objectContaining({
            key: 'dpu-time-task-conflict',
            label: 'Time Task Conflict',
            value: 'No Conflict',
            tone: 'success'
          })
        ]
      })
    );
  });
});
