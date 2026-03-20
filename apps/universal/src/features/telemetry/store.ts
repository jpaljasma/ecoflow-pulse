import type {
  DeviceLiveDetail,
  DeviceLiveSignals,
  DeviceLiveSolarPort,
  DeviceSnapshot,
  FleetTrendSnapshot,
  TelemetryEngineStatus
} from '@/features/telemetry/engine/types';
import { create } from '@/shared/state/zustand';

const DEFAULT_FLEET_TREND: FleetTrendSnapshot = {
  load: Array.from({ length: 60 }, () => 0),
  pv: Array.from({ length: 60 }, () => 0),
  ac: Array.from({ length: 60 }, () => 0),
  dc: Array.from({ length: 60 }, () => 0),
  filledPoints: 0
};

function numberArrayEqual(a: number[], b: number[]): boolean {
  if (a === b) return true;
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i += 1) {
    if (a[i] !== b[i]) return false;
  }
  return true;
}

function fleetTrendEqual(a: FleetTrendSnapshot, b: FleetTrendSnapshot): boolean {
  return (
    numberArrayEqual(a.load, b.load) &&
    numberArrayEqual(a.pv, b.pv) &&
    numberArrayEqual(a.ac, b.ac) &&
    numberArrayEqual(a.dc, b.dc) &&
    a.filledPoints === b.filledPoints
  );
}

function pointSeriesEqual(
  a: DeviceSnapshot['sparkline'][keyof DeviceSnapshot['sparkline']],
  b: DeviceSnapshot['sparkline'][keyof DeviceSnapshot['sparkline']]
): boolean {
  if (a === b) return true;
  if (a.length !== b.length) return false;
  if (a.length === 0) return true;
  const mid = Math.floor(a.length / 2);
  const checks = [0, mid, a.length - 1];
  for (const idx of checks) {
    if (a[idx]?.ts !== b[idx]?.ts || a[idx]?.value !== b[idx]?.value) return false;
  }
  return true;
}

function liveSignalsEqual(a: DeviceLiveSignals | undefined, b: DeviceLiveSignals | undefined): boolean {
  if (a === b) return true;
  if (!a || !b) return !a && !b;
  return (
    a.acOn === b.acOn &&
    a.dcOn === b.dcOn &&
    a.usbOn === b.usbOn &&
    a.dc12vOn === b.dc12vOn &&
    a.evChargingOn === b.evChargingOn &&
    a.fanOn === b.fanOn &&
    a.solarChargingOn === b.solarChargingOn &&
    a.batteryHeatingOn === b.batteryHeatingOn
  );
}

function solarPortsEqual(a: DeviceLiveSolarPort[] | undefined, b: DeviceLiveSolarPort[] | undefined): boolean {
  if (a === b) return true;
  const left = a ?? [];
  const right = b ?? [];
  if (left.length !== right.length) return false;
  for (let i = 0; i < left.length; i += 1) {
    const l = left[i];
    const r = right[i];
    if (
      !l ||
      !r ||
      l.id !== r.id ||
      l.name !== r.name ||
      l.state !== r.state ||
      l.volts !== r.volts ||
      l.amps !== r.amps ||
      l.watts !== r.watts
    ) {
      return false;
    }
  }
  return true;
}

function liveDetailEqual(a: DeviceLiveDetail | undefined, b: DeviceLiveDetail | undefined): boolean {
  if (a === b) return true;
  if (!a || !b) return !a && !b;
  return liveSignalsEqual(a.signals, b.signals) && solarPortsEqual(a.solarPorts, b.solarPorts);
}

