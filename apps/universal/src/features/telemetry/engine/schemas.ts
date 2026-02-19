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

export const TelemetryMessageSchema = z.object({
  type: z.literal('telemetry'),
  deviceId: z.string(),
  ts: z.number(),
  metrics: TelemetryMetricsSchema
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
