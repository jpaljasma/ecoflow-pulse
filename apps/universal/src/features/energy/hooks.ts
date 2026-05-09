import { useQuery } from '@tanstack/react-query';
import {
  fetchEnergyCalendar,
  fetchEnergyComparisonInsight,
  fetchEnergyDashboard,
  fetchEnergyPvPortHistory,
  type EnergyPreset
} from '@/features/energy/api';

export function buildEnergyCalendarQueryKey({
  authKey = 'anonymous',
  scope,
  deviceId,
  year,
  month,
  timezone,
  gridPricePerKwh,
  currency
}: {
  authKey?: string;
  scope: 'device' | 'all';
  deviceId?: string;
  year: number;
  month: number;
  timezone: string;
  gridPricePerKwh?: number;
  currency?: string;
}) {
  return [
    'energy-calendar',
    authKey,
    scope,
    deviceId ?? null,
    year,
    month,
    timezone,
    gridPricePerKwh ?? null,
    currency ?? null
  ] as const;
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
        timezone,
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
        timezone,
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
        timezone,
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
  return useQuery({
    queryKey: buildEnergyCalendarQueryKey({
      authKey,
      scope,
      deviceId,
      year,
      month,
      timezone,
      gridPricePerKwh,
      currency
    }),
    queryFn: () =>
      fetchEnergyCalendar({
        scope,
        deviceId,
        year,
        month,
        timezone,
        gridPricePerKwh,
        currency,
        token
      }),
    enabled: enabled && (scope === 'all' || Boolean(deviceId)),
    staleTime: 5 * 60_000,
    gcTime: 30 * 60_000,
    placeholderData: (previous) => previous
  });
}
