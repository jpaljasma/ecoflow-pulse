import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

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
});
