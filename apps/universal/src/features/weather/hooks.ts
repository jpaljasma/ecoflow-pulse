import { useQuery } from '@tanstack/react-query';
import {
  fetchSolarOutlook,
  fetchWeatherForecast
} from '@/features/weather/api';
import { buildWeatherQueryKey } from '@/features/weather/queryKeys';
import type {
  SolarOutlookResponse,
  SolarOutlookScope,
  WeatherForecastResponse
} from '@/features/weather/api';

type WeatherQueryOptions = {
  token?: string;
  authKey?: string;
  locationKey?: string;
  enabled?: boolean;
  scope?: 'all' | 'device';
  deviceId?: string;
};

const FOUR_HOURS_MS = 4 * 60 * 60_000;

export function useWeatherForecast(options: WeatherQueryOptions = {}) {
  const { token, authKey = 'anonymous', locationKey = 'none', enabled = true } = options;
  return useQuery<WeatherForecastResponse>({
    queryKey: [...buildWeatherQueryKey(authKey, locationKey), 'forecast'],
    queryFn: () => fetchWeatherForecast(token),
    enabled,
    staleTime: FOUR_HOURS_MS,
    gcTime: FOUR_HOURS_MS,
    placeholderData: (previous) => previous
  });
}

export function useSolarOutlook(options: WeatherQueryOptions & SolarOutlookScope = {}) {
  const {
    token,
    authKey = 'anonymous',
    locationKey = 'none',
    enabled = true,
    scope = 'all',
    deviceId
  } = options;
  return useQuery<SolarOutlookResponse>({
    queryKey: [...buildWeatherQueryKey(authKey, locationKey, scope, deviceId ?? ''), 'solar-outlook'],
    queryFn: () => fetchSolarOutlook(token, { scope, deviceId }),
    enabled: enabled && (scope === 'all' || Boolean(deviceId)),
    staleTime: FOUR_HOURS_MS,
    gcTime: FOUR_HOURS_MS,
    placeholderData: (previous) => previous
  });
}

export function useProfileWeather(options: WeatherQueryOptions = {}) {
  const {
    token,
    authKey = 'anonymous',
    locationKey = 'none',
    enabled = true,
    scope = 'all',
    deviceId
  } = options;
  const forecastQuery = useWeatherForecast({
    token,
    authKey,
    locationKey,
    enabled
  });
  const solarOutlookQuery = useSolarOutlook({
    token,
    authKey,
    locationKey,
    enabled,
    scope,
    deviceId
  });

  return {
    forecastQuery,
    solarOutlookQuery,
    solarOutlook: solarOutlookQuery.data?.outlook
  };
}
