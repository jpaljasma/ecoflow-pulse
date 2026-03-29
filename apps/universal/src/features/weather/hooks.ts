import { useQuery } from '@tanstack/react-query';
import {
  fetchSolarOutlook,
  fetchWeatherForecast,
  fetchWeatherYesterdayVerification
} from '@/features/weather/api';
import { buildWeatherQueryKey } from '@/features/weather/queryKeys';
import type {
  SolarOutlookResponse,
  SolarOutlookScope,
  WeatherForecastResponse,
  WeatherYesterdayVerificationResponse
} from '@/features/weather/api';

type WeatherQueryOptions = {
  token?: string;
  authKey?: string;
  locationKey?: string;
  enabled?: boolean;
  verificationEnabled?: boolean;
  scope?: 'all' | 'device';
  deviceId?: string;
};

const SOLAR_OUTLOOK_STALE_MS = 5 * 60_000;

export function useWeatherForecast(options: WeatherQueryOptions = {}) {
  const { token, authKey = 'anonymous', locationKey = 'none', enabled = true } = options;
  return useQuery<WeatherForecastResponse>({
    queryKey: [...buildWeatherQueryKey(authKey, locationKey), 'forecast'],
    queryFn: () => fetchWeatherForecast(token),
    enabled,
    staleTime: 15 * 60_000,
    gcTime: 30 * 60_000,
    placeholderData: (previous) => previous
  });
}

export function useWeatherYesterdayVerification(options: WeatherQueryOptions = {}) {
  const { token, authKey = 'anonymous', locationKey = 'none', enabled = true } = options;
  return useQuery<WeatherYesterdayVerificationResponse>({
    queryKey: [...buildWeatherQueryKey(authKey, locationKey), 'yesterday'],
    queryFn: () => fetchWeatherYesterdayVerification(token),
    enabled,
    staleTime: 30 * 60_000,
    gcTime: 30 * 60_000,
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
    staleTime: SOLAR_OUTLOOK_STALE_MS,
    gcTime: 30 * 60_000,
    placeholderData: (previous) => previous
  });
}

export function useProfileWeather(options: WeatherQueryOptions = {}) {
  const {
    token,
    authKey = 'anonymous',
    locationKey = 'none',
    enabled = true,
    verificationEnabled = false,
    scope = 'all',
    deviceId
  } = options;
  const forecastQuery = useWeatherForecast({
    token,
    authKey,
    locationKey,
    enabled
  });
  const verificationQuery = useWeatherYesterdayVerification({
    token,
    authKey,
    locationKey,
    enabled: enabled && verificationEnabled
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
    verificationQuery,
    solarOutlookQuery,
    solarOutlook: solarOutlookQuery.data?.outlook
  };
}
