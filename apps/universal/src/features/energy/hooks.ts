import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  fetchEnergyCalendar,
  fetchEnergyComparisonInsight,
  fetchEnergyDashboard,
  fetchEnergyPvPortHistory,
  type EnergyPreset
} from '@/features/energy/api';
import { buildEnergyCalendarCachePolicy } from '@/features/energy/model';

export function buildEnergyCalendarQueryKey({
  authKey = 'anonymous',
  scope,
  deviceId,
  year,
  month,
  timezone,
  gridPricePerKwh,
  currency,
  liveDayKey = null
}: {
  authKey?: string;
  scope: 'device' | 'all';
  deviceId?: string;
  year: number;
  month: number;
  timezone: string;
  gridPricePerKwh?: number;
  currency?: string;
  liveDayKey?: string | null;
}) {
  return [
    'energy-calendar',
    authKey,
    scope,
    deviceId ?? null,
    year,
    month,
    timezone,
    liveDayKey,
    gridPricePerKwh ?? null,
    currency ?? null
  ] as const;
}

export function useEnergyCalendarCachePolicy({
  year,
  month,
  timezone
}: {
  year: number;
  month: number;
  timezone: string;
}) {
  const [now, setNow] = useState(() => new Date());
  const policy = useMemo(
    () =>
      buildEnergyCalendarCachePolicy({
        year,
        month,
        timezone,
        now
      }),
    [month, now, timezone, year]
  );

  useEffect(() => {
    if (policy.midnightRefreshMs === null) {
      return undefined;
    }
    const timeout = setTimeout(() => {
      setNow(new Date());
    }, Math.min(Math.max(policy.midnightRefreshMs + 1000, 1000), 2_147_483_647));
    return () => clearTimeout(timeout);
  }, [policy.liveDayKey, policy.midnightRefreshMs]);

  return policy;
}

export function useEnergyDashboard(
  {
    scope,
    deviceId,
    preset,
    timezone,
    includeComparison = true,
    date,
    gridPricePerKwh,
    currency,
    token
  }: {
    scope: 'device' | 'all';
    deviceId?: string;
    preset: EnergyPreset;
    timezone: string;
    includeComparison?: boolean;
    date?: string;
    gridPricePerKwh?: number;
    currency?: string;
    token?: string;
  },
  {
    authKey = 'anonymous',
    enabled = true
  }: {
    authKey?: string;
    enabled?: boolean;
  } = {}
) {
  return useQuery({
    queryKey: [
      'energy-dashboard',
      authKey,
      scope,
      deviceId ?? null,
      preset,
      timezone,
      includeComparison,
      date ?? null,
      gridPricePerKwh ?? null,
      currency ?? null
    ],
    queryFn: () =>
      fetchEnergyDashboard({
        scope,
        deviceId,
        preset,
        includeComparison,
        date,
        gridPricePerKwh,
        currency,
        token
    }),
    enabled: enabled && (scope === 'all' || Boolean(deviceId)),
    staleTime: 30_000,
    gcTime: 10 * 60_000,
    placeholderData: (previous) => previous
  });
}

export function useEnergyPvPortHistory(
  {
    scope,
    deviceId,
    preset,
    timezone,
    date,
    token
  }: {
    scope: 'device' | 'all';
    deviceId?: string;
    preset: EnergyPreset;
    timezone: string;
    date?: string;
    token?: string;
  },
  {
    authKey = 'anonymous',
    enabled = true
  }: {
    authKey?: string;
    enabled?: boolean;
  } = {}
) {
  return useQuery({
    queryKey: ['energy-pv-history', authKey, scope, deviceId ?? null, preset, timezone, date ?? null],
    queryFn: () =>
      fetchEnergyPvPortHistory({
        scope,
        deviceId,
        preset,
        date,
        token
    }),
    enabled: enabled && (scope === 'all' || Boolean(deviceId)),
    staleTime: 30_000,
    gcTime: 10 * 60_000,
    placeholderData: (previous) => previous
  });
}

export function useEnergyComparisonInsight(
  {
    scope,
    deviceId,
    preset,
    timezone,
    date,
    gridPricePerKwh,
    currency,
    token
  }: {
    scope: 'device' | 'all';
    deviceId?: string;
    preset: EnergyPreset;
    timezone: string;
    date?: string;
    gridPricePerKwh?: number;
    currency?: string;
    token?: string;
  },
  {
    authKey = 'anonymous',
    enabled = true
  }: {
    authKey?: string;
    enabled?: boolean;
  } = {}
) {
  return useQuery({
    queryKey: [
      'energy-comparison-insight',
      authKey,
      scope,
      deviceId ?? null,
      preset,
      timezone,
      date ?? null,
      gridPricePerKwh ?? null,
      currency ?? null
    ],
    queryFn: () =>
      fetchEnergyComparisonInsight({
        scope,
        deviceId,
        preset,
        date,
        gridPricePerKwh,
        currency,
        token
    }),
    enabled: enabled && (scope === 'all' || Boolean(deviceId)),
    staleTime: 60 * 60_000,
    gcTime: 2 * 60 * 60_000,
    placeholderData: (previous) => previous
  });
}

export function useEnergyCalendar(
  {
    scope,
    deviceId,
    year,
    month,
    timezone,
    gridPricePerKwh,
    currency,
    token
  }: {
    scope: 'device' | 'all';
    deviceId?: string;
    year: number;
    month: number;
    timezone: string;
    gridPricePerKwh?: number;
    currency?: string;
    token?: string;
  },
  {
    authKey = 'anonymous',
    enabled = true
  }: {
    authKey?: string;
    enabled?: boolean;
  } = {}
) {
  const cachePolicy = useEnergyCalendarCachePolicy({
    year,
    month,
    timezone
  });

  return useQuery({
    queryKey: buildEnergyCalendarQueryKey({
      authKey,
      scope,
      deviceId,
      year,
      month,
      timezone,
      liveDayKey: cachePolicy.liveDayKey,
      gridPricePerKwh,
      currency
    }),
    queryFn: () =>
      fetchEnergyCalendar({
        scope,
        deviceId,
        year,
        month,
        gridPricePerKwh,
        currency,
        token
      }),
    enabled: enabled && (scope === 'all' || Boolean(deviceId)),
    staleTime: cachePolicy.staleTime,
    gcTime: cachePolicy.gcTime,
    refetchInterval: cachePolicy.refetchInterval,
    placeholderData: (previous) => previous
  });
}
