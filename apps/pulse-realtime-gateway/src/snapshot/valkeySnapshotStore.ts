import { createClient, createSentinel } from 'redis';

import type { LiveSnapshot } from '../live/types.js';

const DEFAULT_METRIC_FRESHNESS_MS = 2 * 60_000;
const DEFAULT_METRIC_FLATLINE_MS = 30 * 60_000;

type RedisLike = {
  get(key: string): Promise<string | null>;
  close(): Promise<void>;
  on?(event: 'error' | 'end', listener: (error?: Error) => void): unknown;
};

export interface SnapshotStore {
  getSnapshot(deviceId: string): Promise<LiveSnapshot | null>;
  close(): Promise<void>;
}

export type ValkeySnapshotStoreConfig = {
  addrs: string[];
  sentinelMasterSet?: string;
  sentinelUsername?: string;
  sentinelPassword?: string;
  username?: string;
  password?: string;
  keyPrefix: string;
};

export class ValkeySnapshotStore implements SnapshotStore {
  private readonly cfg: ValkeySnapshotStoreConfig;
  private clientPromises: Promise<RedisLike>[] | null = null;
  private clientGeneration = 0;

  constructor(cfg: ValkeySnapshotStoreConfig) {
    this.cfg = cfg;
  }

  async getSnapshot(deviceId: string): Promise<LiveSnapshot | null> {
    const key = `${this.cfg.keyPrefix}:{did:${sanitizeKeySegment(deviceId)}}:snapshot`;
    for (const client of await this.getClients()) {
      const raw = await client.get(key).catch(() => null);
      if (!raw) {
        continue;
      }
      let parsed: unknown;
      try {
        parsed = JSON.parse(raw);
      } catch {
        return null;
      }
      const value = parsed as {
        device_id?: string;
        cursor_seq?: number;
        cursor_ts_unix_ms?: number;
        metrics?: Record<string, number>;
        metric_observed_at_unix_ms?: Record<string, number>;
        metric_changed_at_unix_ms?: Record<string, number>;
      };
      const cursorTsUnixMs = typeof value.cursor_ts_unix_ms === 'number' ? value.cursor_ts_unix_ms : 0;
      return {
        deviceId: typeof value.device_id === 'string' ? value.device_id : deviceId,
        cursor: {
          seq: typeof value.cursor_seq === 'number' ? value.cursor_seq : 0,
          tsUnixMs: cursorTsUnixMs
        },
        metrics: normalizeFreshMetrics(
          value.metrics,
          value.metric_observed_at_unix_ms,
          value.metric_changed_at_unix_ms,
          cursorTsUnixMs
        )
      };
    }
    return null;
  }

  async close(): Promise<void> {
    const clientPromises = this.clientPromises;
    if (!clientPromises) {
      return;
    }
    this.clientPromises = null;
    this.clientGeneration += 1;
    const clients = await Promise.all(clientPromises.map((client) => client.catch(() => null)));
    for (const client of clients) {
      if (client) {
        await client.close().catch(() => undefined);
      }
    }
  }

  private getClients(): Promise<RedisLike[]> {
    if (!this.clientPromises) {
      this.clientGeneration += 1;
      this.clientPromises = this.createClients(this.clientGeneration);
    }
    const generation = this.clientGeneration;
    return Promise.all(this.clientPromises).catch(() => {
      this.resetClients(generation);
      return [];
    });
  }

  private createClients(generation: number): Promise<RedisLike>[] {
    const addrs = this.cfg.addrs.filter((value) => value.trim() !== '');
    const hosts = addrs.length > 0 ? addrs : [this.cfg.sentinelMasterSet ? '127.0.0.1:26379' : '127.0.0.1:6379'];
    if (this.cfg.sentinelMasterSet) {
      return [this.createSentinelClient(hosts, generation)];
    }
    return hosts.map(async (host) => {
      const client = createClient({
        url: toRedisUrl(host),
        username: this.cfg.username,
        password: this.cfg.password,
        socket: sharedSocketOptions()
      });
      this.attachClientHandlers(client, generation);
      await client.connect();
      return client;
    });
  }

