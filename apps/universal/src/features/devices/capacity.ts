import type { DeviceSummary } from '@/features/devices/api';

export function getCapacityKWh(device: Pick<DeviceSummary, 'model' | 'capabilities'>): number | null {
  const explicit = (device.capabilities as Record<string, unknown> | undefined)?.batteryCapacityKWh;
  if (typeof explicit === 'number' && Number.isFinite(explicit) && explicit > 0) {
    return explicit;
  }

  const model = device.model.toLowerCase();
  if (model.includes('delta pro ultra')) return 12.1;
  if (model.includes('delta 2 max')) return 4.1;
  if (model.includes('delta pro')) return 3.6;
  return null;
}
