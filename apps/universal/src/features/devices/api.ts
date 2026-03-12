import { z } from 'zod';
import { requestJson } from '@/shared/api/restClient';

const BatteryPackDetailSchema = z.object({
  id: z.string(),
  socPct: z.number().optional(),
  powerW: z.number().optional(),
  tempC: z.number().optional(),
  heatingOn: z.boolean().optional(),
  energyWh: z.number().optional(),
  remainMinutes: z.number().optional(),
  socMinPct: z.number().optional(),
  socMaxPct: z.number().optional()
});

const SolarPortDetailSchema = z.object({
  id: z.string(),
  name: z.string(),
  state: z.string().optional(),
  volts: z.number().optional(),
  amps: z.number().optional(),
  watts: z.number().optional(),
  maxVolts: z.number().optional(),
  maxAmps: z.number().optional(),
  maxWatts: z.number().optional()
});

const DeviceTelemetryDetailsSchema = z.object({
  bpCount: z.number().int().optional(),
  packs: z.array(BatteryPackDetailSchema).optional(),
  solarPorts: z.array(SolarPortDetailSchema).optional(),
  overallSocPct: z.number().optional(),
  socWindowMinPct: z.number().optional(),
  socWindowMaxPct: z.number().optional(),
  backupReservePct: z.number().optional(),
  estimateMode: z.string().optional(),
  estimateSource: z.string().optional(),
  estimateEtaMin: z.number().optional(),
  remainChargeMin: z.number().optional(),
  remainDischargeMin: z.number().optional(),
  remainGlobalMin: z.number().optional(),
  mpptLowState: z.string().optional(),
  mpptHighState: z.string().optional(),
  acOn: z.boolean().optional(),
  dcOn: z.boolean().optional(),
  usbOn: z.boolean().optional(),
  dc12vOn: z.boolean().optional(),
  evChargingOn: z.boolean().optional(),
  fanOn: z.boolean().optional(),
  solarChargingOn: z.boolean().optional(),
  batteryHeatingOn: z.boolean().optional(),
  stormGuardActive: z.boolean().optional(),
  stormGuardEndsAtUnixMs: z.number().optional(),
  timezoneId: z.string().optional(),
  timezoneOffsetMinutes: z.number().int().optional(),
  timezoneMode: z.enum(['manual', 'auto']).optional(),
  mqttQueueDepth: z.number().int().optional(),
  mqttQueueDroppedOldest: z.number().int().optional()
});

export const DeviceSchema = z.object({
  id: z.string(),
  serialNumber: z.string(),
  name: z.string(),
  model: z.string(),
  online: z.boolean(),
  batteryPct: z.number(),
  state: z.enum(['charging', 'discharging', 'idle']),
  etaMinutes: z.number().int().nonnegative(),
  pvW: z.number().optional(),
  acInW: z.number().optional(),
  dcW: z.number().optional(),
  loadW: z.number().optional(),
  netW: z.number().optional(),
  tempC: z.number().optional(),
  telemetryTsMs: z.number().optional(),
  capabilities: z.record(z.unknown()).optional(),
  details: DeviceTelemetryDetailsSchema.optional()
});

export const DevicesResponseSchema = z.object({
  devices: z.array(DeviceSchema)
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
