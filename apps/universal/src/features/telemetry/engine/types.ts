import type { MetricKey } from '@/features/telemetry/engine/schemas';
import type { RingBuffer, TimePoint } from '@/features/telemetry/engine/ringBuffer';

export type DeviceLatest = {
  ts: number;
  online: boolean;
  soc: number;
  pvW: number;
  loadW: number;
  batteryW: number;
  tempC: number;
};

export type DeviceRuntime = {
  latest: DeviceLatest | null;
  metrics: Record<MetricKey, RingBuffer>;
  lastMessageAt: number;
};

export type DeviceSnapshot = {
  deviceId: string;
  stale: boolean;
  online: boolean;
  lastSeenAt: number | null;
  metrics: DeviceLatest | null;
  status: 'charging' | 'discharging' | 'idle' | 'stale';
  sparkline: {
    loadW: TimePoint[];
    pvW: TimePoint[];
    batteryW: TimePoint[];
    soc: TimePoint[];
  };
};

export type TelemetryEngineStatus =
  | 'idle'
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'disconnected';
