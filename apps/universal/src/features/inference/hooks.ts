import { useQuery } from '@tanstack/react-query';

import { fetchDeviceInsights, type DeviceInsights } from '@/features/inference/api';

type DeviceInsightsQueryOptions = {
  token?: string;
  authKey?: string;
  enabled?: boolean;
};

export function useDeviceInsights(deviceId: string | undefined, options: DeviceInsightsQueryOptions = {}) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  return useQuery<DeviceInsights>({
    queryKey: ['device-insights', deviceId, authKey],
    queryFn: () => fetchDeviceInsights(deviceId ?? '', { token }),
    enabled: enabled && Boolean(deviceId),
    staleTime: 60_000,
    gcTime: 5 * 60_000
  });
}
