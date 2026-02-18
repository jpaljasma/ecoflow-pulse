import { z } from 'zod';
import { requestJson } from '@/shared/api/restClient';

export const DeviceSchema = z.object({
  id: z.string(),
  serialNumber: z.string(),
  name: z.string(),
  model: z.string(),
  online: z.boolean(),
  batteryPct: z.number(),
  state: z.enum(['charging', 'discharging', 'idle']),
  etaMinutes: z.number().int().nonnegative(),
  capabilities: z.record(z.unknown()).optional()
});

export const DevicesResponseSchema = z.object({
  devices: z.array(DeviceSchema.omit({ capabilities: true }))
});

export type DeviceSummary = z.infer<typeof DeviceSchema>;
export type DevicesResponse = z.infer<typeof DevicesResponseSchema>;

export async function fetchDevices(token?: string): Promise<DevicesResponse> {
  const data = await requestJson<unknown>('/api/devices', { token });
  return DevicesResponseSchema.parse(data);
}

export async function fetchDevice(
  deviceId: string,
  token?: string
): Promise<DeviceSummary> {
  const data = await requestJson<unknown>(`/api/devices/${deviceId}`, { token });
  return DeviceSchema.parse(data);
}
