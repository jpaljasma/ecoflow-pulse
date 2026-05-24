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

  it('filters stale volatile metrics using projection metric timestamps', async () => {
    const now = new Date('2026-05-24T12:00:00.000Z');
    vi.useFakeTimers();
    vi.setSystemTime(now);
    try {
      const client = makeClient();
      client.get.mockResolvedValue(
        JSON.stringify({
          device_id: 'dev-1',
          cursor_seq: 10,
          cursor_ts_unix_ms: now.getTime() - 10_000,
          metrics: {
            'params.pv1ChargeWatts': 46,
            'params.wattsOutSum': 12,
            'params.f32ShowSoc': 80
          },
          metric_observed_at_unix_ms: {
            'params.pv1ChargeWatts': now.getTime() - 5 * 60_000,
            'params.wattsOutSum': now.getTime() - 5 * 60_000,
            'params.f32ShowSoc': now.getTime() - 10_000
          }
        })
      );
      createClientMock.mockReturnValue(client);

      const { ValkeySnapshotStore } = await import('../src/snapshot/valkeySnapshotStore.js');
      const store = new ValkeySnapshotStore({
        addrs: ['127.0.0.1:26380'],
        keyPrefix: 'pulse:projection'
      });

      const snapshot = await store.getSnapshot('dev-1');
      await store.close();

      expect(snapshot?.metrics).toEqual({
        'params.f32ShowSoc': 80
      });
    } finally {
      vi.useRealTimers();
    }
  });

  it('keeps unchanged trickle metrics when their projection timestamp is fresh', async () => {
    const now = new Date('2026-05-24T12:00:00.000Z');
    vi.useFakeTimers();
    vi.setSystemTime(now);
    try {
      const client = makeClient();
      client.get.mockResolvedValue(
        JSON.stringify({
          device_id: 'dev-1',
          cursor_seq: 11,
          cursor_ts_unix_ms: now.getTime() - 20_000,
          metrics: {
            'params.pv1ChargeWatts': 2,
            'params.wattsOutSum': 0,
            'params.f32ShowSoc': 44
          },
          metric_observed_at_unix_ms: {
            'params.pv1ChargeWatts': now.getTime() - 20_000,
            'params.wattsOutSum': now.getTime() - 20_000,
            'params.f32ShowSoc': now.getTime() - 20_000
          }
        })
      );
      createClientMock.mockReturnValue(client);

      const { ValkeySnapshotStore } = await import('../src/snapshot/valkeySnapshotStore.js');
      const store = new ValkeySnapshotStore({
        addrs: ['127.0.0.1:26380'],
        keyPrefix: 'pulse:projection'
      });

      const snapshot = await store.getSnapshot('dev-1');
      await store.close();

      expect(snapshot?.metrics['params.pv1ChargeWatts']).toBe(2);
      expect(snapshot?.metrics['params.wattsOutSum']).toBe(0);
    } finally {
      vi.useRealTimers();
    }
  });

  it('filters current metrics when recent frames are carrying a flatlined stale cohort', async () => {
    const now = new Date('2026-05-24T12:00:00.000Z');
    vi.useFakeTimers();
    vi.setSystemTime(now);
    try {
      const client = makeClient();
      client.get.mockResolvedValue(
        JSON.stringify({
          device_id: 'dev-1',
          cursor_seq: 12,
          cursor_ts_unix_ms: now.getTime() - 10_000,
          metrics: {
            'params.pv1ChargeWatts': 46,
            'params.wattsInSum': 46,
            'params.remainTime': 5999,
            'params.f32ShowSoc': 77.5
          },
          metric_observed_at_unix_ms: {
            'params.pv1ChargeWatts': now.getTime() - 10_000,
            'params.wattsInSum': now.getTime() - 10_000,
            'params.remainTime': now.getTime() - 10_000,
            'params.f32ShowSoc': now.getTime() - 10_000
          },
          metric_changed_at_unix_ms: {
            'params.pv1ChargeWatts': now.getTime() - 45 * 60_000,
            'params.wattsInSum': now.getTime() - 45 * 60_000,
            'params.remainTime': now.getTime() - 45 * 60_000,
            'params.f32ShowSoc': now.getTime() - 10_000
          }
        })
      );
      createClientMock.mockReturnValue(client);

      const { ValkeySnapshotStore } = await import('../src/snapshot/valkeySnapshotStore.js');
      const store = new ValkeySnapshotStore({
        addrs: ['127.0.0.1:26380'],
        keyPrefix: 'pulse:projection'
      });

      const snapshot = await store.getSnapshot('dev-1');
      await store.close();

      expect(snapshot?.metrics).toEqual({
        'params.f32ShowSoc': 77.5
      });
    } finally {
      vi.useRealTimers();
    }
  });

  it('filters current metrics when EcoFlow idle state contradicts non-zero input', async () => {
    const now = new Date('2026-05-24T12:00:00.000Z');
    vi.useFakeTimers();
    vi.setSystemTime(now);
    try {
      const client = makeClient();
      client.get.mockResolvedValue(
        JSON.stringify({
          device_id: 'dev-1',
          cursor_seq: 14,
          cursor_ts_unix_ms: now.getTime() - 10_000,
          metrics: {
            'params.pv1ChargeWatts': 46,
            'params.wattsInSum': 46,
            'params.wattsOutSum': 0,
            'params.bmsInputWatts': 0,
            'params.bmsOutputWatts': 0,
            'params.remainTime': 5999,
            'params.dsgRemainTime': 5999,
            'params.chgRemainTime': 5999,
            'params.chgPauseFlag': 1,
            'params.chgDsgState': 2,
            'params.sysState': 2,
            'params.f32ShowSoc': 77.5
          },
          metric_observed_at_unix_ms: {
            'params.pv1ChargeWatts': now.getTime() - 10_000,
            'params.wattsInSum': now.getTime() - 10_000,
            'params.wattsOutSum': now.getTime() - 10_000,
            'params.bmsInputWatts': now.getTime() - 10_000,
            'params.bmsOutputWatts': now.getTime() - 10_000,
            'params.remainTime': now.getTime() - 10_000,
            'params.dsgRemainTime': now.getTime() - 10_000,
            'params.chgRemainTime': now.getTime() - 10_000,
            'params.chgPauseFlag': now.getTime() - 10_000,
            'params.chgDsgState': now.getTime() - 10_000,
            'params.sysState': now.getTime() - 10_000,
            'params.f32ShowSoc': now.getTime() - 10_000
          },
          metric_changed_at_unix_ms: {
            'params.pv1ChargeWatts': now.getTime() - 10_000,
            'params.wattsInSum': now.getTime() - 10_000,
            'params.wattsOutSum': now.getTime() - 10_000,
            'params.bmsInputWatts': now.getTime() - 10_000,
            'params.bmsOutputWatts': now.getTime() - 10_000,
            'params.remainTime': now.getTime() - 10_000,
            'params.dsgRemainTime': now.getTime() - 10_000,
            'params.chgRemainTime': now.getTime() - 10_000,
            'params.chgPauseFlag': now.getTime() - 10_000,
            'params.chgDsgState': now.getTime() - 10_000,
            'params.sysState': now.getTime() - 10_000,
            'params.f32ShowSoc': now.getTime() - 10_000
          }
        })
      );
      createClientMock.mockReturnValue(client);

      const { ValkeySnapshotStore } = await import('../src/snapshot/valkeySnapshotStore.js');
      const store = new ValkeySnapshotStore({
        addrs: ['127.0.0.1:26380'],
        keyPrefix: 'pulse:projection'
      });

      const snapshot = await store.getSnapshot('dev-1');
      await store.close();

      for (const key of [
        'params.pv1ChargeWatts',
        'params.wattsInSum',
        'params.wattsOutSum',
        'params.bmsInputWatts',
        'params.bmsOutputWatts',
        'params.remainTime',
        'params.dsgRemainTime',
        'params.chgRemainTime'
      ]) {
        expect(snapshot?.metrics[key]).toBeUndefined();
      }
      expect(snapshot?.metrics['params.f32ShowSoc']).toBe(77.5);
    } finally {
      vi.useRealTimers();
    }
  });

  it('keeps constant trickle current when a sibling current metric is still moving', async () => {
    const now = new Date('2026-05-24T12:00:00.000Z');
    vi.useFakeTimers();
    vi.setSystemTime(now);
    try {
      const client = makeClient();
      client.get.mockResolvedValue(
        JSON.stringify({
          device_id: 'dev-1',
          cursor_seq: 13,
          cursor_ts_unix_ms: now.getTime() - 20_000,
          metrics: {
            'params.pv1ChargeWatts': 2,
            'params.wattsInSum': 2,
            'params.pv1InVol': 18.2,
            'params.f32ShowSoc': 44
          },
          metric_observed_at_unix_ms: {
            'params.pv1ChargeWatts': now.getTime() - 20_000,
            'params.wattsInSum': now.getTime() - 20_000,
            'params.pv1InVol': now.getTime() - 20_000,
            'params.f32ShowSoc': now.getTime() - 20_000
          },
          metric_changed_at_unix_ms: {
            'params.pv1ChargeWatts': now.getTime() - 45 * 60_000,
            'params.wattsInSum': now.getTime() - 45 * 60_000,
            'params.pv1InVol': now.getTime() - 20_000,
            'params.f32ShowSoc': now.getTime() - 20_000
          }
        })
      );
      createClientMock.mockReturnValue(client);

      const { ValkeySnapshotStore } = await import('../src/snapshot/valkeySnapshotStore.js');
      const store = new ValkeySnapshotStore({
        addrs: ['127.0.0.1:26380'],
        keyPrefix: 'pulse:projection'
      });

      const snapshot = await store.getSnapshot('dev-1');
      await store.close();

      expect(snapshot?.metrics['params.pv1ChargeWatts']).toBe(2);
      expect(snapshot?.metrics['params.wattsInSum']).toBe(2);
    } finally {
      vi.useRealTimers();
    }
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
