import { z } from 'zod';

const deviceIdsSchema = z.array(z.string().trim().min(1)).min(1).max(128).transform((ids) => {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const id of ids) {
    if (!seen.has(id)) {
      seen.add(id);
      out.push(id);
    }
  }
  return out;
});

export const SubscribeMessageSchema = z.object({
  type: z.literal('subscribe'),
  deviceIds: deviceIdsSchema
});

export const UnsubscribeMessageSchema = z.object({
  type: z.literal('unsubscribe'),
  deviceIds: deviceIdsSchema
});

export const PingMessageSchema = z.object({
  type: z.literal('ping'),
  ts: z.number().optional()
});

export const ClientMessageSchema = z.union([
  SubscribeMessageSchema,
  UnsubscribeMessageSchema,
  PingMessageSchema
]);

export type ClientMessage = z.infer<typeof ClientMessageSchema>;

export type ServerTelemetryMessage = {
  type: 'telemetry';
  deviceId: string;
  ts: number;
  metrics: {
    soc: number;
    pvW: number;
    loadW: number;
    batteryW: number;
    tempC: number;
    acW?: number;
    dcW?: number;
  };
  detail?: {
    signals?: {
      acOn?: boolean;
      dcOn?: boolean;
      usbOn?: boolean;
      dc12vOn?: boolean;
      evChargingOn?: boolean;
      fanOn?: boolean;
      solarChargingOn?: boolean;
      batteryHeatingOn?: boolean;
    };
    solarPorts?: Array<{
      id: string;
      name: string;
      state?: string;
      volts?: number;
      amps?: number;
      watts?: number;
    }>;
  };
};

export type ServerDeviceStatusMessage = {
  type: 'device_status';
  deviceId: string;
  ts: number;
  online: boolean;
};

export type ServerPongMessage = {
  type: 'pong';
  ts: number;
};
