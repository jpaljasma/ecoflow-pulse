import { describe, expect, it } from 'vitest';

import { createBffCache } from '../src/cache/bffCache.js';

describe('BFF response cache', () => {
  it('coalesces concurrent misses and serves later TTL hits', async () => {
    let now = 1_000;
    const cache = createBffCache({
      enabled: true,
      maxEntries: 100,
      now: () => now
    });
    let loads = 0;

    const loader = async () => {
      loads += 1;
      return { value: `forecast-${loads}` };
    };

    const [first, second] = await Promise.all([
      cache.getOrLoad('weather_forecast', 'same-location', 5_000, loader),
      cache.getOrLoad('weather_forecast', 'same-location', 5_000, loader)
    ]);

    expect(first).toEqual({ value: 'forecast-1' });
    expect(second).toEqual({ value: 'forecast-1' });
    expect(loads).toBe(1);

    const third = await cache.getOrLoad('weather_forecast', 'same-location', 5_000, loader);

    expect(third).toEqual({ value: 'forecast-1' });
    expect(loads).toBe(1);

    now += 5_001;
    const afterExpiry = await cache.getOrLoad('weather_forecast', 'same-location', 5_000, loader);

    expect(afterExpiry).toEqual({ value: 'forecast-2' });
    expect(loads).toBe(2);
  });

  it('does not leak caller mutations back into cached values', async () => {
    const cache = createBffCache({
      enabled: true,
      maxEntries: 100,
      now: () => 1_000
    });

    const first = await cache.getOrLoad('weather_forecast', 'same-location', 5_000, async () => ({
      nested: { value: 'original' }
    }));
    first.nested.value = 'mutated';

    const second = await cache.getOrLoad('weather_forecast', 'same-location', 5_000, async () => ({
      nested: { value: 'replacement' }
    }));

    expect(second).toEqual({ nested: { value: 'original' } });
  });

  it('bypasses storage when disabled or TTL is zero', async () => {
    const disabled = createBffCache({
      enabled: false,
      maxEntries: 100,
      now: () => 1_000
    });
    let disabledLoads = 0;

    await disabled.getOrLoad('weather_forecast', 'key', 5_000, async () => {
      disabledLoads += 1;
      return disabledLoads;
    });
    await disabled.getOrLoad('weather_forecast', 'key', 5_000, async () => {
      disabledLoads += 1;
      return disabledLoads;
    });

    expect(disabledLoads).toBe(2);

    const ttlZero = createBffCache({
      enabled: true,
      maxEntries: 100,
      now: () => 1_000
    });
    let ttlZeroLoads = 0;

    await ttlZero.getOrLoad('weather_forecast', 'key', 0, async () => {
      ttlZeroLoads += 1;
      return ttlZeroLoads;
    });
    await ttlZero.getOrLoad('weather_forecast', 'key', 0, async () => {
      ttlZeroLoads += 1;
      return ttlZeroLoads;
    });

    expect(ttlZeroLoads).toBe(2);
  });
});
