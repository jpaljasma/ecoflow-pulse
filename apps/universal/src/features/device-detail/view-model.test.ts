import { describe, expect, it } from 'vitest';

import {
  mergeDeviceDetailSolarPorts,
  resolveLiveBatteryHeatingOn,
  sumSolarPortWatts
} from '@/features/device-detail/liveDetail';
import { buildDeviceDetailSignalPills } from '@/features/device-detail/signalPills';
import type { DeviceSummary } from '@/features/devices/schema';

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
      { id: 'pv-3', name: 'PV 3', state: 'charging', volts: 38.1, amps: 4.07, watts: 155 }
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

  it('adds a solar passthrough system signal when PV covers load plus small self-load overhead', () => {
    const { signalPills } = buildDeviceDetailSignalPills({
      details: undefined,
      supportsEvCharging: false,
      supportsBatteryHeating: false,
      preconditioningOn: undefined,
      powerBalance: {
        acInW: 0,
        pvW: 706,
        loadW: 682,
        netW: 40
      }
    });

    expect(signalPills).toEqual([
      expect.objectContaining({
        key: 'solar-passthrough',
        label: 'Solar Passthrough',
        standalone: true,
        tone: 'success'
      })
    ]);
  });

  it('does not label solar passthrough when PV is rebuilding reserve', () => {
    const { signalPills } = buildDeviceDetailSignalPills({
      details: undefined,
      supportsEvCharging: false,
      supportsBatteryHeating: false,
      preconditioningOn: undefined,
      powerBalance: {
        acInW: 0,
        pvW: 920,
        loadW: 360,
        netW: 560
      }
    });

    expect(signalPills).not.toEqual(
      expect.arrayContaining([expect.objectContaining({ key: 'solar-passthrough' })])
    );
  });

  it('does not label solar passthrough when AC input is part of the balance', () => {
    const { signalPills } = buildDeviceDetailSignalPills({
      details: undefined,
      supportsEvCharging: false,
      supportsBatteryHeating: false,
      preconditioningOn: undefined,
      powerBalance: {
        acInW: 360,
        pvW: 360,
        loadW: 690,
        netW: 30
      }
    });

    expect(signalPills).not.toEqual(
      expect.arrayContaining([expect.objectContaining({ key: 'solar-passthrough' })])
    );
  });

  it('builds extended system signals and diagnostics from device details', () => {
    const details: DeviceSummary['details'] = {
      acOn: true,
      xBoostOn: true,
      solarMode: 'Solar Only',
      passthroughMode: 'L14 Transfer Switch',
      acAutoOnMode: 'Always On',
      energyManagementOn: true,
      diagnostics: [
        {
          key: 'dpu-sys-work-mode',
          label: 'System Work Mode',
          value: '2',
          tone: 'info'
        }
      ]
    };

    const { signalPills, diagnosticPills } = buildDeviceDetailSignalPills({
      details,
      supportsEvCharging: false,
      supportsBatteryHeating: false,
      preconditioningOn: undefined
    });

    expect(signalPills).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ key: 'ac', on: true, tone: 'success' }),
        expect.objectContaining({ key: 'xboost', on: true, tone: 'success' }),
        expect.objectContaining({ key: 'solar-mode', value: 'Solar Only', tone: 'success' }),
        expect.objectContaining({
          key: 'passthrough-mode',
          value: 'L14 Transfer Switch',
          tone: 'success'
        }),
        expect.objectContaining({
          key: 'ac-auto-on-mode',
          value: 'Always On',
          tone: 'success'
        }),
        expect.objectContaining({
          key: 'energy-mgmt',
          on: true,
          tone: 'success'
        })
      ])
    );
    expect(diagnosticPills).toEqual([
      expect.objectContaining({
        key: 'dpu-sys-work-mode',
        label: 'System Work Mode',
        value: '2',
        tone: 'info'
      })
    ]);
  });
});
