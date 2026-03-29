import { describe, expect, it, vi } from 'vitest';

vi.mock('@/features/weather/api', () => ({
  fetchSolarOutlook: vi.fn(),
  fetchWeatherForecast: vi.fn(),
  fetchWeatherYesterdayVerification: vi.fn()
}));

import { buildWeatherQueryKey } from '@/features/weather/queryKeys';

describe('weather hooks', () => {
  it('builds query keys that partition auth and location state', () => {
    expect(buildWeatherQueryKey('auth-1', '42.616:-77.401:America/New_York')).toEqual([
      'weather',
      'auth-1',
      '42.616:-77.401:America/New_York',
      'all',
      ''
    ]);
  });

  it('builds distinct query keys for device-scoped solar outlooks', () => {
    expect(
      buildWeatherQueryKey(
        'auth-1',
        '42.616:-77.401:America/New_York',
        'device',
        '019c9f0e-4521-775d-873e-e80039f16d75'
      )
    ).toEqual([
      'weather',
      'auth-1',
      '42.616:-77.401:America/New_York',
      'device',
      '019c9f0e-4521-775d-873e-e80039f16d75'
    ]);
  });
});
