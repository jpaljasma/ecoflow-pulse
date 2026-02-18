import { env } from '@/shared/config/env';
import {
  IncomingMessageSchema,
  type IncomingMessage,
  type MetricKey
} from '@/features/telemetry/engine/schemas';
import { RingBuffer } from '@/features/telemetry/engine/ringBuffer';
import type {
  DeviceRuntime,
  DeviceSnapshot,
  TelemetryEngineStatus
} from '@/features/telemetry/engine/types';

type EngineOptions = {
  wsUrl?: string;
  snapshotIntervalMs?: number;
  staleAfterMs?: number;
  ringCapacity?: number;
  sparklinePoints?: number;
  heartbeatMs?: number;
};

type SnapshotListener = (payload: {
  snapshots: Record<string, DeviceSnapshot>;
  lastUpdatedAt: number;
  status: TelemetryEngineStatus;
}) => void;

type StatusListener = (status: TelemetryEngineStatus) => void;

const METRIC_KEYS: MetricKey[] = ['soc', 'pvW', 'loadW', 'batteryW', 'tempC'];
const DEFAULT_WS_URL = 'ws://localhost:8080/ws';

export class TelemetryEngine {
  private ws: WebSocket | null = null;
  private readonly wsUrl: string;
  private readonly snapshotIntervalMs: number;
  private readonly staleAfterMs: number;
  private readonly ringCapacity: number;
  private readonly sparklinePoints: number;
  private readonly heartbeatMs: number;
  private readonly wsEnabled: boolean;

  private status: TelemetryEngineStatus = 'idle';
  private token: string | undefined;
  private shouldReconnect = true;

  private reconnectAttempt = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private snapshotTimer: ReturnType<typeof setInterval> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;

  private readonly subscribedDeviceIds = new Set<string>();
  private readonly devices = new Map<string, DeviceRuntime>();

  private readonly snapshotListeners = new Set<SnapshotListener>();
  private readonly statusListeners = new Set<StatusListener>();

  constructor(options: EngineOptions = {}) {
    this.wsUrl = options.wsUrl ?? env.wsUrl;
    this.snapshotIntervalMs = options.snapshotIntervalMs ?? 200;
    this.staleAfterMs = options.staleAfterMs ?? 3_000;
    this.ringCapacity = options.ringCapacity ?? 600;
    this.sparklinePoints = options.sparklinePoints ?? 60;
    this.heartbeatMs = options.heartbeatMs ?? 20_000;
    // In local mock mode, default WS endpoint is typically not available.
    // Disable socket churn to keep UI stable unless WS is explicitly configured.
    this.wsEnabled = !(env.apiUrl.startsWith('mock://') && this.wsUrl === DEFAULT_WS_URL);
  }

  getStatus(): TelemetryEngineStatus {
    return this.status;
  }

  connect(token?: string): void {
    this.token = token;
    this.shouldReconnect = true;

    if (!this.wsEnabled) {
      this.setStatus('connected');
      // In mock mode without WS, keep a stable connected state and avoid
      // periodic snapshot churn that causes unnecessary UI re-renders/flicker.
      return;
    }

    if (this.ws && (this.status === 'connected' || this.status === 'connecting')) {
      return;
    }

    this.openSocket();
    this.startSnapshotClock();
  }

  disconnect(): void {
    this.shouldReconnect = false;
    this.clearReconnectTimer();
    this.stopHeartbeat();
    this.stopSnapshotClock();

    if (this.ws) {
      this.ws.onopen = null;
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.onmessage = null;
      this.ws.close();
      this.ws = null;
    }

    this.setStatus('disconnected');
  }

  subscribe(deviceIds: string[]): void {
    let changed = false;
    for (const id of deviceIds) {
      if (!this.subscribedDeviceIds.has(id)) {
        this.subscribedDeviceIds.add(id);
        changed = true;
      }
    }

    if (changed) {
      this.sendSubscription();
    }
  }

  unsubscribe(deviceIds: string[]): void {
    let changed = false;
    for (const id of deviceIds) {
      if (this.subscribedDeviceIds.delete(id)) {
        changed = true;
      }
    }

    if (changed) {
      this.sendSubscription();
    }
  }

  onSnapshot(listener: SnapshotListener): () => void {
    this.snapshotListeners.add(listener);
    return () => this.snapshotListeners.delete(listener);
  }

  onStatus(listener: StatusListener): () => void {
    this.statusListeners.add(listener);
    return () => this.statusListeners.delete(listener);
  }

  private setStatus(status: TelemetryEngineStatus): void {
    if (this.status === status) {
      return;
    }
    this.status = status;
    this.statusListeners.forEach((listener) => listener(status));
  }

  private openSocket(): void {
    this.clearReconnectTimer();

    this.setStatus(this.reconnectAttempt > 0 ? 'reconnecting' : 'connecting');

    const url = this.token
      ? `${this.wsUrl}${this.wsUrl.includes('?') ? '&' : '?'}token=${encodeURIComponent(this.token)}`
      : this.wsUrl;

    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      this.reconnectAttempt = 0;
      this.setStatus('connected');
      this.sendSubscription();
      this.startHeartbeat();
    };

