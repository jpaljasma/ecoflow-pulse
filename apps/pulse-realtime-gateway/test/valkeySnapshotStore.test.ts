import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { EventEmitter } from 'node:events';

const createClientMock = vi.fn();
const createSentinelMock = vi.fn();

vi.mock('redis', () => ({
  createClient: createClientMock,
  createSentinel: createSentinelMock
}));

describe('ValkeySnapshotStore', () => {
  beforeEach(() => {
    createClientMock.mockReset();
    createSentinelMock.mockReset();
  });

  afterEach(() => {
    vi.resetModules();
  });

  it('uses Sentinel when a master set is configured', async () => {
    const sentinelClient = {
      connect: vi.fn().mockResolvedValue(undefined),
      get: vi.fn().mockResolvedValue(null),
      close: vi.fn().mockResolvedValue(undefined)
    };
    createSentinelMock.mockReturnValue(sentinelClient);

    const { ValkeySnapshotStore } = await import('../src/snapshot/valkeySnapshotStore.js');
    const store = new ValkeySnapshotStore({
      addrs: ['127.0.0.1:26379'],
      sentinelMasterSet: 'myprimary',
      sentinelUsername: 'sentinel-user',
      sentinelPassword: 'sentinel-pass',
      username: 'data-user',
      password: 'data-pass',
      keyPrefix: 'pulse:projection'
    });

    await store.getSnapshot('dev-1');
    await store.close();

    expect(createSentinelMock).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'myprimary',
        sentinelRootNodes: [{ host: '127.0.0.1', port: 26379 }],
        nodeClientOptions: expect.objectContaining({
          username: 'data-user',
          password: 'data-pass'
        }),
        sentinelClientOptions: expect.objectContaining({
          username: 'sentinel-user',
          password: 'sentinel-pass'
        })
      })
    );
    expect(createClientMock).not.toHaveBeenCalled();
    expect(sentinelClient.connect).toHaveBeenCalledTimes(1);
    expect(sentinelClient.close).toHaveBeenCalledTimes(1);
  });

  it('handles Valkey socket errors without crashing and reconnects on the next read', async () => {
    const firstClient = makeClient();
    const secondClient = makeClient();
    createClientMock
      .mockReturnValueOnce(firstClient)
      .mockReturnValueOnce(secondClient);

    const { ValkeySnapshotStore } = await import('../src/snapshot/valkeySnapshotStore.js');
    const store = new ValkeySnapshotStore({
      addrs: ['127.0.0.1:26380'],
      keyPrefix: 'pulse:projection'
    });

    await store.getSnapshot('dev-1');

    expect(() => firstClient.emit('error', new Error('socket closed unexpectedly'))).not.toThrow();

    await store.getSnapshot('dev-1');
    await store.close();

    expect(createClientMock).toHaveBeenCalledTimes(2);
    expect(firstClient.close).toHaveBeenCalledTimes(1);
    expect(secondClient.connect).toHaveBeenCalledTimes(1);
  });

  it('ignores stale end events from an old Valkey client after reconnecting', async () => {
    const firstClient = makeClient();
    const secondClient = makeClient();
    let resolveOldClose: (() => void) | undefined;
    firstClient.close = vi.fn().mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveOldClose = resolve;
        })
    );
    createClientMock
      .mockReturnValueOnce(firstClient)
      .mockReturnValueOnce(secondClient);

    const { ValkeySnapshotStore } = await import('../src/snapshot/valkeySnapshotStore.js');
    const store = new ValkeySnapshotStore({
      addrs: ['127.0.0.1:26380'],
      keyPrefix: 'pulse:projection'
    });

    await store.getSnapshot('dev-1');
    firstClient.emit('error', new Error('socket closed unexpectedly'));
    await store.getSnapshot('dev-1');

    firstClient.emit('end');
    await store.getSnapshot('dev-1');
    resolveOldClose?.();
    await store.close();

    expect(createClientMock).toHaveBeenCalledTimes(2);
    expect(secondClient.close).toHaveBeenCalledTimes(1);
  });
});

function makeClient() {
  const client = new EventEmitter() as EventEmitter & {
    connect: ReturnType<typeof vi.fn>;
    get: ReturnType<typeof vi.fn>;
    close: ReturnType<typeof vi.fn>;
  };
  client.connect = vi.fn().mockResolvedValue(undefined);
  client.get = vi.fn().mockResolvedValue(null);
  client.close = vi.fn().mockResolvedValue(undefined);
  return client;
}
