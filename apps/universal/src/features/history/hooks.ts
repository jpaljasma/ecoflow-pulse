import { useMemo } from 'react';
import { useQueries, useQuery } from '@tanstack/react-query';
import { fetchDeviceHistory } from '@/features/history/api';
import {
  buildSolarHistoryView,
  combineSolarHistoryViews,
  buildTodayBounds,
  historyRefreshIntervalMs,
  type SolarHistoryView
} from '@/features/history/solar';

type HistoryQueryOptions = {
  token?: string;
  authKey?: string;
  enabled?: boolean;
};

function buildDayKey(): string {
  return buildTodayBounds().from.toISOString().slice(0, 10);
}

export function useDeviceSolarHistory(
  deviceId: string | undefined,
  options: HistoryQueryOptions = {}
) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  const dayKey = buildDayKey();

  return useQuery<SolarHistoryView>({
    queryKey: ['device-solar-history', deviceId, dayKey, authKey],
    enabled: enabled && Boolean(deviceId),
    queryFn: async () => {
      const { from, to } = buildTodayBounds();
      const series = await fetchDeviceHistory({
        deviceId: deviceId ?? '',
        resolution: 'minute',
        fromIso: from.toISOString(),
        toIso: to.toISOString(),
        token
      });
      return buildSolarHistoryView(series.points);
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
  const { token, authKey = 'anonymous', enabled = true } = options;
  const sortedIds = useMemo(() => [...deviceIds].sort(), [deviceIds]);
  const dayKey = buildDayKey();

  const queries = useQueries({
    queries: sortedIds.map((deviceId) => {
      return {
        queryKey: ['device-solar-history', deviceId, dayKey, authKey],
        enabled: enabled && Boolean(deviceId),
        queryFn: async () => {
          const { from, to } = buildTodayBounds();
          const series = await fetchDeviceHistory({
            deviceId,
            resolution: 'minute',
            fromIso: from.toISOString(),
            toIso: to.toISOString(),
            token
          });
          return buildSolarHistoryView(series.points);
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
