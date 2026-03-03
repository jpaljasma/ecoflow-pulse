import { useMemo } from 'react';
import { useQueries, useQuery } from '@tanstack/react-query';
import { fetchCompareDeviceHistory } from '@/features/history/api';
import {
  buildCompareSolarHistoryView,
  combineSolarHistoryViews,
  buildTodayBounds,
  historyRefreshIntervalMs,
  type SolarHistoryView
} from '@/features/history/solar';

type HistoryQueryOptions = {
  token?: string;
  authKey?: string;
  enabled?: boolean;
  maxSolarWatts?: number;
  maxSolarWattsByDeviceId?: Record<string, number | undefined>;
};

function buildDayKey(): string {
  return buildTodayBounds().from.toISOString().slice(0, 10);
}

export function useDeviceSolarHistory(
  deviceId: string | undefined,
  options: HistoryQueryOptions = {}
) {
  const { token, authKey = 'anonymous', enabled = true, maxSolarWatts } = options;
  const dayKey = buildDayKey();

  return useQuery<SolarHistoryView>({
    queryKey: ['device-solar-history', deviceId, dayKey, authKey, maxSolarWatts ?? null],
    enabled: enabled && Boolean(deviceId),
    queryFn: async () => {
      const { from, to } = buildTodayBounds();
      const series = await fetchCompareDeviceHistory({
        deviceId: deviceId ?? '',
        resolution: 'minute',
        fromIso: from.toISOString(),
        toIso: to.toISOString(),
        token
      });
      return buildCompareSolarHistoryView(series, { maxSolarWatts });
    },
    staleTime: 30_000,
    gcTime: 10 * 60_000,
    refetchInterval: deviceId ? historyRefreshIntervalMs(deviceId) : false
  });
}

export function useFleetSolarHistory(
  deviceIds: string[],
  options: HistoryQueryOptions = {}
) {
  const { token, authKey = 'anonymous', enabled = true, maxSolarWattsByDeviceId } = options;
  const sortedIds = useMemo(() => [...deviceIds].sort(), [deviceIds]);
  const dayKey = buildDayKey();

  const queries = useQueries({
    queries: sortedIds.map((deviceId) => {
      const maxSolarWatts = maxSolarWattsByDeviceId?.[deviceId];
      return {
        queryKey: ['device-solar-history', deviceId, dayKey, authKey, maxSolarWatts ?? null],
        enabled: enabled && Boolean(deviceId),
        queryFn: async () => {
          const { from, to } = buildTodayBounds();
          const series = await fetchCompareDeviceHistory({
            deviceId,
            resolution: 'minute',
            fromIso: from.toISOString(),
            toIso: to.toISOString(),
            token
          });
          return buildCompareSolarHistoryView(series, { maxSolarWatts });
        },
        staleTime: 30_000,
        gcTime: 10 * 60_000,
        refetchInterval: historyRefreshIntervalMs(deviceId)
      };
    })
  });

  return useMemo(() => {
    const data = combineSolarHistoryViews(queries.map((query) => query.data));
    return {
      data,
      isLoading: queries.some((query) => query.isLoading),
      isFetching: queries.some((query) => query.isFetching),
      error: queries.find((query) => query.error)?.error
    };
  }, [queries]);
}
