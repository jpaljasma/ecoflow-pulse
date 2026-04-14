import { describe, expect, it } from 'vitest';

import { resolveNetPowerW } from '@/features/devices/net';

describe('resolveNetPowerW', () => {
  it('includes AC passthrough in net balance', () => {
    expect(
      resolveNetPowerW({
        acInW: 390,
        pvW: 0,
        loadW: 401
      })
    ).toBe(-11);
  });

  it('subtracts internal DC draw from available net power', () => {
    expect(
      resolveNetPowerW({
        acInW: 390,
        pvW: 0,
        loadW: 401,
        dcW: 11
      })
    ).toBe(-22);
  });

  it('falls back when load is unavailable', () => {
    expect(
      resolveNetPowerW({
        acInW: 200,
        pvW: 150,
        fallbackNetW: 25
      })
    ).toBe(25);
  });
});
