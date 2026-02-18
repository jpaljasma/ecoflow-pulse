import { useQuery } from '@tanstack/react-query';
import { fetchDevice, fetchDevices } from '@/features/devices/api';

export function useDevices(token?: string) {
  return useQuery({
    queryKey: ['devices'],
    queryFn: () => fetchDevices(token),
    staleTime: 60_000,
    gcTime: 5 * 60_000
  });
}

export function useDevice(deviceId: string | undefined, token?: string) {
  return useQuery({
    queryKey: ['device', deviceId],
    queryFn: () => fetchDevice(deviceId ?? '', token),
    enabled: Boolean(deviceId),
    staleTime: 60_000,
    gcTime: 5 * 60_000
  });
}
