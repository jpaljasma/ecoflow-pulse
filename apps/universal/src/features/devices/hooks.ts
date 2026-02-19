import { useQuery } from '@tanstack/react-query';
import { fetchDevice, fetchDevices } from '@/features/devices/api';
import { env } from '@/shared/config/env';

const isMock = env.apiUrl.startsWith('mock://');

export function useDevices(token?: string) {
  return useQuery({
    queryKey: ['devices'],
    queryFn: () => fetchDevices(token),
    refetchInterval: isMock ? 1_000 : false,
    refetchIntervalInBackground: true,
    staleTime: 60_000,
    gcTime: 5 * 60_000
  });
}

export function useDevice(deviceId: string | undefined, token?: string) {
  return useQuery({
    queryKey: ['device', deviceId],
    queryFn: () => fetchDevice(deviceId ?? '', token),
    enabled: Boolean(deviceId),
    refetchInterval: isMock ? 1_000 : false,
    refetchIntervalInBackground: true,
    staleTime: 60_000,
    gcTime: 5 * 60_000
  });
}