    this.ws.onmessage = (event) => {
      if (typeof event.data !== 'string') {
        return;
      }

      let decoded: unknown;
      try {
        decoded = JSON.parse(event.data);
      } catch {
        return;
      }

      const parsed = IncomingMessageSchema.safeParse(decoded);
      if (!parsed.success) {
        return;
      }

      this.ingest(parsed.data);
    };

    this.ws.onclose = () => {
      this.ws = null;
      this.stopHeartbeat();
      if (!this.shouldReconnect) {
        this.setStatus('disconnected');
        return;
      }
      this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      this.ws?.close();
    };
  }

  private ingest(message: IncomingMessage): void {
    const runtime = this.getOrInitDeviceRuntime(message.deviceId);
    runtime.lastMessageAt = message.ts;

    if (message.type === 'device_status') {
      const latest = runtime.latest;
      runtime.latest = latest
        ? { ...latest, ts: message.ts, online: message.online }
        : {
            ts: message.ts,
            online: message.online,
            soc: 0,
            pvW: 0,
            loadW: 0,
            batteryW: 0,
            tempC: 0
          };
      return;
    }

    runtime.latest = {
      ts: message.ts,
      online: runtime.latest?.online ?? true,
      ...message.metrics
    };

    for (const metric of METRIC_KEYS) {
      runtime.metrics[metric].push({ ts: message.ts, value: message.metrics[metric] });
    }
  }

  private getOrInitDeviceRuntime(deviceId: string): DeviceRuntime {
    const existing = this.devices.get(deviceId);
    if (existing) {
      return existing;
    }

    const metrics = Object.fromEntries(
      METRIC_KEYS.map((key) => [key, new RingBuffer(this.ringCapacity)])
    ) as DeviceRuntime['metrics'];

    const runtime: DeviceRuntime = {
      latest: null,
      metrics,
      lastMessageAt: 0
    };

    this.devices.set(deviceId, runtime);
    return runtime;
  }

  private buildSnapshot(): Record<string, DeviceSnapshot> {
    const now = Date.now();
    const snapshots: Record<string, DeviceSnapshot> = {};

    for (const deviceId of this.subscribedDeviceIds) {
      const runtime = this.devices.get(deviceId);

      if (!runtime) {
        snapshots[deviceId] = {
          deviceId,
          stale: true,
          online: false,
          lastSeenAt: null,
          metrics: null,
          status: 'stale',
          sparkline: { loadW: [], pvW: [], batteryW: [], soc: [] }
        };
        continue;
      }

      const latest = runtime.latest;
      const stale = !latest || now - runtime.lastMessageAt > this.staleAfterMs;
      const online = latest?.online ?? false;

      const status = stale
        ? 'stale'
        : latest.batteryW > 20
          ? 'charging'
          : latest.batteryW < -20
            ? 'discharging'
            : 'idle';

      snapshots[deviceId] = {
        deviceId,
        stale,
        online,
        lastSeenAt: latest?.ts ?? null,
        metrics: latest,
        status,
        sparkline: {
          loadW: runtime.metrics.loadW.downsample(this.sparklinePoints),
          pvW: runtime.metrics.pvW.downsample(this.sparklinePoints),
          batteryW: runtime.metrics.batteryW.downsample(this.sparklinePoints),
          soc: runtime.metrics.soc.downsample(this.sparklinePoints)
        }
      };
    }

    return snapshots;
  }

  private emitSnapshot(): void {
    const payload = {
      snapshots: this.buildSnapshot(),
      lastUpdatedAt: Date.now(),
      status: this.status
    };

    this.snapshotListeners.forEach((listener) => listener(payload));
  }

  private startSnapshotClock(): void {
    if (this.snapshotTimer) {
      return;
    }

    this.snapshotTimer = setInterval(() => {
      this.emitSnapshot();
    }, this.snapshotIntervalMs);
  }

  private stopSnapshotClock(): void {
    if (!this.snapshotTimer) {
      return;
    }
    clearInterval(this.snapshotTimer);
    this.snapshotTimer = null;
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        return;
      }
      this.ws.send(JSON.stringify({ type: 'ping', ts: Date.now() }));
    }, this.heartbeatMs);
  }

  private stopHeartbeat(): void {
    if (!this.heartbeatTimer) {
      return;
    }
    clearInterval(this.heartbeatTimer);
    this.heartbeatTimer = null;
  }

  private sendSubscription(): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      return;
    }

    this.ws.send(
      JSON.stringify({
        type: 'subscribe',
        deviceIds: Array.from(this.subscribedDeviceIds)
      })
    );
  }

  private scheduleReconnect(): void {
    const base = Math.min(30_000, 1_000 * 2 ** this.reconnectAttempt);
    // Full jitter: spread retries uniformly from 0..base to avoid
    // synchronized reconnect storms and improve fleet stability.
    const delay = Math.floor(Math.random() * (base + 1));

    this.reconnectAttempt += 1;
    this.setStatus('reconnecting');

    this.reconnectTimer = setTimeout(() => {
      this.openSocket();
    }, delay);
  }

  private clearReconnectTimer(): void {
    if (!this.reconnectTimer) {
      return;
    }
    clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
  }
}
