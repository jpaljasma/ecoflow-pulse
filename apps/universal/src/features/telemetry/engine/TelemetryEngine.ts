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
  FleetTrendSnapshot,
  TelemetryEngineStatus
} from '@/features/telemetry/engine/types';

type EngineOptions = {
  wsUrl?: string;
  snapshotIntervalMs?: number;
  staleAfterMs?: number;
  ringCapacity?: number;
  sparklinePoints?: number;
  heartbeatMs?: number;
  stalledReconnectMs?: number;
  reconnectBaseMs?: number;
  reconnectMaxMs?: number;
  fleetTrendPoints?: number;
  fleetTrendBucketMs?: number;
  createSocket?: (url: string) => WebSocketLike;
};

type ConnectOptions = {
  authRequired?: boolean;
};

type WebSocketLike = {
  readonly readyState: number;
  onopen: ((event: Event) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent) => void) | null;
  send(data: string): void;
  close(): void;
};

type SnapshotListener = (payload: {
  snapshots: Record<string, DeviceSnapshot>;
  fleetTrend: FleetTrendSnapshot;
  lastUpdatedAt: number;
  status: TelemetryEngineStatus;
}) => void;

type StatusListener = (status: TelemetryEngineStatus) => void;

const METRIC_KEYS: MetricKey[] = ['soc', 'pvW', 'loadW', 'batteryW', 'tempC', 'acW', 'dcW'];
const SOCKET_READY_STATE_OPEN = 1;
export class TelemetryEngine {
  private ws: WebSocketLike | null = null;
  private readonly wsUrl: string;
  private readonly wsCandidates: string[];
  private wsCandidateIndex = 0;
  private readonly wsUrlExplicit: boolean;
  private readonly snapshotIntervalMs: number;
  private readonly staleAfterMs: number;
  private readonly ringCapacity: number;
  private readonly sparklinePoints: number;
  private readonly heartbeatMs: number;
  private readonly stalledReconnectMs: number;
  private readonly reconnectBaseMs: number;
  private readonly reconnectMaxMs: number;
  private readonly fleetTrendPoints: number;
  private readonly fleetTrendBucketMs: number;
  private readonly createSocket: (url: string) => WebSocketLike;

  private status: TelemetryEngineStatus = 'idle';
  private token: string | undefined;
  private authRequired = false;
  private shouldReconnect = true;

  private reconnectAttempt = 0;
  private lastInboundAt = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private snapshotTimer: ReturnType<typeof setInterval> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private fleetTrend: FleetTrendSnapshot = {
    load: [],
    pv: [],
    ac: [],
    dc: [],
    filledPoints: 0
  };
  private fleetTrendPending: {
    bucket: number;
    loadSum: number;
    pvSum: number;
    acSum: number;
    dcSum: number;
    count: number;
  } | null = null;

  private readonly subscribedDeviceIds = new Set<string>();
  private readonly subscriptionRefCounts = new Map<string, number>();
  private readonly devices = new Map<string, DeviceRuntime>();

  private readonly snapshotListeners = new Set<SnapshotListener>();
  private readonly statusListeners = new Set<StatusListener>();

  constructor(options: EngineOptions = {}) {
    this.wsUrl = options.wsUrl ?? env.wsUrl;
    this.wsUrlExplicit = env.wsUrlExplicit;
    this.wsCandidates = this.buildWsCandidates(this.wsUrl);
    this.snapshotIntervalMs = options.snapshotIntervalMs ?? 200;
    this.staleAfterMs = options.staleAfterMs ?? 5_000;
    this.ringCapacity = options.ringCapacity ?? 600;
    this.sparklinePoints = options.sparklinePoints ?? 60;
    this.heartbeatMs = options.heartbeatMs ?? 20_000;
    this.stalledReconnectMs = options.stalledReconnectMs ?? 20_000;
    this.reconnectBaseMs = options.reconnectBaseMs ?? 1_000;
    this.reconnectMaxMs = options.reconnectMaxMs ?? 30_000;
    this.fleetTrendPoints = options.fleetTrendPoints ?? 60;
    this.fleetTrendBucketMs = options.fleetTrendBucketMs ?? 5_000;
    this.createSocket = options.createSocket ?? ((url) => new WebSocket(url));
    this.fleetTrend = {
      load: Array.from({ length: this.fleetTrendPoints }, () => 0),
      pv: Array.from({ length: this.fleetTrendPoints }, () => 0),
      ac: Array.from({ length: this.fleetTrendPoints }, () => 0),
      dc: Array.from({ length: this.fleetTrendPoints }, () => 0),
      filledPoints: 0
    };
  }

  getStatus(): TelemetryEngineStatus {
    return this.status;
  }

