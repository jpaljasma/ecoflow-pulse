import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';
import { resolveDeviceSocPct } from '@/features/devices/soc';

export type FleetDevicePreviewChargeState = 'charging' | 'discharging' | 'neutral';

export type FleetDevicePreviewItem = {
  id: string;
  name: string;
  model: string;
  batteryPct: number | null;
  chargeState: FleetDevicePreviewChargeState;
  device: DeviceSummary;
};

function resolveChargeState(
  device: DeviceSummary,
  snapshot?: DeviceSnapshot
): FleetDevicePreviewChargeState {
  const status =
    snapshot && !snapshot.stale && !snapshot.inactive
      ? snapshot.status
      : device.state;

  if (status === 'charging' || status === 'discharging') {
    return status;
  }
  return 'neutral';
}

export function resolveFleetDevicePreviewLimit(contentWidth: number): number {
  return contentWidth >= 640 ? 4 : 2;
}

export function buildFleetDevicePreview(
  devices: DeviceSummary[],
  {
    maxItems,
    snapshotsById = {}
  }: {
    maxItems: number;
    snapshotsById?: Record<string, DeviceSnapshot>;
  }
): FleetDevicePreviewItem[] {
  const ranked = devices
    .map((device, index) => {
      const snapshot = snapshotsById[device.id];
      const chargeState = resolveChargeState(device, snapshot);
      return {
        id: device.id,
        name: device.name,
        model: device.model,
        batteryPct: resolveDeviceSocPct({ device, snapshot }) ?? null,
        chargeState,
        device,
        originalIndex: index
      };
    })
    .sort((a, b) => {
      const aActive = a.chargeState === 'neutral' ? 1 : 0;
      const bActive = b.chargeState === 'neutral' ? 1 : 0;
      return aActive - bActive || a.originalIndex - b.originalIndex;
    });

  return ranked
    .slice(0, maxItems)
    .map((item) => ({
      id: item.id,
      name: item.name,
      model: item.model,
      batteryPct: item.batteryPct,
      chargeState: item.chargeState,
      device: item.device
    }));
}
