import Fastify, { type FastifyInstance, type preValidationHookHandler } from 'fastify';
import websocket from '@fastify/websocket';
import { status as grpcStatus } from '@grpc/grpc-js';
import type WebSocket from 'ws';
import type { RawData } from 'ws';

import { buildWsPreValidation } from './auth.js';
import type { AppConfig } from './config.js';
import type {
  LiveSubscription,
  LiveTelemetryClient
} from './grpc/liveTelemetryClient.js';
import { ClientMessageSchema, type ServerDeviceStatusMessage, type ServerTelemetryMessage } from './schemas.js';
import { deriveTelemetryMetrics, mergeRawMetrics, type RawTelemetryMetrics } from './telemetryMap.js';

type BuildAppOptions = {
  wsPreValidation?: preValidationHookHandler;
};

type DeviceStreamState = {
  deviceId: string;
  rawMetrics: RawTelemetryMetrics;
  reconnectAttempt: number;
  reconnectTimer: ReturnType<typeof setTimeout> | null;
  activeStream: LiveSubscription | null;
  disposed: boolean;
};

type GatewaySessionDeps = {
  socket: WebSocket;
  requestId: string;
  authHeader?: string;
  config: AppConfig;
  liveClient: LiveTelemetryClient;
};

export function buildApp(
  config: AppConfig,
  liveClient: LiveTelemetryClient,
  options: BuildAppOptions = {}
): FastifyInstance {
  const app = Fastify({ logger: false });
  const wsPreValidation = options.wsPreValidation ?? buildWsPreValidation(config);

  app.get('/healthz', async () => ({ ok: true }));

  void app.register(async (scopedApp) => {
    await scopedApp.register(websocket);
    scopedApp.get(
      '/ws',
      { websocket: true, preValidation: wsPreValidation },
      (socket, request) => {
        const session = new GatewaySession({
          socket,
          requestId: request.id,
          authHeader: request.wsAuthHeader,
          config,
          liveClient
        });

        socket.on('message', (raw: RawData) => {
          session.handleMessage(raw.toString());
        });
        socket.on('close', () => {
          session.close();
        });
        socket.on('error', () => {
          session.close();
        });
      }
    );
  });

  app.addHook('onClose', async () => {
    liveClient.close();
  });

  return app;
}

class GatewaySession {
  private readonly socket: WebSocket;
  private readonly requestId: string;
  private readonly authHeader?: string;
  private readonly config: AppConfig;
  private readonly liveClient: LiveTelemetryClient;
  private readonly deviceStreams = new Map<string, DeviceStreamState>();
  private closed = false;

  constructor(deps: GatewaySessionDeps) {
    this.socket = deps.socket;
    this.requestId = deps.requestId;
    this.authHeader = deps.authHeader;
    this.config = deps.config;
    this.liveClient = deps.liveClient;
  }

  handleMessage(raw: string): void {
    if (this.closed) {
      return;
    }
    let parsed: unknown;
    try {
      parsed = JSON.parse(raw);
    } catch {
      return;
    }

    const message = ClientMessageSchema.safeParse(parsed);
    if (!message.success) {
      return;
    }

    switch (message.data.type) {
      case 'subscribe':
        for (const deviceId of message.data.deviceIds) {
          this.subscribeDevice(deviceId);
        }
        break;
      case 'unsubscribe':
        for (const deviceId of message.data.deviceIds) {
          this.unsubscribeDevice(deviceId);
        }
        break;
      case 'ping':
        break;
    }
  }

  close(): void {
    if (this.closed) {
      return;
    }
    this.closed = true;
    for (const state of this.deviceStreams.values()) {
      this.stopState(state);
    }
    this.deviceStreams.clear();
  }

  private subscribeDevice(deviceId: string): void {
    if (this.deviceStreams.has(deviceId)) {
      return;
    }
    const state: DeviceStreamState = {
      deviceId,
      rawMetrics: {},
      reconnectAttempt: 0,
      reconnectTimer: null,
      activeStream: null,
      disposed: false
    };
    this.deviceStreams.set(deviceId, state);
    this.openStream(state, true);
  }

  private unsubscribeDevice(deviceId: string): void {
    const state = this.deviceStreams.get(deviceId);
    if (!state) {
      return;
    }
    state.disposed = true;
    this.stopState(state);
    this.deviceStreams.delete(deviceId);
    this.sendDeviceStatus(deviceId, false, Date.now());
  }

