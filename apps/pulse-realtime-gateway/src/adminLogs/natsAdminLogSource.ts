import {
  connect,
  consumerOpts,
  type ConnectionOptions,
  type JetStreamPullSubscription,
  type NatsConnection,
  type Subscription
} from 'nats';

import { decodeEnvelope } from '../live/envelopeCodec.js';
import { ingestWildcardSubject } from '../live/natsSubjects.js';
import { adminLogEntryFromEnvelope, matchesAdminLogFilters } from './logEntry.js';
import type { AdminLogSource, AdminLogSubscribeInput, AdminLogSubscription } from './types.js';

export type NatsAdminLogSourceConfig = {
  urls: string[];
  subjectPrefix: string;
  streamName: string;
};

export type NatsConnectionFactory = (options: ConnectionOptions) => Promise<NatsConnection>;

export class NatsAdminLogSource implements AdminLogSource {
  private readonly cfg: NatsAdminLogSourceConfig;
  private readonly connectFn: NatsConnectionFactory;
  private conn: NatsConnection | null = null;
  private connPromise: Promise<NatsConnection> | null = null;
  private closed = false;

  constructor(cfg: NatsAdminLogSourceConfig, connectFn: NatsConnectionFactory = connect) {
    this.cfg = cfg;
    this.connectFn = connectFn;
  }

  subscribe(input: AdminLogSubscribeInput): AdminLogSubscription {
    const controller = new AbortController();
    void this.run(input, controller.signal);
    return {
      close: () => {
        controller.abort();
      }
    };
  }

  async close(): Promise<void> {
    this.closed = true;
    const conn = this.conn;
    this.conn = null;
    this.connPromise = null;
    if (conn) {
      await conn.drain().catch(() => undefined);
    }
  }

  private async run(input: AdminLogSubscribeInput, signal: AbortSignal): Promise<void> {
    let liveSubscription: Subscription | null = null;
    try {
      const conn = await this.ensureConnected();
      if (signal.aborted) {
        return;
      }

      input.onStatus({ state: 'replay' });
      const replayed = await this.replayRecent(conn, input, signal);
      if (signal.aborted) {
        return;
      }
      input.onReplayDone({ replayed });
      input.onStatus({ state: 'live' });

      liveSubscription = conn.subscribe(ingestWildcardSubject(this.cfg.subjectPrefix));
      for await (const message of liveSubscription) {
        if (signal.aborted || this.closed) {
          break;
        }
        this.emitEnvelope(message.data, input);
      }
    } catch (error) {
      if (!signal.aborted && !this.closed) {
        input.onStatus({ state: 'error', message: error instanceof Error ? error.message : 'log stream failed' });
      }
    } finally {
      liveSubscription?.unsubscribe();
    }
  }

  private async replayRecent(
    conn: NatsConnection,
    input: AdminLogSubscribeInput,
    signal: AbortSignal
  ): Promise<number> {
    const js = conn.jetstream();
    const subject = ingestWildcardSubject(this.cfg.subjectPrefix);
    const opts = consumerOpts();
    opts.bindStream(this.cfg.streamName);
    opts.filterSubject(subject);
    opts.startTime(new Date(input.replaySinceUnixMs));
    opts.ackNone();
    opts.replayInstantly();
    opts.inactiveEphemeralThreshold(30_000);

    let sub: JetStreamPullSubscription | null = null;
    let stopReplay: ReturnType<typeof setTimeout> | null = null;
    let replayed = 0;
    let replayTimedOut = false;
    try {
      sub = await js.pullSubscribe(subject, opts);
      sub.pull({ batch: input.replayLimit, expires: 350 });

      const deadline = Date.now() + 1_000;
      stopReplay = setTimeout(() => {
        replayTimedOut = true;
        void sub?.destroy().catch(() => undefined);
      }, 1_000);
      for await (const message of sub) {
        if (signal.aborted || this.closed) {
          break;
        }
        if (this.emitEnvelope(message.data, input)) {
          replayed += 1;
        }
        if (replayed >= input.replayLimit || Date.now() >= deadline) {
          break;
        }
      }
      return replayed;
    } catch (error) {
      if (signal.aborted || this.closed || replayTimedOut) {
        return replayed;
      }
      throw error;
    } finally {
      if (stopReplay) {
        clearTimeout(stopReplay);
      }
      await sub?.destroy().catch(() => undefined);
    }
  }

  private emitEnvelope(data: Uint8Array, input: AdminLogSubscribeInput): boolean {
    const envelope = decodeEnvelope(data);
    if (!envelope || !envelope.deviceId) {
      return false;
    }
    const entry = adminLogEntryFromEnvelope(envelope, Date.now());
    if (!matchesAdminLogFilters(entry, input.filters)) {
      return false;
    }
    input.onEntry(entry);
    return true;
  }

  private async ensureConnected(): Promise<NatsConnection> {
    if (this.closed) {
      throw new Error('admin log source is closed');
    }
    if (this.conn) {
      return this.conn;
    }
    if (!this.connPromise) {
      this.connPromise = this.connectFn({
        servers: this.cfg.urls,
        name: 'pulse-realtime-gateway-admin-logs',
        maxReconnectAttempts: -1,
        reconnect: true,
        pingInterval: 20_000,
        maxPingOut: 3,
        reconnectTimeWait: 2_000
      }).then((conn) => {
        this.conn = conn;
        return conn;
      });
    }
    return this.connPromise;
  }
}
