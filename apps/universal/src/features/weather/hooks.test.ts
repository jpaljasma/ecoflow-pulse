import { beforeEach, describe, expect, it, vi } from 'vitest';

const reactQueryMock = vi.hoisted(() => ({
  useQuery: vi.fn()
}));

const weatherApiMock = vi.hoisted(() => ({
  fetchSolarOutlook: vi.fn(),
  fetchWeatherForecast: vi.fn(),
  fetchWeatherYesterdayVerification: vi.fn()
}));

vi.mock('@tanstack/react-query', () => reactQueryMock);
vi.mock('@/features/weather/api', () => weatherApiMock);

import {
  useSolarOutlook,
  useWeatherForecast,
  useWeatherYesterdayVerification
} from '@/features/weather/hooks';
import { buildWeatherQueryKey } from '@/features/weather/queryKeys';

const FOUR_HOURS_MS = 4 * 60 * 60_000;
const ONE_DAY_MS = 24 * 60 * 60_000;

beforeEach(() => {
  reactQueryMock.useQuery.mockReset();
  reactQueryMock.useQuery.mockReturnValue({
    data: undefined,
    isLoading: false,
    error: null
  });
  weatherApiMock.fetchSolarOutlook.mockReset();
  weatherApiMock.fetchWeatherForecast.mockReset();
  weatherApiMock.fetchWeatherYesterdayVerification.mockReset();
});

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

  it('keeps weather forecast queries hot for four hours without polling and preserves placeholder data', async () => {
    useWeatherForecast({
      token: 'token-123',
      authKey: 'auth-1',
      locationKey: '42.616:-77.401:America/New_York'
    });

    const config = reactQueryMock.useQuery.mock.calls[0]?.[0];
    expect(config.queryKey).toEqual([
      ...buildWeatherQueryKey('auth-1', '42.616:-77.401:America/New_York'),
      'forecast'
    ]);
    expect(config.enabled).toBe(true);
    expect(config.staleTime).toBe(FOUR_HOURS_MS);
    expect(config.gcTime).toBe(FOUR_HOURS_MS);
    expect(config.refetchInterval).toBeUndefined();
    expect(config.placeholderData({ marker: 'previous' })).toEqual({ marker: 'previous' });

    await config.queryFn();
    expect(weatherApiMock.fetchWeatherForecast).toHaveBeenCalledWith('token-123');
  });

  it('keeps yesterday verification lazy and fresh for a full day', async () => {
    useWeatherYesterdayVerification({
      token: 'token-123',
      authKey: 'auth-1',
      locationKey: '42.616:-77.401:America/New_York',
      enabled: false
    });

    const config = reactQueryMock.useQuery.mock.calls[0]?.[0];
    expect(config.queryKey).toEqual([
      ...buildWeatherQueryKey('auth-1', '42.616:-77.401:America/New_York'),
      'yesterday'
    ]);
    expect(config.enabled).toBe(false);
    expect(config.staleTime).toBe(ONE_DAY_MS);
    expect(config.gcTime).toBe(ONE_DAY_MS);
    expect(config.refetchInterval).toBeUndefined();
    expect(config.placeholderData({ marker: 'previous' })).toEqual({ marker: 'previous' });

    await config.queryFn();
    expect(weatherApiMock.fetchWeatherYesterdayVerification).toHaveBeenCalledWith('token-123');
  });

  it('keeps solar outlook freshness aligned to four hours and preserves device query-key invalidation', async () => {
    useSolarOutlook({
      token: 'token-123',
      authKey: 'auth-1',
      locationKey: '42.616:-77.401:America/New_York',
      scope: 'device',
      deviceId: '019c9f0e-4521-775d-873e-e80039f16d75'
    });

    const config = reactQueryMock.useQuery.mock.calls[0]?.[0];
    expect(config.queryKey).toEqual([
      ...buildWeatherQueryKey(
        'auth-1',
        '42.616:-77.401:America/New_York',
        'device',
        '019c9f0e-4521-775d-873e-e80039f16d75'
      ),
      'solar-outlook'
    ]);
    expect(config.enabled).toBe(true);
    expect(config.staleTime).toBe(FOUR_HOURS_MS);
    expect(config.gcTime).toBe(FOUR_HOURS_MS);
    expect(config.refetchInterval).toBeUndefined();
    expect(config.placeholderData({ marker: 'previous' })).toEqual({ marker: 'previous' });

    await config.queryFn();
    expect(weatherApiMock.fetchSolarOutlook).toHaveBeenCalledWith('token-123', {
      scope: 'device',
      deviceId: '019c9f0e-4521-775d-873e-e80039f16d75'
    });
  });

  it('keeps device-scoped solar outlook lazy until a device id exists', () => {
    useSolarOutlook({
      authKey: 'auth-1',
      locationKey: '42.616:-77.401:America/New_York',
      scope: 'device'
    });

    const config = reactQueryMock.useQuery.mock.calls[0]?.[0];
    expect(config.enabled).toBe(false);
    expect(config.queryKey).toEqual([
      ...buildWeatherQueryKey('auth-1', '42.616:-77.401:America/New_York', 'device', ''),
      'solar-outlook'
    ]);
  });
});
