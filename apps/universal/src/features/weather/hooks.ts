import { useQuery } from '@tanstack/react-query';
import {
  fetchSolarOutlook,
  fetchWeatherForecast,
  fetchWeatherYesterdayVerification
} from '@/features/weather/api';
import { buildWeatherQueryKey } from '@/features/weather/queryKeys';
import type {
  SolarOutlookResponse,
  WeatherForecastResponse,
  WeatherYesterdayVerificationResponse
} from '@/features/weather/api';

type WeatherQueryOptions = {
  token?: string;
  authKey?: string;
  locationKey?: string;
  enabled?: boolean;
  verificationEnabled?: boolean;
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

export function useProfileWeather(options: WeatherQueryOptions = {}) {
  const {
    token,
    authKey = 'anonymous',
    locationKey = 'none',
    enabled = true,
    verificationEnabled = false
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
  const solarOutlookQuery = useQuery<SolarOutlookResponse>({
    queryKey: [...buildWeatherQueryKey(authKey, locationKey), 'solar-outlook'],
    queryFn: () => fetchSolarOutlook(token),
    enabled,
    staleTime: SOLAR_OUTLOOK_STALE_MS,
    gcTime: 30 * 60_000,
    placeholderData: (previous) => previous
  });

  return {
    forecastQuery,
    verificationQuery,
    solarOutlookQuery,
    solarOutlook: solarOutlookQuery.data?.outlook
  };
}
