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
  defaultSolarHistoryWindow,
  msUntilNextLocalDay,
  SOLAR_HISTORY_POINTS,
  historyRefreshIntervalMs,
  type SolarHistoryWindow
} from '@/features/history/solar';
import {
  buildPowerTrendBounds,
  buildPowerTrendView,
  emptyPowerTrendView,
  sumPowerTrendViews,
  type PowerTrendView
} from '@/features/history/powerTrend';
import { hasNonZeroSeriesValue, useStableChartData } from '@/features/history/stableChartData';

export const TODAY_SOLAR_HISTORY_STALE_TIME_MS = 2 * 60_000;
export const POWER_TREND_HISTORY_STALE_TIME_MS = 90_000;
export const CHART_HISTORY_GC_TIME_MS = 45 * 60_000;

type HistoryQueryOptions = {
  token?: string;
  authKey?: string;
  enabled?: boolean;
  maxSolarWatts?: number;
  maxSolarWattsByDeviceId?: Record<string, number | undefined>;
  window?: SolarHistoryWindow;
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

function emptySolarHistoryView(points = SOLAR_HISTORY_POINTS): SolarHistoryView {
  return {
    todayWh: 0,
    yesterdayWh: 0,
    deltaPct: null,
    yesterdayRunningWh: 0,
    seriesWh: Array.from({ length: points }, () => 0),
    yesterdaySeriesWh: Array.from({ length: points }, () => 0)
  };
}

function buildPowerTrendKey(): string {
  return buildPowerTrendBounds().queryTo.toISOString().slice(0, 16);
}

type StableHistoryQuery<T> = {
  data: T | undefined;
  isFetching: boolean;
  isError: boolean;
  isPlaceholderData: boolean;
  isSuccess: boolean;
};

function buildStableKey(parts: readonly unknown[]): string {
  return JSON.stringify(parts);
}

function useStableHistoryQuery<T, Query extends StableHistoryQuery<T>>(
  query: Query,
  queryKey: readonly unknown[],
  isUsable: (data: T) => boolean
): Query & { data: T | undefined } {
  const stableData = useStableChartData({
    data: query.data,
    stableKey: buildStableKey(queryKey),
    isFetching: query.isFetching,
    isError: query.isError,
    isPlaceholderData: query.isPlaceholderData,
    isSuccess: query.isSuccess,
    isUsable
  });

  return {
    ...query,
    data: stableData
  };
}

function isUsableSolarHistoryView(view: SolarHistoryView): boolean {
  return (
    view.todayWh > 0 ||
    view.yesterdayWh > 0 ||
    view.yesterdayRunningWh > 0 ||
    hasNonZeroSeriesValue(view.seriesWh) ||
    hasNonZeroSeriesValue(view.yesterdaySeriesWh)
  );
}

function isUsablePowerTrendView(view: PowerTrendView): boolean {
  return (
    hasNonZeroSeriesValue(view.load) ||
    hasNonZeroSeriesValue(view.solar) ||
    hasNonZeroSeriesValue(view.ac) ||
    hasNonZeroSeriesValue(view.dc)
  );
}

export function useDeviceSolarHistory(
  deviceId: string | undefined,
  options: HistoryQueryOptions = {}
) {
  const { token, authKey = 'anonymous', enabled = true, maxSolarWatts } = options;
  const window = options.window ?? defaultSolarHistoryWindow();
  const dayKey = useSolarHistoryDayKey();

  const queryKey = [
    'device-solar-history',
    deviceId,
    dayKey,
    authKey,
    maxSolarWatts ?? null,
    window.startMinutes,
    window.endMinutes
  ] as const;
  const query = useQuery<SolarHistoryView>({
    queryKey,
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
          windowStartMinutes: window.startMinutes,
          windowEndMinutes: window.endMinutes,
          token
        });
      } catch (error) {
        if (isHistoryNotFound(error)) {
          return emptySolarHistoryView(window.points);
        }
        throw error;
      }
    },
    staleTime: TODAY_SOLAR_HISTORY_STALE_TIME_MS,
    gcTime: CHART_HISTORY_GC_TIME_MS,
    refetchInterval: deviceId ? historyRefreshIntervalMs(deviceId) : false,
    placeholderData: (previous) => previous
  });

  return useStableHistoryQuery(query, queryKey, isUsableSolarHistoryView);
}

export function useFleetSolarHistory(
  deviceIds: string[],
  options: HistoryQueryOptions = {}
) {
  const { token, authKey = 'anonymous', enabled = true, maxSolarWattsByDeviceId } = options;
  const window = options.window ?? defaultSolarHistoryWindow();
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

  const queryKey = [
    'fleet-solar-history',
    sortedIds,
    dayKey,
    authKey,
    maxSolarKey,
    window.startMinutes,
    window.endMinutes
  ] as const;
  const query = useQuery<SolarHistoryView>({
    queryKey,
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
          windowStartMinutes: window.startMinutes,
          windowEndMinutes: window.endMinutes,
          token
        });
      } catch (error) {
        if (isHistoryNotFound(error)) {
          return emptySolarHistoryView(window.points);
        }
        throw error;
      }
    },
    staleTime: TODAY_SOLAR_HISTORY_STALE_TIME_MS,
    gcTime: CHART_HISTORY_GC_TIME_MS,
    refetchInterval: sortedIds.length > 0 ? historyRefreshIntervalMs(sortedIds.join(',')) : false,
    placeholderData: (previous) => previous
  });

  return useStableHistoryQuery(query, queryKey, isUsableSolarHistoryView);
}

export function useDevicePowerTrendHistory(
  deviceId: string | undefined,
  options: HistoryQueryOptions = {}
) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  const windowKey = buildPowerTrendKey();

  const queryKey = ['device-power-trend-history', deviceId, windowKey, authKey] as const;
  const query = useQuery<PowerTrendView>({
    queryKey,
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
    staleTime: POWER_TREND_HISTORY_STALE_TIME_MS,
    gcTime: CHART_HISTORY_GC_TIME_MS,
    placeholderData: (previous) => previous
  });

  return useStableHistoryQuery(query, queryKey, isUsablePowerTrendView);
}

export function useFleetPowerTrendHistory(
  deviceIds: string[],
  options: HistoryQueryOptions = {}
) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  const sortedIds = useMemo(() => [...deviceIds].sort(), [deviceIds]);
  const windowKey = buildPowerTrendKey();

  const queryKey = ['fleet-power-trend-history', sortedIds, windowKey, authKey] as const;
  const query = useQuery<PowerTrendView>({
    queryKey,
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
    staleTime: POWER_TREND_HISTORY_STALE_TIME_MS,
    gcTime: CHART_HISTORY_GC_TIME_MS,
    placeholderData: (previous) => previous
  });

  const stableQuery = useStableHistoryQuery(query, queryKey, isUsablePowerTrendView);

  return {
    ...stableQuery,
    data: stableQuery.data ?? emptyPowerTrendView()
  };
}
