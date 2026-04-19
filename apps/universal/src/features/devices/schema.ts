import { z } from 'zod';

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

const DeviceDiagnosticEntrySchema = z.object({
  key: z.string(),
  label: z.string(),
  value: z.string(),
  tone: z.enum(['neutral', 'success', 'warning', 'danger', 'info']).optional()
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
  xBoostOn: z.boolean().optional(),
  solarMode: z.string().optional(),
  passthroughMode: z.string().optional(),
  acAutoOnMode: z.string().optional(),
  energyManagementOn: z.boolean().optional(),
  diagnostics: z.array(DeviceDiagnosticEntrySchema).optional(),
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

export const AvailableDeviceSchema = z.object({
  provider: z.string(),
  providerDeviceId: z.string(),
  credentialId: z.string(),
  serialNumber: z.string(),
  name: z.string(),
  model: z.string()
});

export const AvailableDevicesResponseSchema = z.object({
  devices: z.array(AvailableDeviceSchema),
  hasActiveCredentials: z.boolean(),
  warningCode: z.string().optional(),
  warningMessage: z.string().optional()
});

export const DeviceMQTTTestResultSchema = z.object({
  success: z.boolean(),
  status: z.string(),
  sampleTopic: z.string(),
  payloadBytes: z.string(),
  observedAtUnixMs: z.string()
});

export const EnableAvailableDeviceResponseSchema = z.object({
  deviceId: z.string()
});

export const ImportAvailableDevicePayloadSchema = z.object({
  provider: z.string(),
  credentialId: z.string(),
  providerDeviceId: z.string(),
  isActive: z.boolean().optional().default(false),
  ingestDesiredState: z.string().optional()
});

export type DeviceSummary = z.infer<typeof DeviceSchema>;
export type DevicesResponse = z.infer<typeof DevicesResponseSchema>;
export type AvailableDeviceSummary = z.infer<typeof AvailableDeviceSchema>;
export type AvailableDevicesResponse = z.infer<typeof AvailableDevicesResponseSchema>;
export type DeviceMQTTTestResult = z.infer<typeof DeviceMQTTTestResultSchema>;
export type EnableAvailableDeviceResponse = z.infer<typeof EnableAvailableDeviceResponseSchema>;
export type ImportAvailableDevicePayload = z.infer<typeof ImportAvailableDevicePayloadSchema>;
