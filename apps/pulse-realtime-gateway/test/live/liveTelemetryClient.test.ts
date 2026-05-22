import { describe, expect, it, vi } from 'vitest';

import type { DeviceAuthorizer } from '../../src/controlplane/deviceAuthorizer.js';
import { createLiveTelemetryClient } from '../../src/live/liveTelemetryClient.js';
import type { DeltaHub } from '../../src/live/natsDeltaHub.js';
import type { LiveDelta, LiveHeartbeat, LiveSnapshot, LiveSubscription } from '../../src/live/types.js';
import type { SnapshotStore } from '../../src/snapshot/valkeySnapshotStore.js';

class FakeAuthorizer implements DeviceAuthorizer {
  constructor(private readonly allowed = true, private readonly canonicalize: (deviceId: string) => string = (deviceId) => deviceId) {}

  async authorize(input: { deviceId: string }): Promise<{ canonicalDeviceId: string }> {
    if (!this.allowed) {
      throw Object.assign(new Error('forbidden'), { code: 7 });
    }
    return { canonicalDeviceId: this.canonicalize(input.deviceId) };
  }

  async listAuthorizedDevices(): Promise<{ deviceIds: string[] }> {
    return { deviceIds: [] };
  }

  close(): void {}
}

class FakeSnapshots implements SnapshotStore {
  constructor(private readonly snapshot: LiveSnapshot | null) {}

  async getSnapshot(): Promise<LiveSnapshot | null> {
    return this.snapshot;
  }

  async close(): Promise<void> {}
}

class FakeDeltaHub implements DeltaHub {
  listener: { onDelta: (delta: LiveDelta) => void; onHeartbeat: (heartbeat: LiveHeartbeat) => void } | null = null;

  async subscribe(_deviceId: string, listener: { onDelta: (delta: LiveDelta) => void; onHeartbeat: (heartbeat: LiveHeartbeat) => void }): Promise<LiveSubscription> {
    this.listener = listener;
    return { close: () => { this.listener = null; } };
  }

  async close(): Promise<void> {}
}

describe('createLiveTelemetryClient', () => {
  it('emits snapshot before buffered deltas', async () => {
    const authorizer = new FakeAuthorizer(true);
    const snapshots = new FakeSnapshots({
      deviceId: 'dev-1',
      cursor: { seq: 4, tsUnixMs: 1_000 },
      metrics: { 'params.soc': 40 }
    });
    const deltaHub = new FakeDeltaHub();
    const client = createLiveTelemetryClient({ authorizer, snapshots, deltaHub });
    const order: string[] = [];

    client.subscribe({
      deviceId: 'dev-1',
      deadlineMs: 1_000,
      onSnapshot(snapshot) {
        order.push(`snapshot:${snapshot.cursor.tsUnixMs}`);
      },
      onDelta(delta) {
        order.push(`delta:${delta.cursor.tsUnixMs}`);
      },
      onHeartbeat() {
        order.push('heartbeat');
      },
      onClose(error) {
        throw error ?? new Error('unexpected close');
      }
    });

    await vi.waitFor(() => expect(deltaHub.listener).not.toBeNull());
    deltaHub.listener?.onDelta({
      deviceId: 'dev-1',
      cursor: { seq: 5, tsUnixMs: 1_100 },
      changed: { 'params.wattsInSum': 120 },
      cleared: []
    });
    deltaHub.listener?.onHeartbeat({ deviceId: 'dev-1', cursor: { seq: 5, tsUnixMs: 1_100 } });

    await vi.waitFor(() => expect(order).toEqual(['snapshot:1000', 'delta:1100', 'heartbeat']));
    client.close();
  });

  it('closes immediately on authz denial', async () => {
    const client = createLiveTelemetryClient({
      authorizer: new FakeAuthorizer(false),
      snapshots: new FakeSnapshots(null),
      deltaHub: new FakeDeltaHub()
    });
    const onClose = vi.fn();

    client.subscribe({
      deviceId: 'dev-1',
      deadlineMs: 1_000,
      onSnapshot: vi.fn(),
      onDelta: vi.fn(),
      onHeartbeat: vi.fn(),
      onClose
    });

    await vi.waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onClose.mock.calls[0]?.[0]).toMatchObject({ code: 7 });
    client.close();
  });
});
