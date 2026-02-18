import { create } from 'zustand';
import type { DeviceSnapshot, TelemetryEngineStatus } from '@/features/telemetry/engine/types';

type TelemetrySnapshotState = {
  visibleDeviceIds: string[];
  snapshotByDeviceId: Record<string, DeviceSnapshot>;
  lastUpdatedAt: number;
  connectionStatus: TelemetryEngineStatus;
  setVisibleDeviceIds: (ids: string[]) => void;
  setConnectionStatus: (status: TelemetryEngineStatus) => void;
  updateSnapshots: (payload: {
    snapshots: Record<string, DeviceSnapshot>;
    lastUpdatedAt: number;
    status: TelemetryEngineStatus;
  }) => void;
};

export const useTelemetryStore = create<TelemetrySnapshotState>((set) => ({
  visibleDeviceIds: [],
  snapshotByDeviceId: {},
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
  updateSnapshots: ({ snapshots, lastUpdatedAt, status }) =>
    set({
      snapshotByDeviceId: snapshots,
      lastUpdatedAt,
      connectionStatus: status
    })
}));
