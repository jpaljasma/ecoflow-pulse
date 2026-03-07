import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { fetchDeviceSolarHistory, fetchFleetSolarHistory, type SolarHistoryView } from '@/features/history/api';
import { ApiError } from '@/shared/api/restClient';
import {
  buildTodayBounds,
  SOLAR_HISTORY_POINTS,
  historyRefreshIntervalMs
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
        return await fetchDeviceSolarHistory({
          deviceId: deviceId ?? '',
          fromIso: from.toISOString(),
          toIso: to.toISOString(),
          token
        });
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
  const maxSolarKey = useMemo(
    () =>
      sortedIds
        .map((deviceId) => [deviceId, maxSolarWattsByDeviceId?.[deviceId] ?? null] as const)
        .map(([deviceId, watts]) => `${deviceId}:${watts ?? 'null'}`)
        .join('|'),
    [maxSolarWattsByDeviceId, sortedIds]
  );

  const query = useQuery<SolarHistoryView>({
    queryKey: ['fleet-solar-history', sortedIds, dayKey, authKey, maxSolarKey],
    enabled: enabled && sortedIds.length > 0,
    queryFn: async () => {
      const { from, to } = buildTodayBounds();
      try {
        return await fetchFleetSolarHistory({
          deviceIds: sortedIds,
          fromIso: from.toISOString(),
          toIso: to.toISOString(),
          token
        });
      } catch (error) {
        if (isHistoryNotFound(error)) {
          return emptySolarHistoryView();
        }
        throw error;
      }
    },
    staleTime: 30_000,
    gcTime: 10 * 60_000,
    refetchInterval: sortedIds.length > 0 ? historyRefreshIntervalMs(sortedIds.join(',')) : false
  });

  return {
    ...query,
    data: query.data ?? emptySolarHistoryView()
  };
}