  private openStream(state: DeviceStreamState, includeInitialSnapshot: boolean): void {
    if (this.closed || state.disposed) {
      return;
    }
    this.stopActiveStream(state);

    state.activeStream = this.liveClient.subscribe({
      deviceId: state.deviceId,
      includeInitialSnapshot,
      maxUpdateHz: this.config.subscribeUpdateHz,
      authHeader: this.authHeader,
      requestID: this.requestId,
      deadlineMs: this.config.grpcDeadlineMs,
      onSnapshot: (snapshot) => {
        state.reconnectAttempt = 0;
        state.rawMetrics = { ...snapshot.metrics };
        this.sendDeviceStatus(state.deviceId, true, timestamp(snapshot.cursor.tsUnixMs));
        this.sendTelemetry(state.deviceId, timestamp(snapshot.cursor.tsUnixMs), state.rawMetrics);
      },
      onDelta: (delta) => {
        state.reconnectAttempt = 0;
        state.rawMetrics = mergeRawMetrics(state.rawMetrics, delta.changed, delta.cleared);
        this.sendTelemetry(state.deviceId, timestamp(delta.cursor.tsUnixMs), state.rawMetrics);
      },
      onHeartbeat: (heartbeat) => {
        this.sendDeviceStatus(state.deviceId, true, timestamp(heartbeat.cursor.tsUnixMs));
      },
      onClose: (error) => {
        state.activeStream = null;
        if (this.closed || state.disposed) {
          return;
        }
        this.sendDeviceStatus(state.deviceId, false, Date.now());
        if (!shouldReconnect(error)) {
          this.deviceStreams.delete(state.deviceId);
          return;
        }
        this.scheduleReconnect(state);
      }
    });
  }

  private scheduleReconnect(state: DeviceStreamState): void {
    if (this.closed || state.disposed || state.reconnectTimer) {
      return;
    }
    state.reconnectAttempt += 1;
    const delay = computeBackoffWithJitter(
      this.config.reconnectBackoff.baseMs,
      this.config.reconnectBackoff.maxMs,
      state.reconnectAttempt
    );
    state.reconnectTimer = setTimeout(() => {
      state.reconnectTimer = null;
      this.openStream(state, true);
    }, delay);
  }

  private stopState(state: DeviceStreamState): void {
    if (state.reconnectTimer) {
      clearTimeout(state.reconnectTimer);
      state.reconnectTimer = null;
    }
    this.stopActiveStream(state);
  }

  private stopActiveStream(state: DeviceStreamState): void {
    if (!state.activeStream) {
      return;
    }
    state.activeStream.close();
    state.activeStream = null;
  }

  private sendTelemetry(deviceId: string, ts: number, rawMetrics: RawTelemetryMetrics): void {
    const metrics = deriveTelemetryMetrics(rawMetrics);
    this.send({
      type: 'telemetry',
      deviceId,
      ts,
      metrics
    } satisfies ServerTelemetryMessage);
  }

  private sendDeviceStatus(deviceId: string, online: boolean, ts: number): void {
    this.send({
      type: 'device_status',
      deviceId,
      ts,
      online
    } satisfies ServerDeviceStatusMessage);
  }

  private send(message: ServerTelemetryMessage | ServerDeviceStatusMessage): void {
    if (this.closed || this.socket.readyState !== 1) {
      return;
    }
    try {
      this.socket.send(JSON.stringify(message));
    } catch {
      this.close();
    }
  }
}

function shouldReconnect(error?: (Error & { code?: number }) | undefined): boolean {
  if (!error || typeof error.code !== 'number') {
    return true;
  }
  return ![
    grpcStatus.INVALID_ARGUMENT,
    grpcStatus.NOT_FOUND,
    grpcStatus.PERMISSION_DENIED,
    grpcStatus.UNAUTHENTICATED
  ].includes(error.code);
}

function computeBackoffWithJitter(baseMs: number, maxMs: number, attempt: number): number {
  const exponential = Math.min(maxMs, baseMs * 2 ** Math.max(0, attempt - 1));
  return Math.max(baseMs, Math.floor(exponential / 2 + Math.random() * (exponential / 2)));
}

function timestamp(cursorTsUnixMs: number): number {
  return cursorTsUnixMs > 0 ? cursorTsUnixMs : Date.now();
}
