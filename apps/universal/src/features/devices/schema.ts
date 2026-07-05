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
  capabilities: z.record(z.string(), z.unknown()).optional(),
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
  model: z.string(),
  capabilities: z.record(z.string(), z.unknown()).optional(),
  metadata: z.record(z.string(), z.unknown()).optional()
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
  observedAtUnixMs: z.string(),
  deviceId: z.string().optional()
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

export const EdgeCollectorSchema = z.object({
  id: z.string(),
  displayName: z.string(),
  isActive: z.boolean(),
  lastHeartbeatAtUnixMs: z.string(),
  createdAtUnixMs: z.string(),
  updatedAtUnixMs: z.string(),
  collectorVersion: z.string(),
  hostname: z.string()
});

export const EdgeCollectorsResponseSchema = z.object({
  collectors: z.array(EdgeCollectorSchema)
});

export const CreateEdgeCollectorPayloadSchema = z.object({
  displayName: z.string().trim().min(1).max(128).optional()
});

export const CreateEdgeCollectorResponseSchema = z.object({
  collector: EdgeCollectorSchema,
  setupToken: z.string()
});

export const RevokeEdgeCollectorSetupTokenResponseSchema = z.object({
  collector: EdgeCollectorSchema
});

export const EDGE_DEVICE_SOURCE_STATUS_PENDING = 'pending';
export const EDGE_DEVICE_SOURCE_STATUS_LINKED = 'linked';
export const EDGE_DEVICE_SOURCE_STATUS_IGNORED = 'ignored';
export const EDGE_DEVICE_SOURCE_STATUS_UNKNOWN = 'unknown';
export const EDGE_DEVICE_SOURCE_FILTER_STATUSES = [
  EDGE_DEVICE_SOURCE_STATUS_PENDING,
  EDGE_DEVICE_SOURCE_STATUS_LINKED,
  EDGE_DEVICE_SOURCE_STATUS_IGNORED
] as const;
export const EDGE_DEVICE_SOURCE_STATUSES = [
  ...EDGE_DEVICE_SOURCE_FILTER_STATUSES,
  EDGE_DEVICE_SOURCE_STATUS_UNKNOWN
] as const;

export const EdgeDeviceSourceFilterStatusSchema = z.enum(
  EDGE_DEVICE_SOURCE_FILTER_STATUSES
);
export const EdgeDeviceSourceStatusSchema = z.enum(EDGE_DEVICE_SOURCE_STATUSES);

export const EdgeDeviceSourceSchema = z.object({
  id: z.string(),
  collectorId: z.string(),
  provider: z.string(),
  transport: z.string(),
  displayName: z.string(),
  model: z.string(),
  status: EdgeDeviceSourceStatusSchema,
  rawStatus: z.string().optional(),
  linkedDeviceId: z.string(),
  rssiDbm: z.number(),
  lastSeenAtUnixMs: z.string(),
  createdAtUnixMs: z.string(),
  updatedAtUnixMs: z.string()
});

export const EdgeDeviceSourcesResponseSchema = z.object({
  sources: z.array(EdgeDeviceSourceSchema)
});

export const ApproveEdgeDeviceSourcePayloadSchema = z.object({
  sourceId: z.string().trim().min(1),
  deviceId: z.string().trim().optional(),
  productName: z.string().trim().max(128).optional(),
  model: z.string().trim().max(128).optional()
});

export const ApproveEdgeDeviceSourceResponseSchema = z.object({
  source: EdgeDeviceSourceSchema,
  deviceId: z.string()
});

export type DeviceSummary = z.infer<typeof DeviceSchema>;
export type DevicesResponse = z.infer<typeof DevicesResponseSchema>;
export type AvailableDeviceSummary = z.infer<typeof AvailableDeviceSchema>;
export type AvailableDevicesResponse = z.infer<
  typeof AvailableDevicesResponseSchema
>;
export type DeviceMQTTTestResult = z.infer<typeof DeviceMQTTTestResultSchema>;
export type EnableAvailableDeviceResponse = z.infer<
  typeof EnableAvailableDeviceResponseSchema
>;
export type ImportAvailableDevicePayload = z.infer<
  typeof ImportAvailableDevicePayloadSchema
>;
export type EdgeCollector = z.infer<typeof EdgeCollectorSchema>;
export type EdgeDeviceSourceFilterStatus = z.infer<
  typeof EdgeDeviceSourceFilterStatusSchema
>;
export type EdgeDeviceSourceStatus = z.infer<
  typeof EdgeDeviceSourceStatusSchema
>;
export type EdgeDeviceSource = z.infer<typeof EdgeDeviceSourceSchema>;
export type CreateEdgeCollectorPayload = z.infer<
  typeof CreateEdgeCollectorPayloadSchema
>;
export type CreateEdgeCollectorResponse = z.infer<
  typeof CreateEdgeCollectorResponseSchema
>;
export type RevokeEdgeCollectorSetupTokenResponse = z.infer<
  typeof RevokeEdgeCollectorSetupTokenResponseSchema
>;
export type ApproveEdgeDeviceSourcePayload = z.infer<
  typeof ApproveEdgeDeviceSourcePayloadSchema
>;
export type ApproveEdgeDeviceSourceResponse = z.infer<
  typeof ApproveEdgeDeviceSourceResponseSchema
>;
