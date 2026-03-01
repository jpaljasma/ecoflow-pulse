import { createClient } from 'redis';

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
  private clientPromises: Promise<RedisLike>[] | null = null;

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
    return null;
  }

  async close(): Promise<void> {
    if (!this.clientPromises) {
      return;
    }
    const clients = await Promise.all(this.clientPromises.map((client) => client.catch(() => null)));
    this.clientPromises = null;
    for (const client of clients) {
      if (client) {
        await client.quit().catch(() => undefined);
      }
    }
  }

  private getClients(): Promise<RedisLike[]> {
    if (!this.clientPromises) {
      this.clientPromises = this.createClients();
    }
    return Promise.all(this.clientPromises);
  }

  private createClients(): Promise<RedisLike>[] {
    const addrs = this.cfg.addrs.filter((value) => value.trim() !== '');
    const hosts = addrs.length > 0 ? addrs : ['127.0.0.1:6379'];
    return hosts.map(async (host) => {
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

function toRedisUrl(input: string): string {
  const trimmed = input.trim();
  if (trimmed.startsWith('redis://') || trimmed.startsWith('rediss://')) {
    return trimmed;
  }
  return `redis://${trimmed}`;
}
