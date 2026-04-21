import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  fetchDeviceHistory,
  fetchDeviceSolarHistory,
  fetchFleetSolarHistory,
  type SolarHistoryView
} from '@/features/history/api';
import { ApiError } from '@/shared/api/restClient';
import {
  buildSolarHistoryBounds,
  msUntilNextLocalDay,
  SOLAR_HISTORY_POINTS,
  historyRefreshIntervalMs
} from '@/features/history/solar';
import {
  buildPowerTrendBounds,
  buildPowerTrendView,
  emptyPowerTrendView,
  sumPowerTrendViews,
  type PowerTrendView
} from '@/features/history/powerTrend';

type HistoryQueryOptions = {
  token?: string;
  authKey?: string;
  enabled?: boolean;
  maxSolarWatts?: number;
  maxSolarWattsByDeviceId?: Record<string, number | undefined>;
};

function buildDayKey(): string {
  return buildSolarHistoryBounds().from.toISOString().slice(0, 10);
}

function useSolarHistoryDayKey(): string {
  const [dayKey, setDayKey] = useState(() => buildDayKey());

  useEffect(() => {
    const timer = setTimeout(() => {
      setDayKey(buildDayKey());
    }, msUntilNextLocalDay());

    return () => {
      clearTimeout(timer);
    };
  }, [dayKey]);

  return dayKey;
}

function isHistoryNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

function emptySolarHistoryView(): SolarHistoryView {
  return {
    todayWh: 0,
    yesterdayWh: 0,
    deltaPct: null,
    seriesWh: Array.from({ length: SOLAR_HISTORY_POINTS }, () => 0),
    yesterdaySeriesWh: Array.from({ length: SOLAR_HISTORY_POINTS }, () => 0)
  };
}

function buildPowerTrendKey(): string {
  return buildPowerTrendBounds().queryTo.toISOString().slice(0, 16);
}

export function useDeviceSolarHistory(
  deviceId: string | undefined,
  options: HistoryQueryOptions = {}
) {
  const { token, authKey = 'anonymous', enabled = true, maxSolarWatts } = options;
  const dayKey = useSolarHistoryDayKey();

  return useQuery<SolarHistoryView>({
    queryKey: ['device-solar-history', deviceId, dayKey, authKey, maxSolarWatts ?? null],
    enabled: enabled && Boolean(deviceId),
    queryFn: async () => {
      const { from, to, compareFrom, compareTo } = buildSolarHistoryBounds();
      try {
        return await fetchDeviceSolarHistory({
          deviceId: deviceId ?? '',
          fromIso: from.toISOString(),
          toIso: to.toISOString(),
          compareFromIso: compareFrom.toISOString(),
          compareToIso: compareTo.toISOString(),
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
    refetchInterval: deviceId ? historyRefreshIntervalMs(deviceId) : false,
    placeholderData: (previous) => previous
  });
}

export function useFleetSolarHistory(
  deviceIds: string[],
  options: HistoryQueryOptions = {}
) {
  const { token, authKey = 'anonymous', enabled = true, maxSolarWattsByDeviceId } = options;
  const sortedIds = useMemo(() => [...deviceIds].sort(), [deviceIds]);
  const dayKey = useSolarHistoryDayKey();
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
      const { from, to, compareFrom, compareTo } = buildSolarHistoryBounds();
      try {
        return await fetchFleetSolarHistory({
          deviceIds: sortedIds,
          fromIso: from.toISOString(),
          toIso: to.toISOString(),
          compareFromIso: compareFrom.toISOString(),
          compareToIso: compareTo.toISOString(),
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
    refetchInterval: sortedIds.length > 0 ? historyRefreshIntervalMs(sortedIds.join(',')) : false,
    placeholderData: (previous) => previous
  });

  return query;
}

export function useDevicePowerTrendHistory(
  deviceId: string | undefined,
  options: HistoryQueryOptions = {}
) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  const windowKey = buildPowerTrendKey();

  return useQuery<PowerTrendView>({
    queryKey: ['device-power-trend-history', deviceId, windowKey, authKey],
    enabled: enabled && Boolean(deviceId),
    queryFn: async () => {
      const { queryFrom, queryTo } = buildPowerTrendBounds();
      try {
        const series = await fetchDeviceHistory({
          deviceId: deviceId ?? '',
          resolution: 'minute',
          fromIso: queryFrom.toISOString(),
          toIso: queryTo.toISOString(),
          token
        });
        return buildPowerTrendView(series, queryTo);
      } catch (error) {
        if (isHistoryNotFound(error)) {
          return emptyPowerTrendView();
        }
        throw error;
      }
    },
    staleTime: 60_000,
    gcTime: 10 * 60_000,
    placeholderData: (previous) => previous
  });
}

export function useFleetPowerTrendHistory(
  deviceIds: string[],
  options: HistoryQueryOptions = {}
) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  const sortedIds = useMemo(() => [...deviceIds].sort(), [deviceIds]);
  const windowKey = buildPowerTrendKey();

  const query = useQuery<PowerTrendView>({
    queryKey: ['fleet-power-trend-history', sortedIds, windowKey, authKey],
    enabled: enabled && sortedIds.length > 0,
    queryFn: async () => {
      const { queryFrom, queryTo } = buildPowerTrendBounds();
      try {
        const seriesList = await Promise.all(
          sortedIds.map(async (deviceId) => {
            try {
              return await fetchDeviceHistory({
                deviceId,
                resolution: 'minute',
                fromIso: queryFrom.toISOString(),
                toIso: queryTo.toISOString(),
                token
              });
            } catch (error) {
              if (isHistoryNotFound(error)) {
                return null;
              }
              throw error;
            }
          })
        );
        return sumPowerTrendViews(
          seriesList
            .filter((series): series is NonNullable<typeof series> => Boolean(series))
            .map((series) => buildPowerTrendView(series, queryTo))
        );
      } catch (error) {
        if (isHistoryNotFound(error)) {
          return emptyPowerTrendView();
        }
        throw error;
      }
    },
    staleTime: 60_000,
    gcTime: 10 * 60_000,
    placeholderData: (previous) => previous
  });

  return {
    ...query,
    data: query.data ?? emptyPowerTrendView()
  };
}
