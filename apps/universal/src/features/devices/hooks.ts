import { useQuery } from '@tanstack/react-query';
import { fetchDevice, fetchDevices } from '@/features/devices/api';
import { env } from '@/shared/config/env';

const isMock = env.apiUrl.startsWith('mock://');

type DeviceQueryOptions = {
  token?: string;
  authKey?: string;
  enabled?: boolean;
};

export function useDevices(options: DeviceQueryOptions = {}) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  return useQuery({
    queryKey: ['devices', authKey],
    queryFn: () => fetchDevices(token),
    enabled,
    refetchInterval: isMock ? 1_000 : false,
    refetchIntervalInBackground: true,
    staleTime: 60_000,
    gcTime: 5 * 60_000
  });
}

export function useDevice(deviceId: string | undefined, options: DeviceQueryOptions = {}) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  return useQuery({
    queryKey: ['device', deviceId, authKey],
    queryFn: () => fetchDevice(deviceId ?? '', token),
    enabled: enabled && Boolean(deviceId),
    refetchInterval: isMock ? 1_000 : false,
    refetchIntervalInBackground: true,
    staleTime: 60_000,
    gcTime: 5 * 60_000
  });
}
