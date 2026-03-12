import { useQuery } from '@tanstack/react-query';
import { fetchEnergyDashboard, fetchEnergyPvPortHistory, type EnergyPreset } from '@/features/energy/api';

export function useEnergyDashboard(
  {
    scope,
    deviceId,
    preset,
    timezone,
    includeComparison = true,
    gridPricePerKwh,
    currency,
    token
  }: {
    scope: 'device' | 'all';
    deviceId?: string;
    preset: EnergyPreset;
    timezone: string;
    includeComparison?: boolean;
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
        gridPricePerKwh,
        currency,
        token
      }),
    enabled: enabled && (scope === 'all' || Boolean(deviceId)),
    staleTime: 30_000,
    gcTime: 10 * 60_000
  });
}

export function useEnergyPvPortHistory(
  {
    scope,
    deviceId,
    preset,
    timezone,
    token
  }: {
    scope: 'device' | 'all';
    deviceId?: string;
    preset: EnergyPreset;
    timezone: string;
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
    queryKey: ['energy-pv-history', authKey, scope, deviceId ?? null, preset, timezone],
    queryFn: () =>
      fetchEnergyPvPortHistory({
        scope,
        deviceId,
        preset,
        timezone,
        token
      }),
    enabled: enabled && (scope === 'all' || Boolean(deviceId)),
    staleTime: 30_000,
    gcTime: 10 * 60_000
  });
}
