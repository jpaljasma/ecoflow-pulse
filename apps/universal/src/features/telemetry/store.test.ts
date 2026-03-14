import { afterEach, describe, expect, it } from 'vitest';
import { useTelemetryStore } from '@/features/telemetry/store';
import type { DeviceSnapshot, FleetTrendSnapshot } from '@/features/telemetry/engine/types';

const baseFleetTrend = (): FleetTrendSnapshot => ({
  load: Array.from({ length: 60 }, (_, index) => index),
  pv: Array.from({ length: 60 }, (_, index) => index + 1),
  ac: Array.from({ length: 60 }, (_, index) => index + 2),
  dc: Array.from({ length: 60 }, (_, index) => index + 3),
  filledPoints: 60
});

const sampleSnapshot = (): DeviceSnapshot => ({
  deviceId: 'dev-1',
  stale: false,
  inactive: false,
  online: true,
  lastSeenAt: 1234,
  metrics: {
    ts: 1234,
    online: true,
    soc: 50,
    pvW: 120,
    loadW: 80,
    batteryW: -20,
    tempC: 24,
    acW: 12,
    dcW: 8
  },
  status: 'charging',
  sparkline: {
    loadW: [{ ts: 1234, value: 80 }],
    pvW: [{ ts: 1234, value: 120 }],
    batteryW: [{ ts: 1234, value: -20 }],
    soc: [{ ts: 1234, value: 50 }],
    acW: [{ ts: 1234, value: 12 }],
    dcW: [{ ts: 1234, value: 8 }]
  }
});

afterEach(() => {
  useTelemetryStore.getState().reset();
});

describe('telemetry store', () => {
  it('resets live telemetry state back to the idle baseline', () => {
    useTelemetryStore.getState().setVisibleDeviceIds(['dev-1', 'dev-2']);
    useTelemetryStore.getState().setConnectionStatus('connected');
    useTelemetryStore.getState().updateSnapshots({
      snapshots: {
        'dev-1': sampleSnapshot()
      },
      fleetTrend: baseFleetTrend(),
      lastUpdatedAt: 1234,
      status: 'connected'
    });

    useTelemetryStore.getState().reset();

    const state = useTelemetryStore.getState();
    expect(state.visibleDeviceIds).toEqual([]);
    expect(state.snapshotByDeviceId).toEqual({});
    expect(state.connectionStatus).toBe('idle');
    expect(state.lastUpdatedAt).toBe(0);
    expect(state.fleetTrend.filledPoints).toBe(0);
    expect(state.fleetTrend.load.every((value) => value === 0)).toBe(true);
    expect(state.fleetTrend.pv.every((value) => value === 0)).toBe(true);
    expect(state.fleetTrend.ac.every((value) => value === 0)).toBe(true);
    expect(state.fleetTrend.dc.every((value) => value === 0)).toBe(true);
  });
});
