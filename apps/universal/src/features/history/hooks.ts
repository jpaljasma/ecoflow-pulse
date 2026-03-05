import { useMemo } from 'react';
import { useQueries, useQuery } from '@tanstack/react-query';
import { fetchCompareDeviceHistory } from '@/features/history/api';
import { ApiError } from '@/shared/api/restClient';
import {
  buildCompareSolarHistoryView,
  combineSolarHistoryViews,
  buildTodayBounds,
  SOLAR_HISTORY_POINTS,
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

function isHistoryNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

function emptySolarHistoryView(): SolarHistoryView {
  return {
    todayWh: 0,
    yesterdayWh: 0,
    deltaPct: null,
    seriesWh: Array.from({ length: SOLAR_HISTORY_POINTS }, () => 0)
  };
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
      try {
        const series = await fetchCompareDeviceHistory({
          deviceId: deviceId ?? '',
          resolution: 'minute',
          fromIso: from.toISOString(),
          toIso: to.toISOString(),
          token
        });
        return buildCompareSolarHistoryView(series, { maxSolarWatts });
      } catch (error) {
        if (isHistoryNotFound(error)) {
          return emptySolarHistoryView();
        }
        throw error;
      }
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
          try {
            const series = await fetchCompareDeviceHistory({
              deviceId,
              resolution: 'minute',
              fromIso: from.toISOString(),
              toIso: to.toISOString(),
              token
            });
            return buildCompareSolarHistoryView(series, { maxSolarWatts });
          } catch (error) {
            if (isHistoryNotFound(error)) {
              return emptySolarHistoryView();
            }
            throw error;
          }
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
