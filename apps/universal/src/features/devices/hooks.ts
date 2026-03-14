import { useQuery } from '@tanstack/react-query';
import { fetchDevice, fetchDevices } from '@/features/devices/api';

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
    staleTime: 60_000,
    gcTime: 5 * 60_000,
    placeholderData: (previous) => previous
  });
}

export function useDevice(deviceId: string | undefined, options: DeviceQueryOptions = {}) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  return useQuery({
    queryKey: ['device', deviceId, authKey],
    queryFn: () => fetchDevice(deviceId ?? '', token),
    enabled: enabled && Boolean(deviceId),
    staleTime: 60_000,
    gcTime: 5 * 60_000,
    placeholderData: (previous) => previous
  });
}
