import { useEffect, useMemo } from 'react';
import { useShallow } from 'zustand/react/shallow';
import { useTelemetryEngine } from '@/features/telemetry/TelemetryEngineContext';
import { useTelemetryStore } from '@/features/telemetry/store';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';

function useStableDeviceIds(deviceIds: string[]): string[] {
  const idsKey = useMemo(() => [...new Set(deviceIds)].sort().join(','), [deviceIds]);
  return useMemo(() => (idsKey ? idsKey.split(',') : []), [idsKey]);
}

export function useTelemetrySubscription(deviceIds: string[]) {
  const engine = useTelemetryEngine();
  const setVisibleDeviceIds = useTelemetryStore((s) => s.setVisibleDeviceIds);
  const setConnectionStatus = useTelemetryStore((s) => s.setConnectionStatus);
  const updateSnapshots = useTelemetryStore((s) => s.updateSnapshots);
  const stableIds = useStableDeviceIds(deviceIds);

  useEffect(() => {
    // Idempotent safety call: ensures engine clocks/socket are running
    // even after route/hot-reload edge cases.
    engine.connect();
    // Seed current engine status immediately so UI doesn't stay on initial "idle"
    // when provider connected before listeners were attached.
    setConnectionStatus(engine.getStatus());
    const unsubSnapshot = engine.onSnapshot(updateSnapshots);
    const unsubStatus = engine.onStatus(setConnectionStatus);
    return () => {
      unsubSnapshot();
      unsubStatus();
    };
  }, [engine, setConnectionStatus, updateSnapshots]);

  useEffect(() => {
    setVisibleDeviceIds(stableIds);
    engine.subscribe(stableIds);

    return () => {
      engine.unsubscribe(stableIds);
    };
  }, [stableIds, engine, setVisibleDeviceIds]);
}

export function useTelemetryConnectionStatus() {
  return useTelemetryStore((s) => s.connectionStatus);
}

export function useTelemetryLastUpdatedAt() {
  return useTelemetryStore((s) => s.lastUpdatedAt);
}

export function useTelemetryFleetTrend() {
  return useTelemetryStore((s) => s.fleetTrend);
}

export function useTelemetryDeviceSnapshot(deviceId: string | undefined): DeviceSnapshot | undefined {
  return useTelemetryStore(
    useMemo(
      () => (state) => (deviceId ? state.snapshotByDeviceId[deviceId] : undefined),
      [deviceId]
    )
  );
}

export function useTelemetrySnapshotsByIds(deviceIds: string[]): Record<string, DeviceSnapshot> {
  const stableIds = useStableDeviceIds(deviceIds);
  const snapshots = useTelemetryStore(
    useShallow(
      useMemo(
        () => (state) => stableIds.map((id) => state.snapshotByDeviceId[id]),
        [stableIds]
      )
    )
  );
  return useMemo(
    () =>
      stableIds.reduce<Record<string, DeviceSnapshot>>((acc, id, idx) => {
        const snapshot = snapshots[idx];
        if (snapshot) acc[id] = snapshot;
        return acc;
      }, {}),
    [snapshots, stableIds]
  );
}

export function useTelemetrySnapshot(deviceIds: string[]) {
  useTelemetrySubscription(deviceIds);
  const byId = useTelemetrySnapshotsByIds(deviceIds);
  const stableIds = useStableDeviceIds(deviceIds);
  const connectionStatus = useTelemetryConnectionStatus();
  const lastUpdatedAt = useTelemetryLastUpdatedAt();
  const data = useMemo(() => stableIds.map((id) => byId[id]).filter(Boolean), [stableIds, byId]);

  return { data, byId, connectionStatus, lastUpdatedAt };
}
