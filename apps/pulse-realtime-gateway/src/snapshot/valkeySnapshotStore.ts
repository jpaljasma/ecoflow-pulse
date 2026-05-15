import { createClient, createSentinel } from 'redis';

import type { LiveSnapshot } from '../live/types.js';

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