  private async createSentinelClient(hosts: string[], generation: number): Promise<RedisLike> {
    const client = createSentinel({
      name: this.cfg.sentinelMasterSet!,
      sentinelRootNodes: hosts.map((host) => toRedisNode(host)),
      nodeClientOptions: {
        username: this.cfg.username,
        password: this.cfg.password,
        socket: sharedSocketOptions()
      },
      sentinelClientOptions: {
        username: this.cfg.sentinelUsername,
        password: this.cfg.sentinelPassword,
        socket: sharedSocketOptions()
      }
    });
    this.attachClientHandlers(client, generation);
    await client.connect();
    return client;
  }

  private attachClientHandlers(client: RedisLike, generation: number): void {
    client.on?.('error', () => {
      this.resetClients(generation);
    });
    client.on?.('end', () => {
      this.resetClients(generation);
    });
  }

  private resetClients(generation: number): void {
    if (generation !== this.clientGeneration) {
      return;
    }
    const clientPromises = this.clientPromises;
    if (!clientPromises) {
      return;
    }
    this.clientPromises = null;
    void Promise.all(clientPromises.map((client) => client.catch(() => null))).then(async (clients) => {
      await Promise.all(clients.map((client) => client?.close().catch(() => undefined)));
    });
  }
}

export function preferredLocalValkeyAddrs(input: string[]): string[] {
  const addrs = input.map((value) => value.trim()).filter(Boolean);
  if (addrs.length !== 1 || addrs[0] !== '127.0.0.1:6379') {
    return addrs;
  }
  return ['127.0.0.1:6380', '127.0.0.1:6379'];
}

function sanitizeKeySegment(input: string): string {
  return input.trim().replaceAll('{', '_').replaceAll('}', '_').replaceAll(' ', '_');
}

function normalizeFreshMetrics(
  metrics: Record<string, number> | undefined,
  observedAt: Record<string, number> | undefined,
  changedAt: Record<string, number> | undefined,
  cursorTsUnixMs: number
): Record<string, number> {
  if (!metrics || typeof metrics !== 'object') {
    return {};
  }
  const now = Date.now();
  const out: Record<string, number> = {};
  const freshness = buildMetricFreshnessContext(metrics, observedAt, changedAt, cursorTsUnixMs);
  for (const [key, value] of Object.entries(metrics)) {
    if (typeof value !== 'number' || !Number.isFinite(value)) {
      continue;
    }
    const metricObservedAt =
      observedAt && typeof observedAt[key] === 'number' && Number.isFinite(observedAt[key])
        ? observedAt[key]
        : cursorTsUnixMs;
    const metricChangedAt =
      changedAt && typeof changedAt[key] === 'number' && Number.isFinite(changedAt[key])
        ? changedAt[key]
        : metricObservedAt;
    if (metricExpired(key, metricObservedAt, metricChangedAt, now, freshness)) {
      continue;
    }
    out[key] = value;
  }
  return out;
}

function buildMetricFreshnessContext(
  metrics: Record<string, number>,
  observedAt: Record<string, number> | undefined,
  changedAt: Record<string, number> | undefined,
  cursorTsUnixMs: number
): { currentTelemetryFlatlined: boolean; currentTelemetryIdleStale: boolean } {
  let newestObservedAt = 0;
  let newestChangedAt = 0;
  for (const key of Object.keys(metrics)) {
    if (!isCurrentTelemetryMetricKey(key)) {
      continue;
    }
    const metricObservedAt =
      observedAt && typeof observedAt[key] === 'number' && Number.isFinite(observedAt[key])
        ? observedAt[key]
        : cursorTsUnixMs;
    const metricChangedAt =
      changedAt && typeof changedAt[key] === 'number' && Number.isFinite(changedAt[key])
        ? changedAt[key]
        : metricObservedAt;
    if (metricObservedAt > newestObservedAt) {
      newestObservedAt = metricObservedAt;
    }
    if (metricChangedAt > newestChangedAt) {
      newestChangedAt = metricChangedAt;
    }
  }
  return {
    currentTelemetryFlatlined:
      newestObservedAt > 0 &&
      newestChangedAt > 0 &&
      newestObservedAt - newestChangedAt > DEFAULT_METRIC_FLATLINE_MS,
    currentTelemetryIdleStale: currentTelemetryIdleStale(metrics)
  };
}

