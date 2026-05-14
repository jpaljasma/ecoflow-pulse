import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';

type ResolveDeviceSocInput = {
  device?: Pick<DeviceSummary, 'batteryPct' | 'details'>;
  details?: DeviceSummary['details'];
  snapshot?: DeviceSnapshot;
};

function clampPct(value: number | null | undefined): number | undefined {
  if (typeof value !== 'number' || !Number.isFinite(value)) return undefined;
  return Math.max(0, Math.min(100, value));
}

export function resolveDeviceSocPct({
  device,
  details = device?.details,
  snapshot
}: ResolveDeviceSocInput): number | undefined {
  const overallSoc = clampPct(details?.overallSocPct);
  if (overallSoc !== undefined) {
    return overallSoc;
  }

  const packSocs = (details?.packs ?? [])
    .map((pack) => clampPct(pack.socPct))
    .filter((value): value is number => value !== undefined);
  if (packSocs.length > 0) {
    return packSocs.reduce((total, value) => total + value, 0) / packSocs.length;
  }

  const liveSoc = clampPct(snapshot?.metrics?.soc);
  if (liveSoc !== undefined) {
    return liveSoc;
  }

  return clampPct(device?.batteryPct);
}
