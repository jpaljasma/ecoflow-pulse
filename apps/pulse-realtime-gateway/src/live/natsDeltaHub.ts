import { connect, type ConnectionOptions, type NatsConnection, type Subscription } from 'nats';

import { decodeEnvelope } from './envelopeCodec.js';
import { extractNumericMetrics } from './flattenMetrics.js';
import { ingestWildcardSubject } from './natsSubjects.js';
import type { LiveDelta, LiveHeartbeat, LiveSubscription } from './types.js';

export type DeltaListener = {
  onDelta: (delta: LiveDelta) => void;
  onHeartbeat: (heartbeat: LiveHeartbeat) => void;
};

export interface DeltaHub {
  subscribe(deviceId: string, listener: DeltaListener): Promise<LiveSubscription>;
  close(): Promise<void>;
}

export type NatsDeltaHubConfig = {
  urls: string[];
  subjectPrefix: string;
};

export type NatsConnectionFactory = (options: ConnectionOptions) => Promise<NatsConnection>;

export class NatsDeltaHub implements DeltaHub {
  private readonly listenersByDevice = new Map<string, Set<DeltaListener>>();
  private readonly sequenceByDevice = new Map<string, number>();
  private readonly cfg: NatsDeltaHubConfig;
  private readonly connectFn: NatsConnectionFactory;
  private startedPromise: Promise<void> | null = null;
  private conn: NatsConnection | null = null;
  private subscription: Subscription | null = null;
  private closed = false;

  constructor(cfg: NatsDeltaHubConfig, connectFn: NatsConnectionFactory = connect) {
    this.cfg = cfg;
    this.connectFn = connectFn;
  }

  async subscribe(deviceId: string, listener: DeltaListener): Promise<LiveSubscription> {
    await this.ensureStarted();
    const bucket = this.listenersByDevice.get(deviceId) ?? new Set<DeltaListener>();
    bucket.add(listener);
    this.listenersByDevice.set(deviceId, bucket);

    let active = true;
    return {
      close: () => {
        if (!active) {
          return;
        }
        active = false;
        const current = this.listenersByDevice.get(deviceId);
        if (!current) {
          return;
        }
        current.delete(listener);
        if (current.size === 0) {
          this.listenersByDevice.delete(deviceId);
        }
      }
    };
  }

  async close(): Promise<void> {
    this.closed = true;
    this.listenersByDevice.clear();
    this.sequenceByDevice.clear();
    if (this.subscription) {
      this.subscription.unsubscribe();
      this.subscription = null;
    }
    if (this.conn) {
      await this.conn.drain().catch(() => undefined);
      this.conn = null;
    }
  }

  private async ensureStarted(): Promise<void> {
    if (this.closed) {
      throw new Error('nats delta hub is closed');
    }
    if (this.startedPromise) {
      return this.startedPromise;
    }
    this.startedPromise = this.start().catch((error) => {
      this.startedPromise = null;
      throw error;
    });
    return this.startedPromise;
  }

  private async start(): Promise<void> {
    this.conn = await this.connectFn({
      servers: this.cfg.urls,
      name: 'pulse-realtime-gateway',
      maxReconnectAttempts: -1,
      reconnect: true,
      pingInterval: 20_000,
      maxPingOut: 3,
      reconnectTimeWait: 2_000
    });
    this.subscription = this.conn.subscribe(ingestWildcardSubject(this.cfg.subjectPrefix));
    void this.consumeLoop(this.subscription);
  }

  private async consumeLoop(subscription: Subscription): Promise<void> {
    for await (const message of subscription) {
      if (this.closed) {
        break;
      }
      const envelope = decodeEnvelope(message.data);
      if (!envelope || !envelope.deviceId) {
        continue;
      }
      const listeners = this.listenersByDevice.get(envelope.deviceId);
      if (!listeners || listeners.size === 0) {
        continue;
      }
      const changed = extractNumericMetrics(envelope.payload);
      const cursor = {
        seq: this.nextSequence(envelope.deviceId),
        tsUnixMs: envelope.ingestedTimeUnixMs || envelope.observedTimeUnixMs || envelope.deviceTimeUnixMs || Date.now()
      };
      const heartbeat: LiveHeartbeat = {
        deviceId: envelope.deviceId,
        cursor
      };
      const delta: LiveDelta = {
        deviceId: envelope.deviceId,
        cursor,
        changed,
        cleared: []
      };
      for (const listener of listeners) {
        listener.onHeartbeat(heartbeat);
        if (Object.keys(changed).length > 0) {
          listener.onDelta(delta);
        }
      }
    }
  }

  private nextSequence(deviceId: string): number {
    const next = (this.sequenceByDevice.get(deviceId) ?? 0) + 1;
    this.sequenceByDevice.set(deviceId, next);
    return next;
  }
}
