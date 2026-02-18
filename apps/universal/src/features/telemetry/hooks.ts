import { useEffect, useMemo } from 'react';
import { useTelemetryEngine } from '@/features/telemetry/TelemetryEngineContext';
import { useTelemetryStore } from '@/features/telemetry/store';

export function useTelemetrySnapshot(deviceIds: string[]) {
  const engine = useTelemetryEngine();
  const setVisibleDeviceIds = useTelemetryStore((s) => s.setVisibleDeviceIds);
  const setConnectionStatus = useTelemetryStore((s) => s.setConnectionStatus);
  const updateSnapshots = useTelemetryStore((s) => s.updateSnapshots);
  const byId = useTelemetryStore((s) => s.snapshotByDeviceId);
  const connectionStatus = useTelemetryStore((s) => s.connectionStatus);
  const lastUpdatedAt = useTelemetryStore((s) => s.lastUpdatedAt);

  const idsKey = useMemo(() => [...new Set(deviceIds)].sort().join(','), [deviceIds]);
  const stableIds = useMemo(() => (idsKey ? idsKey.split(',') : []), [idsKey]);

  useEffect(() => {
    const unsubSnapshot = engine.onSnapshot(updateSnapshots);
    const unsubStatus = engine.onStatus(setConnectionStatus);
    engine.connect();
    return () => {
      unsubSnapshot();
      unsubStatus();
      engine.disconnect();
    };
  }, [engine, setConnectionStatus, updateSnapshots]);

  useEffect(() => {
    setVisibleDeviceIds(stableIds);
    engine.subscribe(stableIds);

    return () => {
      engine.unsubscribe(stableIds);
    };
  }, [stableIds, engine, setVisibleDeviceIds]);

  const data = useMemo(
    () => stableIds.map((id) => byId[id]).filter(Boolean),
    [stableIds, byId]
  );

  return { data, byId, connectionStatus, lastUpdatedAt };
}