function metricExpired(
  key: string,
  observedAtUnixMs: number,
  changedAtUnixMs: number,
  nowUnixMs: number,
  freshness: { currentTelemetryFlatlined: boolean; currentTelemetryIdleStale: boolean }
): boolean {
  if (!isVolatileMetricKey(key) || !(observedAtUnixMs > 0) || !(nowUnixMs > 0)) {
    return false;
  }
  if (nowUnixMs - observedAtUnixMs > DEFAULT_METRIC_FRESHNESS_MS) {
    return true;
  }
  return isCurrentTelemetryMetricKey(key) && (
    (freshness.currentTelemetryFlatlined && changedAtUnixMs > 0) ||
    freshness.currentTelemetryIdleStale
  );
}

function currentTelemetryIdleStale(metrics: Record<string, number>): boolean {
  const input = currentInputWatts(metrics);
  if (!input.found || input.value <= 5) {
    return false;
  }
  const load = firstMetric(metrics, ['loadW', 'params.wattsOutSum', 'param.wattsOutSum', 'params.invOutWatts']);
  const batterySink = currentBatterySinkWatts(metrics);
  if ((load.found && Math.abs(load.value) > 1) || (batterySink.found && batterySink.value > 1)) {
    return false;
  }
  if (!load.found && !batterySink.found) {
    return false;
  }
  return hasIdleOrPausedSignal(metrics) || hasSentinelRemainTime(metrics);
}

function currentInputWatts(metrics: Record<string, number>): { value: number; found: boolean } {
  const aggregate = firstMetric(metrics, ['pvW', 'params.wattsInSum', 'param.wattsInSum', 'params.inWatts']);
  if (aggregate.found) {
    return aggregate;
  }
  return sumIfPresent(metrics, [
    'params.inLvMpptPwr',
    'params.inHvMpptPwr',
    'params.pv1ChargeWatts',
    'params.pv2ChargeWatts',
    'params.pv1InWatts',
    'params.pv2InWatts'
  ]);
}

function currentBatterySinkWatts(metrics: Record<string, number>): { value: number; found: boolean } {
  const input = firstMetric(metrics, ['params.bmsInputWatts', 'params.inputWatts']);
  if (input.found) {
    const output = firstMetric(metrics, ['params.bmsOutputWatts', 'params.outputWatts']);
    return { value: Math.abs(input.value) + Math.abs(output.found ? output.value : 0), found: true };
  }
  const battery = firstMetric(metrics, ['batteryW']);
  if (battery.found) {
    return { value: Math.abs(battery.value), found: true };
  }
  const volts = firstMetric(metrics, ['params.batVol']);
  const amps = firstMetric(metrics, ['params.batAmp']);
  if (!volts.found || !amps.found) {
    return { value: 0, found: false };
  }
  return {
    value: Math.abs(normalizePotentialMilliUnit(volts.value, 1000) * normalizePotentialMilliUnit(amps.value, 200)),
    found: true
  };
}

function hasIdleOrPausedSignal(metrics: Record<string, number>): boolean {
  const pause = firstMetric(metrics, ['params.chgPauseFlag']);
  if (pause.found && pause.value >= 1) {
    return true;
  }
  const chargeDischargeState = firstMetric(metrics, ['params.chgDsgState']);
  if (chargeDischargeState.found && chargeDischargeState.value === 2) {
    return true;
  }
  const systemState = firstMetric(metrics, ['params.sysState']);
  return systemState.found && systemState.value === 2;
}

function hasSentinelRemainTime(metrics: Record<string, number>): boolean {
  const remain = firstMetric(metrics, ['params.remainTime', 'param.remainTime']);
  const charge = firstMetric(metrics, ['params.chgRemainTime', 'param.chgRemainTime']);
  const discharge = firstMetric(metrics, ['params.dsgRemainTime', 'param.dsgRemainTime']);
  if (!remain.found && !charge.found && !discharge.found) {
    return false;
  }
  return (!remain.found || remain.value >= 5990) &&
    (!charge.found || charge.value >= 5990) &&
    (!discharge.found || discharge.value >= 5990);
}

