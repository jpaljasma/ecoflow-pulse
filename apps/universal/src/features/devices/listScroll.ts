import type { DeviceSummary } from '@/features/devices/schema';

export function findDeviceScrollIndex(devices: DeviceSummary[], deviceId: string | undefined): number {
  if (devices.length === 0) {
    return -1;
  }
  const target = deviceId?.trim();
  if (!target) {
    return 0;
  }
  const index = devices.findIndex((device) => device.id === target);
  return index >= 0 ? index : 0;
}
