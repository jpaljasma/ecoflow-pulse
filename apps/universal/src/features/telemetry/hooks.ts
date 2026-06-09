import { useEffect, useMemo } from 'react';
// eslint-disable-next-line @typescript-eslint/no-require-imports
const { useShallow } = require('zustand/react/shallow') as typeof import('zustand/react/shallow');
import { useTelemetryEngine } from '@/features/telemetry/TelemetryEngineContext';
import { useTelemetryStore } from '@/features/telemetry/store';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';

export type TelemetrySubscriptionOptions = {
  active?: boolean;
};

export function normalizeTelemetryDeviceIds(deviceIds: readonly string[]): string[] {
  return [...new Set(deviceIds)].sort();
}

export function resolveTelemetrySubscriptionDeviceIds(
  deviceIds: readonly string[],
  options: TelemetrySubscriptionOptions = {}
): string[] {
  return options.active === false ? [] : normalizeTelemetryDeviceIds(deviceIds);
}

function useStableDeviceIds(deviceIds: string[]): string[] {
  const idsKey = useMemo(() => normalizeTelemetryDeviceIds(deviceIds).join(','), [deviceIds]);
  return useMemo(() => (idsKey ? idsKey.split(',') : []), [idsKey]);
}

export function useTelemetrySubscription(deviceIds: string[], options: TelemetrySubscriptionOptions = {}) {
  const engine = useTelemetryEngine();
  const setVisibleDeviceIds = useTelemetryStore((s) => s.setVisibleDeviceIds);
  const setConnectionStatus = useTelemetryStore((s) => s.setConnectionStatus);
  const updateSnapshots = useTelemetryStore((s) => s.updateSnapshots);
  const stableIds = useStableDeviceIds(deviceIds);
  const active = options.active ?? true;
  const subscriptionIds = useMemo(
    () => resolveTelemetrySubscriptionDeviceIds(stableIds, { active }),
    [active, stableIds]
  );

  useEffect(() => {
    setConnectionStatus(engine.getStatus());
    const unsubSnapshot = engine.onSnapshot(updateSnapshots);
    const unsubStatus = engine.onStatus(setConnectionStatus);
    return () => {
      unsubSnapshot();
      unsubStatus();
    };
  }, [engine, setConnectionStatus, updateSnapshots]);

  useEffect(() => {
    setVisibleDeviceIds(subscriptionIds);
    engine.subscribe(subscriptionIds);

    return () => {
      engine.unsubscribe(subscriptionIds);
    };
  }, [subscriptionIds, engine, setVisibleDeviceIds]);
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

export function useTelemetrySnapshot(deviceIds: string[], options: TelemetrySubscriptionOptions = {}) {
  useTelemetrySubscription(deviceIds, options);
  const byId = useTelemetrySnapshotsByIds(deviceIds);
  const stableIds = useStableDeviceIds(deviceIds);
  const connectionStatus = useTelemetryConnectionStatus();
  const lastUpdatedAt = useTelemetryLastUpdatedAt();
  const data = useMemo(() => stableIds.map((id) => byId[id]).filter(Boolean), [stableIds, byId]);

  return { data, byId, connectionStatus, lastUpdatedAt };
}
