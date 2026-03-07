import { z } from 'zod';

export const TelemetryMetricsSchema = z.object({
  soc: z.number(),
  pvW: z.number(),
  loadW: z.number(),
  batteryW: z.number(),
  tempC: z.number(),
  acW: z.number().optional(),
  dcW: z.number().optional()
});

export const TelemetrySignalSchema = z.object({
  acOn: z.boolean().optional(),
  dcOn: z.boolean().optional(),
  usbOn: z.boolean().optional(),
  dc12vOn: z.boolean().optional(),
  evChargingOn: z.boolean().optional(),
  fanOn: z.boolean().optional(),
  solarChargingOn: z.boolean().optional(),
  batteryHeatingOn: z.boolean().optional()
});

export const TelemetrySolarPortSchema = z.object({
  id: z.string(),
  name: z.string(),
  state: z.string().optional(),
  volts: z.number().optional(),
  amps: z.number().optional(),
  watts: z.number().optional()
});

export const TelemetryDetailSchema = z.object({
  signals: TelemetrySignalSchema.optional(),
  solarPorts: z.array(TelemetrySolarPortSchema).optional()
});

export const TelemetryMessageSchema = z.object({
  type: z.literal('telemetry'),
  deviceId: z.string(),
  ts: z.number(),
  metrics: TelemetryMetricsSchema,
  detail: TelemetryDetailSchema.optional()
});

export const DeviceStatusMessageSchema = z.object({
  type: z.literal('device_status'),
  deviceId: z.string(),
  ts: z.number(),
  online: z.boolean()
});

export const IncomingMessageSchema = z.union([
  TelemetryMessageSchema,
  DeviceStatusMessageSchema
]);

export type TelemetryMessage = z.infer<typeof TelemetryMessageSchema>;
export type DeviceStatusMessage = z.infer<typeof DeviceStatusMessageSchema>;
export type IncomingMessage = z.infer<typeof IncomingMessageSchema>;

export type MetricKey = keyof z.infer<typeof TelemetryMetricsSchema>;
