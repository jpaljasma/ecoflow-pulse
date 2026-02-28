import { createClient, createCluster } from 'redis';

import type { LiveSnapshot } from '../live/types.js';

type RedisLike = {
  get(key: string): Promise<string | null>;
  quit(): Promise<unknown>;
};

export interface SnapshotStore {
  getSnapshot(deviceId: string): Promise<LiveSnapshot | null>;
  close(): Promise<void>;
}

export type ValkeySnapshotStoreConfig = {
  addrs: string[];
  username?: string;
  password?: string;
  keyPrefix: string;
};

export class ValkeySnapshotStore implements SnapshotStore {
  private readonly cfg: ValkeySnapshotStoreConfig;
  private clientPromise: Promise<RedisLike> | null = null;

  constructor(cfg: ValkeySnapshotStoreConfig) {
    this.cfg = cfg;
  }

  async getSnapshot(deviceId: string): Promise<LiveSnapshot | null> {
    const client = await this.getClient();
    const key = `${this.cfg.keyPrefix}:{did:${sanitizeKeySegment(deviceId)}}:snapshot`;
    const raw = await client.get(key);
    if (!raw) {
      return null;
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
    };
    return {
      deviceId: typeof value.device_id === 'string' ? value.device_id : deviceId,
      cursor: {
        seq: typeof value.cursor_seq === 'number' ? value.cursor_seq : 0,
        tsUnixMs: typeof value.cursor_ts_unix_ms === 'number' ? value.cursor_ts_unix_ms : 0
      },
      metrics: value.metrics && typeof value.metrics === 'object' ? value.metrics : {}
    };
  }

  async close(): Promise<void> {
    if (!this.clientPromise) {
      return;
    }
    const client = await this.clientPromise.catch(() => null);
    this.clientPromise = null;
    if (client) {
      await client.quit().catch(() => undefined);
    }
  }

  private getClient(): Promise<RedisLike> {
    if (!this.clientPromise) {
      this.clientPromise = this.createClient();
    }
    return this.clientPromise;
  }

  private async createClient(): Promise<RedisLike> {
    const addrs = this.cfg.addrs.filter((value) => value.trim() !== '');
    if (addrs.length <= 1) {
      const [host = '127.0.0.1:6379'] = addrs;
      const client = createClient({
        url: toRedisUrl(host),
        username: this.cfg.username,
        password: this.cfg.password,
        socket: {
          reconnectStrategy(retries) {
            return Math.min(5_000, 250 * 2 ** retries);
          }
        }
      });
      await client.connect();
      return client;
    }

    const client = createCluster({
      rootNodes: addrs.map((addr) => ({ url: toRedisUrl(addr) })),
      defaults: {
        username: this.cfg.username,
        password: this.cfg.password,
        socket: {
          reconnectStrategy(retries) {
            return Math.min(5_000, 250 * 2 ** retries);
          }
        }
      }
    });
    await client.connect();
    return client;
  }
}

function sanitizeKeySegment(input: string): string {
  return input.trim().replaceAll('{', '_').replaceAll('}', '_').replaceAll(' ', '_');
}

function toRedisUrl(input: string): string {
  const trimmed = input.trim();
  if (trimmed.startsWith('redis://') || trimmed.startsWith('rediss://')) {
    return trimmed;
  }
  return `redis://${trimmed}`;
}
