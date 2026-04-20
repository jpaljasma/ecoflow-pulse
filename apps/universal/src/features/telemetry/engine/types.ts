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
  acW: number;
  dcW: number;
};

export type DeviceLiveSignals = {
  acOn?: boolean;
  dcOn?: boolean;
  usbOn?: boolean;
  dc12vOn?: boolean;
  evChargingOn?: boolean;
  fanOn?: boolean;
  solarChargingOn?: boolean;
  batteryHeatingOn?: boolean;
};

export type DeviceLiveSolarPort = {
  id: string;
  name: string;
  state?: string;
  volts?: number;
  amps?: number;
  watts?: number;
};

export type DeviceLiveDetail = {
  signals?: DeviceLiveSignals;
  solarPorts?: DeviceLiveSolarPort[];
};

export type DeviceRuntime = {
  latest: DeviceLatest | null;
  online: boolean;
  liveDetail: DeviceLiveDetail | null;
  metrics: Record<MetricKey, RingBuffer>;
  lastMessageAt: number;
};

export type DeviceSnapshot = {
  deviceId: string;
  stale: boolean;
  inactive: boolean;
  online: boolean;
  lastSeenAt: number | null;
  metrics: DeviceLatest | null;
  liveDetail?: DeviceLiveDetail;
  status: 'charging' | 'discharging' | 'idle' | 'stale';
  sparkline: {
    loadW: TimePoint[];
    pvW: TimePoint[];
    batteryW: TimePoint[];
    soc: TimePoint[];
    acW: TimePoint[];
    dcW: TimePoint[];
  };
};

export type FleetTrendSnapshot = {
  load: number[];
  pv: number[];
  ac: number[];
  dc: number[];
  filledPoints: number;
};

export type TelemetryEngineStatus =
  | 'idle'
  | 'auth_required'
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'disconnected';
