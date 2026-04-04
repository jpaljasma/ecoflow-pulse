import { describe, expect, it } from 'vitest';

import { getBatteryUpsellUrl, getMaxBatteryCount } from '@/shared/config/merchandising';

describe('merchandising config', () => {
  it('uses the Ultra X battery ceiling for DELTA Pro Ultra X', () => {
    expect(getMaxBatteryCount('DELTA Pro Ultra X')).toBe(10);
    expect(
      getBatteryUpsellUrl({
        model: 'DELTA Pro Ultra X',
        batteryCount: 4
      })
    ).toBe(
      'https://us.ecoflow.com/products/delta-pro-ultra-x-smart-extra-battery?inviteCode=ATH7F3EF1P'
    );
  });

  it('keeps the legacy DPU battery ceiling unchanged', () => {
    expect(getMaxBatteryCount('DELTA Pro Ultra')).toBe(5);
  });
});

