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

const optionalStringListSchema = z
  .array(z.string().trim().min(1))
  .max(128)
  .optional()
  .default([])
  .transform((values) => {
    const seen = new Set<string>();
    const out: string[] = [];
    for (const value of values) {
      if (!seen.has(value)) {
        seen.add(value);
        out.push(value);
      }
    }
    return out;
  });

const logStatusSchema = z.enum(['ok', 'warning', 'error']);
const emptyLogFilters = {
  deviceIds: [],
  statuses: [],
  providers: [],
  sources: [],
  typeCodes: [],
  typeCodeSuffixes: []
};

export const LogsSubscribeMessageSchema = z.object({
  type: z.literal('logs_subscribe'),
  subscriptionId: z.string().trim().min(1).max(80).optional().default('default'),
  filters: z
    .object({
      deviceIds: optionalStringListSchema,
      statuses: z.array(logStatusSchema).max(8).optional().default([]),
      providers: optionalStringListSchema,
      sources: optionalStringListSchema,
      typeCodes: optionalStringListSchema,
      typeCodeSuffixes: optionalStringListSchema
    })
    .optional()
    .default(emptyLogFilters)
    .transform((filters) => ({
      deviceIds: filters.deviceIds,
      statuses: filters.statuses,
      providers: filters.providers,
      sources: filters.sources,
      typeCodes: filters.typeCodes,
      typeCodeSuffixes: filters.typeCodeSuffixes
    })),
  replayLimit: z.number().int().min(1).max(200).optional().default(200),
  replaySinceUnixMs: z.number().int().min(0).optional().default(0)
});

export const LogsUnsubscribeMessageSchema = z.object({
  type: z.literal('logs_unsubscribe'),
  subscriptionId: z.string().trim().min(1).max(80).optional().default('default')
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
  LogsSubscribeMessageSchema,
  LogsUnsubscribeMessageSchema,
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

export type ServerLogEntryMessage = {
  type: 'log_entry';
  subscriptionId: string;
  entry: {
    id: string;
    ts: number;
    receivedTs: number;
    deviceId: string;
    status: 'ok' | 'warning' | 'error';
    source: string;
    sourceKind: string;
    typeCode: string;
    summary: string;
    labels: Record<string, string>;
    detail: Record<string, unknown>;
  };
};

export type ServerLogsStatusMessage = {
  type: 'logs_status';
  subscriptionId: string;
  ts: number;
  state: 'replay' | 'live' | 'forbidden' | 'error' | 'closed';
  message?: string;
};

export type ServerLogsReplayDoneMessage = {
  type: 'logs_replay_done';
  subscriptionId: string;
  ts: number;
  replayed: number;
};
