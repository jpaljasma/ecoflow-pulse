import { describe, expect, it } from 'vitest';

import { loadConfig } from '../src/config.js';

describe('loadConfig', () => {
  it('preserves sentinel endpoints when a master set is configured', () => {
    const config = loadConfig({
      VALKEY_ADDRS: '127.0.0.1:26379',
      VALKEY_SENTINEL_MASTER_SET: 'myprimary',
      VALKEY_SENTINEL_USERNAME: 'sentinel-user',
      VALKEY_SENTINEL_PASSWORD: 'sentinel-pass',
      VALKEY_USERNAME: 'data-user',
      VALKEY_PASSWORD: 'data-pass'
    });

    expect(config.valkey).toMatchObject({
      addrs: ['127.0.0.1:26379'],
      sentinelMasterSet: 'myprimary',
      sentinelUsername: 'sentinel-user',
      sentinelPassword: 'sentinel-pass',
      username: 'data-user',
      password: 'data-pass'
    });
  });

  it('keeps the local direct-node fallback only when sentinel is not configured', () => {
    const config = loadConfig({
      VALKEY_ADDRS: '127.0.0.1:6379'
    });

    expect(config.valkey.addrs).toEqual(['127.0.0.1:6380', '127.0.0.1:6379']);
    expect(config.valkey.sentinelMasterSet).toBeUndefined();
  });
});