  connect(token?: string, options: ConnectOptions = {}): void {
    const normalizedToken = token?.trim() ? token.trim() : undefined;
    const nextAuthRequired = options.authRequired ?? false;
    const tokenChanged = this.token !== normalizedToken;
    const authChanged = this.authRequired !== nextAuthRequired;

    this.token = normalizedToken;
    this.authRequired = nextAuthRequired;

    this.startSnapshotClock();

    if (this.authRequired && !this.token) {
      this.shouldReconnect = false;
      this.clearReconnectTimer();
      this.disposeSocket('auth_required');
      return;
    }

    this.shouldReconnect = true;

    if (tokenChanged || authChanged) {
      this.clearReconnectTimer();
      this.disposeSocket();
    }

    if (
      this.ws &&
      (this.status === 'connected' ||
        this.status === 'connecting' ||
        this.status === 'reconnecting')
    ) {
      return;
    }

    this.openSocket();
  }

  subscribe(deviceIds: string[]): void {
    let changed = false;
    for (const id of deviceIds) {
      const nextCount = (this.subscriptionRefCounts.get(id) ?? 0) + 1;
      this.subscriptionRefCounts.set(id, nextCount);
      if (nextCount === 1) {
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
      const prevCount = this.subscriptionRefCounts.get(id) ?? 0;
      if (prevCount <= 1) {
        this.subscriptionRefCounts.delete(id);
        if (this.subscribedDeviceIds.delete(id)) {
          changed = true;
        }
      } else {
        this.subscriptionRefCounts.set(id, prevCount - 1);
      }
    }

    if (this.subscribedDeviceIds.size === 0 && this.subscriptionRefCounts.size > 0) {
      this.subscriptionRefCounts.clear();
    }

    if (changed) {
      this.sendSubscription();
    }
  }

  private clearSubscriptions(): void {
    this.subscribedDeviceIds.clear();
    this.subscriptionRefCounts.clear();
  }

  disconnect(): void {
    this.shouldReconnect = false;
    this.clearReconnectTimer();
    this.stopSnapshotClock();
    this.clearSubscriptions();
    this.disposeSocket('disconnected');
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

    const baseUrl = this.wsCandidates[this.wsCandidateIndex] ?? this.wsUrl;
    const url = this.token
      ? `${baseUrl}${baseUrl.includes('?') ? '&' : '?'}token=${encodeURIComponent(this.token)}`
      : baseUrl;

    console.info('[TelemetryEngine] opening websocket', { url, attempt: this.reconnectAttempt + 1 });

    this.ws = this.createSocket(url);

    this.ws.onopen = () => {
      this.reconnectAttempt = 0;
      this.lastInboundAt = Date.now();
      this.setStatus('connected');
      this.sendSubscription();
      this.startHeartbeat();
    };

    this.ws.onmessage = (event) => {
      this.lastInboundAt = Date.now();
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

    this.ws.onclose = (event) => {
      console.warn('[TelemetryEngine] websocket closed', {
        url: baseUrl,
        code: event.code,
        reason: event.reason
      });
      this.ws = null;
      this.stopHeartbeat();
      if (!this.shouldReconnect) {
        this.setStatus('disconnected');
        return;
      }
      this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      console.warn('[TelemetryEngine] websocket error', { url: baseUrl });
      this.ws?.close();
    };
  }

  private disposeSocket(nextStatus?: TelemetryEngineStatus): void {
    if (this.ws) {
      this.ws.onopen = null;
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.onmessage = null;
      this.ws.close();
      this.ws = null;
    }
    this.stopHeartbeat();
    if (nextStatus) {
      this.setStatus(nextStatus);
    }
  }

  private ingest(message: IncomingMessage): void {
    const runtime = this.getOrInitDeviceRuntime(message.deviceId);
    // Freshness for UI/staleness should be based on receive-time, not source ts,
    // because delayed or replayed streams may emit repeated timestamps.
    runtime.lastMessageAt = Date.now();

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
            tempC: 0,
            acW: 0,
            dcW: 0
          };
      return;
    }

    const normalized = {
      soc: message.metrics.soc,
      pvW: message.metrics.pvW,
      loadW: message.metrics.loadW,
      batteryW: message.metrics.batteryW,
      tempC: message.metrics.tempC,
      acW: message.metrics.acW ?? runtime.latest?.acW ?? 0,
      dcW: message.metrics.dcW ?? runtime.latest?.dcW ?? 0
    };

    runtime.latest = {
      ts: message.ts,
      online: runtime.latest?.online ?? true,
      ...normalized
    };
    if (message.detail) {
      runtime.liveDetail = message.detail;
    }

    for (const metric of METRIC_KEYS) {
      runtime.metrics[metric].push({ ts: message.ts, value: normalized[metric] });
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
      liveDetail: null,
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
          inactive: true,
          online: false,
          lastSeenAt: null,
          metrics: null,
          liveDetail: undefined,
          status: 'stale',
          sparkline: { loadW: [], pvW: [], batteryW: [], soc: [], acW: [], dcW: [] }
        };
        continue;
      }

