import Fastify, { type FastifyInstance, type preValidationHookHandler } from 'fastify';
import websocket from '@fastify/websocket';
import { status as grpcStatus } from '@grpc/grpc-js';
import type WebSocket from 'ws';
import type { RawData } from 'ws';

import { buildWsPreValidation } from './auth.js';
import type { AppConfig } from './config.js';
import { DeliveryLane } from './live/deliveryLane.js';
import type {
  LiveSubscription,
  LiveTelemetryClient
} from './live/liveTelemetryClient.js';
import {
  ClientMessageSchema,
  type ServerDeviceStatusMessage,
  type ServerPongMessage,
  type ServerTelemetryMessage
} from './schemas.js';
import { realtimeMetrics } from './metrics.js';

type BuildAppOptions = {
  wsPreValidation?: preValidationHookHandler;
};

type DeviceStreamState = {
  deviceId: string;
  reconnectAttempt: number;
  reconnectTimer: ReturnType<typeof setTimeout> | null;
  activeStream: LiveSubscription | null;
  deliveryLane: DeliveryLane;
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
  app.get('/metrics', async (_request, reply) => {
    reply.header('Content-Type', realtimeMetrics.contentType());
    return await realtimeMetrics.render();
  });

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
    realtimeMetrics.sessionOpened();
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
        this.send({ type: 'pong', ts: Date.now() });
        break;
    }
  }

  close(): void {
    if (this.closed) {
      return;
    }
    this.closed = true;
    realtimeMetrics.sessionClosed();
    for (const state of this.deviceStreams.values()) {
      this.stopState(state);
    }
    this.deviceStreams.clear();
  }

  private subscribeDevice(deviceId: string): void {
    if (this.deviceStreams.has(deviceId)) {
      return;
    }
    realtimeMetrics.recordSubscriptionOutcome('requested');

    const state: DeviceStreamState = {
      deviceId,
      reconnectAttempt: 0,
      reconnectTimer: null,
      activeStream: null,
      deliveryLane: new DeliveryLane({
        deviceId,
        socket: this.socket,
        config: this.config.delivery,
        emitTelemetry: (message) => {
          this.send(message);
        },
        emitStatus: (message) => {
          this.send(message);
        }
      }),
      disposed: false
    };

    this.deviceStreams.set(deviceId, state);
    this.openStream(state);
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

  private openStream(state: DeviceStreamState): void {
    if (this.closed || state.disposed) {
      return;
    }
    this.stopActiveStream(state);

    state.activeStream = this.liveClient.subscribe({
      deviceId: state.deviceId,
      authHeader: this.authHeader,
      requestID: this.requestId,
      deadlineMs: this.config.grpcDeadlineMs,
      onSnapshot: (snapshot) => {
        state.reconnectAttempt = 0;
        state.deliveryLane.applySnapshot(timestamp(snapshot.cursor.tsUnixMs), snapshot.metrics);
      },
      onDelta: (delta) => {
        state.reconnectAttempt = 0;
        state.deliveryLane.applyDelta(timestamp(delta.cursor.tsUnixMs), delta.changed, delta.cleared);
      },
      onHeartbeat: (heartbeat) => {
        state.deliveryLane.applyHeartbeat(timestamp(heartbeat.cursor.tsUnixMs));
      },
      onClose: (error) => {
        state.activeStream = null;
        if (this.closed || state.disposed) {
          return;
        }
        this.sendDeviceStatus(state.deviceId, false, Date.now());
        if (isPermissionDenied(error)) {
          realtimeMetrics.recordSubscriptionOutcome('forbidden');
        }
        if (!shouldReconnect(error)) {
          this.stopState(state);
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
      this.openStream(state);
    }, delay);
  }

  private stopState(state: DeviceStreamState): void {
    if (state.reconnectTimer) {
      clearTimeout(state.reconnectTimer);
      state.reconnectTimer = null;
    }
    this.stopActiveStream(state);
    state.deliveryLane.close();
  }

  private stopActiveStream(state: DeviceStreamState): void {
    if (!state.activeStream) {
      return;
    }
    state.activeStream.close();
    state.activeStream = null;
  }

  private sendDeviceStatus(deviceId: string, online: boolean, ts: number): void {
    this.send({
      type: 'device_status',
      deviceId,
      ts,
      online
    } satisfies ServerDeviceStatusMessage);
  }

  private send(message: ServerTelemetryMessage | ServerDeviceStatusMessage | ServerPongMessage): void {
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

function isPermissionDenied(error?: (Error & { code?: number }) | undefined): boolean {
  return Boolean(error && typeof error.code === 'number' && error.code === grpcStatus.PERMISSION_DENIED);
}

function computeBackoffWithJitter(baseMs: number, maxMs: number, attempt: number): number {
  const exponential = Math.min(maxMs, baseMs * 2 ** Math.max(0, attempt - 1));
  return Math.max(baseMs, Math.floor(exponential / 2 + Math.random() * (exponential / 2)));
}

function timestamp(cursorTsUnixMs: number): number {
  return cursorTsUnixMs > 0 ? cursorTsUnixMs : Date.now();
}