function snapshotEqual(a: DeviceSnapshot | undefined, b: DeviceSnapshot): boolean {
  if (!a) return false;
  if (
    a.stale !== b.stale ||
    a.inactive !== b.inactive ||
    a.online !== b.online ||
    a.lastSeenAt !== b.lastSeenAt ||
    a.status !== b.status ||
    !liveDetailEqual(a.liveDetail, b.liveDetail)
  ) {
    return false;
  }
  const am = a.metrics;
  const bm = b.metrics;
  if (Boolean(am) !== Boolean(bm)) return false;
  if (am && bm) {
    if (
      am.ts !== bm.ts ||
      am.online !== bm.online ||
      am.soc !== bm.soc ||
      am.pvW !== bm.pvW ||
      am.loadW !== bm.loadW ||
      am.batteryW !== bm.batteryW ||
      am.tempC !== bm.tempC ||
      am.acW !== bm.acW ||
      am.dcW !== bm.dcW
    ) {
      return false;
    }
  }

  return (
    pointSeriesEqual(a.sparkline.loadW, b.sparkline.loadW) &&
    pointSeriesEqual(a.sparkline.pvW, b.sparkline.pvW) &&
    pointSeriesEqual(a.sparkline.batteryW, b.sparkline.batteryW) &&
    pointSeriesEqual(a.sparkline.soc, b.sparkline.soc) &&
    pointSeriesEqual(a.sparkline.acW, b.sparkline.acW) &&
    pointSeriesEqual(a.sparkline.dcW, b.sparkline.dcW)
  );
}

type TelemetrySnapshotState = {
  visibleDeviceIds: string[];
  snapshotByDeviceId: Record<string, DeviceSnapshot>;
  fleetTrend: FleetTrendSnapshot;
  lastUpdatedAt: number;
  connectionStatus: TelemetryEngineStatus;
  setVisibleDeviceIds: (ids: string[]) => void;
  setConnectionStatus: (status: TelemetryEngineStatus) => void;
  reset: () => void;
  updateSnapshots: (payload: {
    snapshots: Record<string, DeviceSnapshot>;
    fleetTrend: FleetTrendSnapshot;
    lastUpdatedAt: number;
    status: TelemetryEngineStatus;
  }) => void;
};

export const useTelemetryStore = create<TelemetrySnapshotState>((set) => ({
  visibleDeviceIds: [],
  snapshotByDeviceId: {},
  fleetTrend: DEFAULT_FLEET_TREND,
  lastUpdatedAt: 0,
  connectionStatus: 'idle',
  setVisibleDeviceIds: (ids) =>
    set((state) => {
      if (state.visibleDeviceIds.length === ids.length && state.visibleDeviceIds.every((id, idx) => id === ids[idx])) {
        return state;
      }
      return { visibleDeviceIds: ids };
    }),
  setConnectionStatus: (status) =>
    set((state) => {
      if (state.connectionStatus === status) return state;
      return { connectionStatus: status };
    }),
  reset: () =>
    set({
      visibleDeviceIds: [],
      snapshotByDeviceId: {},
      fleetTrend: DEFAULT_FLEET_TREND,
      lastUpdatedAt: 0,
      connectionStatus: 'idle'
    }),
  updateSnapshots: ({ snapshots, fleetTrend, lastUpdatedAt, status }) =>
    set((state) => {
      let snapshotsChanged = false;
      const nextById: Record<string, DeviceSnapshot> = { ...state.snapshotByDeviceId };
      for (const [deviceId, incoming] of Object.entries(snapshots)) {
        const prev = state.snapshotByDeviceId[deviceId];
        if (snapshotEqual(prev, incoming)) {
          nextById[deviceId] = prev as DeviceSnapshot;
          continue;
        }
        nextById[deviceId] = incoming;
        snapshotsChanged = true;
      }

      const nextLastUpdatedAt = lastUpdatedAt > state.lastUpdatedAt ? lastUpdatedAt : state.lastUpdatedAt;
      const nextStatus = status;
      const nextFleetTrend = fleetTrendEqual(state.fleetTrend, fleetTrend) ? state.fleetTrend : fleetTrend;
      const statusChanged = nextStatus !== state.connectionStatus;
      const lastUpdatedChanged = nextLastUpdatedAt !== state.lastUpdatedAt;
      const fleetTrendChanged = nextFleetTrend !== state.fleetTrend;

      if (!snapshotsChanged && !statusChanged && !lastUpdatedChanged && !fleetTrendChanged) {
        return state;
      }

      return {
        snapshotByDeviceId: snapshotsChanged ? nextById : state.snapshotByDeviceId,
        fleetTrend: nextFleetTrend,
        lastUpdatedAt: nextLastUpdatedAt,
        connectionStatus: nextStatus
      };
    })
}));
