import { describe, expect, it } from 'vitest';

import {
  mergeDeviceDetailSolarPorts,
  resolveLiveBatteryHeatingOn,
  sumSolarPortWatts
} from '@/features/device-detail/liveDetail';

describe('device detail live-detail helpers', () => {
  it('prefers live battery-heating state over stale REST pack heating flags', () => {
    const resolved = resolveLiveBatteryHeatingOn(
      { batteryHeatingOn: false },
      {
        batteryHeatingOn: true,
        packs: [{ id: 'bp1', heatingOn: true }]
      }
    );

    expect(resolved).toBe(false);
  });

  it('merges live solar port values over matching static port ids', () => {
    const merged = mergeDeviceDetailSolarPorts(
      [
        {
          id: 'pv-low',
          name: 'PV Low',
          state: 'charging',
          volts: 64.2,
          amps: 1.2,
          watts: 77,
          maxVolts: 150,
          maxAmps: 15,
          maxWatts: 1600
        },
        {
          id: 'pv-high',
          name: 'PV High',
          state: 'inactive',
          volts: 0,
          amps: 0,
          watts: 0,
          maxVolts: 450,
          maxAmps: 15,
          maxWatts: 4000
        }
      ],
      [{ id: 'pv-low', name: 'PV Low', state: 'idle', volts: 63.9, amps: 0, watts: 0 }]
    );

    expect(merged).toEqual([
      expect.objectContaining({
        id: 'pv-low',
        state: 'idle',
        volts: 63.9,
        amps: 0,
        watts: 0,
        maxWatts: 1600
      }),
      expect.objectContaining({
        id: 'pv-high',
        state: 'inactive',
        watts: 0,
        maxWatts: 4000
      })
    ]);
  });

  it('falls back to index-based solar-port merges when live ids differ from static ids', () => {
    const merged = mergeDeviceDetailSolarPorts(
      [
        {
          id: 'pv-1',
          name: 'PV 1',
          state: 'idle',
          volts: 16.4,
          amps: 5,
          watts: 82,
          maxVolts: 60,
          maxAmps: 15,
          maxWatts: 500
        },
        {
          id: 'pv-2',
          name: 'PV 2',
          state: 'idle',
          volts: 40.9,
          amps: 5.18,
          watts: 212,
          maxVolts: 60,
          maxAmps: 15,
          maxWatts: 500
        }
      ],
      [
        { id: 'pv-low', name: 'PV Low', state: 'charging', volts: 18, amps: 4.2, watts: 76 },
        { id: 'pv-high', name: 'PV High', state: 'inactive', volts: 0, amps: 0, watts: 0 }
      ]
    );

    expect(merged).toEqual([
      expect.objectContaining({
        id: 'pv-1',
        name: 'PV 1',
        state: 'charging',
        volts: 18,
        amps: 4.2,
        watts: 76
      }),
      expect.objectContaining({
        id: 'pv-2',
        name: 'PV 2',
        state: 'inactive',
        volts: 0,
        amps: 0,
        watts: 0
      })
    ]);
  });

  it('sums live solar port watts for display-total consistency', () => {
    expect(
      sumSolarPortWatts([
        { watts: 6 },
        { watts: 14 }
      ])
    ).toBe(20);
  });

  it('uses generic PV numbering for live-only ports beyond pv-2', () => {
    const merged = mergeDeviceDetailSolarPorts(undefined, [
      { id: 'pv-3', state: 'charging', volts: 38.1, amps: 4.07, watts: 155 }
    ]);

    expect(merged).toEqual([
      expect.objectContaining({
        id: 'pv-3',
        name: 'PV 3',
        state: 'charging',
        watts: 155
      })
    ]);
  });
});