      const latest = runtime.latest;
      const stale = !latest || now - runtime.lastMessageAt > this.staleAfterMs;
      const inactive = !latest || now - runtime.lastMessageAt > 60_000;
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
        inactive,
        online,
        lastSeenAt: runtime.lastMessageAt || latest?.ts || null,
        metrics: latest,
        liveDetail: runtime.liveDetail ?? undefined,
        status,
        sparkline: {
          loadW: runtime.metrics.loadW.downsample(this.sparklinePoints),
          pvW: runtime.metrics.pvW.downsample(this.sparklinePoints),
          batteryW: runtime.metrics.batteryW.downsample(this.sparklinePoints),
          soc: runtime.metrics.soc.downsample(this.sparklinePoints),
          acW: runtime.metrics.acW.downsample(this.sparklinePoints),
          dcW: runtime.metrics.dcW.downsample(this.sparklinePoints)
        }
      };
    }

    return snapshots;
  }

  private emitSnapshot(): void {
    this.reconnectIfStalled();
    this.updateFleetTrend(Date.now());
    const snapshots = this.buildSnapshot();
    const freshest = Object.values(snapshots).reduce((max, snapshot) => {
      const seen = snapshot.lastSeenAt ?? 0;
      return seen > max ? seen : max;
    }, 0);

    const payload = {
      snapshots,
      fleetTrend: this.fleetTrend,
      lastUpdatedAt: freshest,
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
      if (!this.isSocketOpen()) {
        return;
      }
      const socket = this.ws;
      if (!socket) {
        return;
      }
      socket.send(JSON.stringify({ type: 'ping', ts: Date.now() }));
    }, this.heartbeatMs);
  }

  private stopHeartbeat(): void {
    if (!this.heartbeatTimer) {
      return;
    }
    clearInterval(this.heartbeatTimer);
    this.heartbeatTimer = null;
  }

  private reconnectIfStalled(): void {
    if (!this.isSocketOpen()) return;
    const socket = this.ws;
    if (!socket) return;
    if (this.status !== 'connected') return;
    if (!this.lastInboundAt) return;
    if (Date.now() - this.lastInboundAt <= this.stalledReconnectMs) return;
    socket.close();
  }

  private sendSubscription(): void {
    if (!this.isSocketOpen()) {
      return;
    }
    const socket = this.ws;
    if (!socket) {
      return;
    }

    socket.send(
      JSON.stringify({
        type: 'subscribe',
        deviceIds: Array.from(this.subscribedDeviceIds)
      })
    );
  }

  private isSocketOpen(): boolean {
    return !!this.ws && this.ws.readyState === SOCKET_READY_STATE_OPEN;
  }

  private scheduleReconnect(): void {
    this.reconnectAttempt += 1;
    this.setStatus('reconnecting');
    this.rotateWsCandidate();
    const delay = computeReconnectBackoffWithJitter(
      this.reconnectBaseMs,
      this.reconnectMaxMs,
      this.reconnectAttempt
    );

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

  private buildWsCandidates(primary: string): string[] {
    if (env.isWeb) {
      return [primary];
    }

    if (this.wsUrlExplicit) {
      return [primary];
    }

    const seedUrls = [primary, ...this.deriveWsCandidatesFromApiBase(env.apiUrl)];
    const seen = new Set<string>();
    const candidates: string[] = [];

    for (const seedUrl of seedUrls) {
      let parsed: URL;
      try {
        parsed = new URL(seedUrl);
      } catch {
        if (!seen.has(seedUrl)) {
          seen.add(seedUrl);
          candidates.push(seedUrl);
        }
        continue;
      }

      if (parsed.protocol !== 'ws:' && parsed.protocol !== 'wss:') {
        if (!seen.has(seedUrl)) {
          seen.add(seedUrl);
          candidates.push(seedUrl);
        }
        continue;
      }

      const hostHints = Array.isArray((env as { nativeHostHints?: unknown }).nativeHostHints)
        ? (((env as { nativeHostHints?: unknown }).nativeHostHints as unknown[]) ?? [])
            .filter((value): value is string => typeof value === 'string' && value.length > 0)
        : [];
      const hosts = [parsed.hostname, ...hostHints, '127.0.0.1', 'localhost'];
      for (const hostname of hosts) {
        if (!hostname) continue;
        const host = parsed.port ? `${hostname}:${parsed.port}` : hostname;
        const candidate = `${parsed.protocol}//${host}${parsed.pathname}${parsed.search}`;
        if (seen.has(candidate)) continue;
        seen.add(candidate);
        candidates.push(candidate);
      }
    }

    return candidates.length > 0 ? candidates : [primary];
  }

  private deriveWsCandidatesFromApiBase(apiBase: string): string[] {
    let parsed: URL;
    try {
      parsed = new URL(apiBase);
    } catch {
      return [];
    }

    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      return [];
    }

    const wsProtocol = parsed.protocol === 'https:' ? 'wss:' : 'ws:';
    const normalizedPath =
      !parsed.pathname || parsed.pathname === '/'
        ? ''
        : parsed.pathname.replace(/\/+$/, '');
    const basePath = normalizedPath.endsWith('/api')
      ? normalizedPath.slice(0, normalizedPath.length - '/api'.length)
      : normalizedPath;
    const wsPath = `${basePath}/ws`.replace(/\/{2,}/g, '/');
    const host = parsed.port ? `${parsed.hostname}:${parsed.port}` : parsed.hostname;

    const candidates = [`${wsProtocol}//${host}${wsPath}`];
    if (parsed.port === '18081') {
      candidates.push(`${wsProtocol}//${parsed.hostname}${wsPath}`);
      candidates.push(`${wsProtocol}//${parsed.hostname}:8082/ws`);
    }
    if (!parsed.port) {
      candidates.push(`${wsProtocol}//${parsed.hostname}:18081${wsPath}`);
      candidates.push(`${wsProtocol}//${parsed.hostname}:8082/ws`);
    }

    return candidates;
  }

  private rotateWsCandidate(): void {
    if (this.wsCandidates.length <= 1 || this.wsUrlExplicit) return;
    this.wsCandidateIndex = (this.wsCandidateIndex + 1) % this.wsCandidates.length;
  }

  private aggregateFleetNow(): { load: number; pv: number; ac: number; dc: number } {
    let load = 0;
    let pv = 0;
    let ac = 0;
    let dc = 0;

    for (const deviceId of this.subscribedDeviceIds) {
      const runtime = this.devices.get(deviceId);
      if (!runtime?.latest) continue;
      load += runtime.latest.loadW ?? 0;
      pv += runtime.latest.pvW ?? 0;
      ac += runtime.latest.acW ?? 0;
      dc += runtime.latest.dcW ?? 0;
    }

    return { load, pv, ac, dc };
  }

  private pushFleetTrendPoint(point: { load: number; pv: number; ac: number; dc: number }): void {
    const append = (arr: number[], value: number): number[] => [...arr, value].slice(-this.fleetTrendPoints);
    this.fleetTrend = {
      load: append(this.fleetTrend.load, point.load),
      pv: append(this.fleetTrend.pv, point.pv),
      ac: append(this.fleetTrend.ac, point.ac),
      dc: append(this.fleetTrend.dc, point.dc),
      filledPoints: Math.min(this.fleetTrend.filledPoints + 1, this.fleetTrendPoints)
    };
  }

  private updateFleetTrend(nowMs: number): void {
    const current = this.aggregateFleetNow();
    const currentBucket = Math.floor(nowMs / this.fleetTrendBucketMs);
    const pending = this.fleetTrendPending;

    if (!pending) {
      this.fleetTrendPending = {
        bucket: currentBucket,
        loadSum: current.load,
        pvSum: current.pv,
        acSum: current.ac,
        dcSum: current.dc,
        count: 1
      };
      return;
    }

    if (pending.bucket === currentBucket) {
      pending.loadSum += current.load;
      pending.pvSum += current.pv;
      pending.acSum += current.ac;
      pending.dcSum += current.dc;
      pending.count += 1;
      return;
    }

    const count = Math.max(1, pending.count);
    const averaged = {
      load: pending.loadSum / count,
      pv: pending.pvSum / count,
      ac: pending.acSum / count,
      dc: pending.dcSum / count
    };
    const bucketDelta = Math.max(1, currentBucket - pending.bucket);
    for (let i = 0; i < bucketDelta; i += 1) {
      this.pushFleetTrendPoint(averaged);
    }

    this.fleetTrendPending = {
      bucket: currentBucket,
      loadSum: current.load,
      pvSum: current.pv,
      acSum: current.ac,
      dcSum: current.dc,
      count: 1
    };
  }
}

function computeReconnectBackoffWithJitter(baseMs: number, maxMs: number, attempt: number): number {
  const exponential = Math.min(maxMs, baseMs * 2 ** Math.max(0, attempt - 1));
  const lowerBound = Math.max(250, Math.floor(exponential / 2));
  const upperRange = Math.max(1, exponential - lowerBound);
  return lowerBound + Math.floor(Math.random() * upperRange);
}