function firstMetric(metrics: Record<string, number>, keys: string[]): { value: number; found: boolean } {
  for (const key of keys) {
    const value = metrics[key];
    if (typeof value === 'number' && Number.isFinite(value)) {
      return { value, found: true };
    }
  }
  return { value: 0, found: false };
}

function sumIfPresent(metrics: Record<string, number>, keys: string[]): { value: number; found: boolean } {
  let total = 0;
  let found = false;
  for (const key of keys) {
    const metric = firstMetric(metrics, [key]);
    if (!metric.found) {
      continue;
    }
    total += metric.value;
    found = true;
  }
  return { value: total, found };
}

function normalizePotentialMilliUnit(value: number, maxAbsCanonical: number): number {
  if (Math.abs(value) > maxAbsCanonical && Math.abs(value / 1000) <= maxAbsCanonical) {
    return value / 1000;
  }
  return value;
}

function isVolatileMetricKey(key: string): boolean {
  const clean = key.trim().toLowerCase();
  if (!clean) {
    return false;
  }
  if (['pvw', 'acw', 'dcw', 'loadw', 'batteryw', 'tempc', 'temp', 'temperature'].includes(clean)) {
    return true;
  }
  if (clean.includes('remaintime')) {
    return true;
  }
  if (clean.includes('watt') || clean.includes('pwr') || clean.includes('mppt')) {
    return true;
  }
  if (
    clean.includes('.pv') &&
    (clean.includes('invol') ||
      clean.includes('inamp') ||
      clean.includes('inwatt') ||
      clean.includes('chargewatt') ||
      clean.includes('chgstate'))
  ) {
    return true;
  }
  if (
    clean.endsWith('.invol') ||
    clean.endsWith('.inamp') ||
    clean.endsWith('.chgstate') ||
    clean.endsWith('.dcoutstate') ||
    clean.endsWith('.cfgacenabled')
  ) {
    return true;
  }
  if (clean.includes('.out') && (clean.includes('vol') || clean.includes('amp') || clean.includes('state'))) {
    return true;
  }
  return clean.endsWith('.batvol') || clean.endsWith('.batamp');
}

function isCurrentTelemetryMetricKey(key: string): boolean {
  const clean = key.trim().toLowerCase();
  if (!clean) {
    return false;
  }
  if (['pvw', 'acw', 'dcw', 'loadw', 'batteryw'].includes(clean)) {
    return true;
  }
  if (clean.includes('remaintime')) {
    return true;
  }
  if (clean.includes('watt') || clean.includes('pwr') || clean.includes('mppt')) {
    return true;
  }
  if (
    clean.includes('.pv') &&
    (clean.includes('invol') ||
      clean.includes('inamp') ||
      clean.includes('inwatt') ||
      clean.includes('chargewatt') ||
      clean.includes('chgstate'))
  ) {
    return true;
  }
  if (
    clean.endsWith('.invol') ||
    clean.endsWith('.inamp') ||
    clean.endsWith('.batvol') ||
    clean.endsWith('.batamp')
  ) {
    return true;
  }
  return clean.includes('.out') && (clean.includes('vol') || clean.includes('amp'));
}

function toRedisUrl(input: string): string {
  const trimmed = input.trim();
  if (trimmed.startsWith('redis://') || trimmed.startsWith('rediss://')) {
    return trimmed;
  }
  return `redis://${trimmed}`;
}

function toRedisNode(input: string): { host: string; port: number } {
  const parsed = new URL(toRedisUrl(input));
  return {
    host: parsed.hostname,
    port: parsed.port ? Number(parsed.port) : 6379
  };
}

function sharedSocketOptions() {
  return {
    reconnectStrategy(retries: number) {
      const baseDelayMs = Math.min(5_000, 250 * 2 ** retries);
      return baseDelayMs + Math.floor(Math.random() * Math.min(250, baseDelayMs * 0.2));
    }
  };
}
