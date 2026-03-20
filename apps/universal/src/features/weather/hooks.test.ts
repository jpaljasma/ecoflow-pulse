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
      '42.616:-77.401:America/New_York'
    ]);
  });
});
