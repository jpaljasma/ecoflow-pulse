import { status as grpcStatus } from '@grpc/grpc-js';

import type { DeviceAuthorizer } from '../controlplane/deviceAuthorizer.js';
import type { SnapshotStore } from '../snapshot/valkeySnapshotStore.js';
import type { DeltaHub } from './natsDeltaHub.js';
import type {
  LiveDelta,
  LiveHeartbeat,
  LiveSnapshot,
  LiveSubscription,
  LiveTelemetryClient,
  SubscribeInput
} from './types.js';

export type {
  LiveDelta,
  LiveHeartbeat,
  LiveSnapshot,
  LiveSubscription,
  LiveTelemetryClient,
  SubscribeInput
};

export type LiveClientDeps = {
  authorizer: DeviceAuthorizer;
  snapshots: SnapshotStore;
  deltaHub: DeltaHub;
};

export function createLiveTelemetryClient(deps: LiveClientDeps): LiveTelemetryClient {
  return {
    subscribe(input: SubscribeInput): LiveSubscription {
      let closed = false;
      let settled = false;
      let deltaSubscription: LiveSubscription | null = null;
      const bufferedEvents: Array<{ type: 'delta'; delta: LiveDelta } | { type: 'heartbeat'; heartbeat: LiveHeartbeat }> = [];
      let snapshotReady = false;
      let snapshotCursorTs = 0;

      const settleClose = (error?: Error & { code?: number }) => {
        if (closed || settled) {
          return;
        }
        settled = true;
        input.onClose(error);
      };

      void (async () => {
        try {
          const authz = await deps.authorizer.authorize({
            deviceId: input.deviceId,
            authHeader: input.authHeader,
            requestID: input.requestID,
            deadlineMs: input.deadlineMs
          });
          if (closed) {
            return;
          }

          deltaSubscription = await deps.deltaHub.subscribe(authz.canonicalDeviceId, {
            onDelta(delta) {
              if (closed) {
                return;
              }
              if (!snapshotReady) {
                bufferedEvents.push({ type: 'delta', delta });
                return;
              }
              if (delta.cursor.tsUnixMs >= snapshotCursorTs) {
                input.onDelta(delta);
              }
            },
            onHeartbeat(heartbeat) {
              if (closed) {
                return;
              }
              if (!snapshotReady) {
                bufferedEvents.push({ type: 'heartbeat', heartbeat });
                return;
              }
              input.onHeartbeat(heartbeat);
            }
          });

          const snapshot = await deps.snapshots.getSnapshot(authz.canonicalDeviceId);
          if (closed) {
            return;
          }

          if (snapshot) {
            snapshotCursorTs = snapshot.cursor.tsUnixMs;
            input.onSnapshot(snapshot);
          }
          snapshotReady = true;

          for (const event of bufferedEvents.splice(0)) {
            if (event.type === 'heartbeat') {
              input.onHeartbeat(event.heartbeat);
              continue;
            }
            if (event.delta.cursor.tsUnixMs >= snapshotCursorTs) {
              input.onDelta(event.delta);
            }
          }
        } catch (error) {
          if (deltaSubscription) {
            deltaSubscription.close();
            deltaSubscription = null;
          }
          settleClose(normalizeError(error));
        }
      })();

      return {
        close() {
          if (closed) {
            return;
          }
          closed = true;
          deltaSubscription?.close();
          deltaSubscription = null;
        }
      };
    },
    close(): void {
      deps.authorizer.close();
      void deps.snapshots.close();
      void deps.deltaHub.close();
    }
  };
}

function normalizeError(error: unknown): Error & { code?: number } {
  if (error instanceof Error) {
    return error as Error & { code?: number };
  }
  return Object.assign(new Error('live telemetry client failed'), { code: grpcStatus.INTERNAL });
}
