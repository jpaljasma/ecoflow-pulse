import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { DeviceSummary } from '@/features/devices/api';
import { useAuthSession } from '@/features/auth/hooks';
import { fetchDeviceHistory } from '@/features/history/api';
import { useFleetSolarHistory } from '@/features/history/hooks';
import { EnergyImpactCard } from '@/features/energy-impact/EnergyImpactCard';
import { buildEnergyRouteParams } from '@/features/energy/model';
import {
  buildPastTwelveMonthsBounds,
  ENERGY_IMPACT_HISTORY_GC_MS,
  ENERGY_IMPACT_HISTORY_STALE_MS,
  type EnergyImpactPeriod
} from '@/features/energy-impact/model';
import { ApiError } from '@/shared/api/restClient';

function getMaxSolarWatts(device: DeviceSummary): number | undefined {
  const total = device.details?.solarPorts?.reduce((sum, port) => sum + (port.maxWatts ?? 0), 0) ?? 0;
  return total > 0 ? total : undefined;
}

function sumSolarWh(points: Array<{ metrics: { solarGeneratedWh: number } }>): number {
  return points.reduce((total, point) => total + Math.max(0, point.metrics.solarGeneratedWh ?? 0), 0);
}

function isHistoryNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

export function FleetEnergyImpactCard({
  devices,
  variant = 'summary'
}: {
  devices: DeviceSummary[];
  variant?: 'detailed' | 'summary';
}) {
  const [period, setPeriod] = useState<EnergyImpactPeriod>('today');
  const { authConfigured, authReady, authKey, sessionValid, token } = useAuthSession();
  const deviceIds = useMemo(() => devices.map((device) => device.id), [devices]);
  const sortedIds = useMemo(() => [...deviceIds].sort(), [deviceIds]);
  const historyEnabled = authReady && (!authConfigured || sessionValid) && deviceIds.length > 0;
  const maxSolarWattsByDeviceId = useMemo(
    () =>
      Object.fromEntries(
        devices.map((device) => {
          return [device.id, getMaxSolarWatts(device)];
        })
      ),
    [devices]
  );
  const fleetSolarHistory = useFleetSolarHistory(deviceIds, {
    token,
    authKey,
    enabled: historyEnabled,
    maxSolarWattsByDeviceId
  });

  const pastTwelveMonthsQuery = useQuery<number>({
    queryKey: ['fleet-energy-impact-12m', sortedIds, authKey],
    enabled: historyEnabled && period === 'past12Months',
    queryFn: async () => {
      const { from, to } = buildPastTwelveMonthsBounds();
      const totals = await Promise.all(
        sortedIds.map(async (deviceId) => {
          try {
            const series = await fetchDeviceHistory({
              deviceId,
              resolution: 'day',
              fromIso: from.toISOString(),
              toIso: to.toISOString(),
              token
            });
            return sumSolarWh(series.points);
          } catch (error) {
            if (isHistoryNotFound(error)) {
              return 0;
            }
            throw error;
          }
        })
      );
      return totals.reduce((total, value) => total + value, 0);
    },
    staleTime: ENERGY_IMPACT_HISTORY_STALE_MS,
    gcTime: ENERGY_IMPACT_HISTORY_GC_MS,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false
  });

  const displaySolarWh =
    period === 'today'
      ? (fleetSolarHistory.data?.todayWh ?? 0)
      : pastTwelveMonthsQuery.data ?? fleetSolarHistory.data?.todayWh ?? 0;
  const displayPeriod =
    period === 'past12Months' && pastTwelveMonthsQuery.data === undefined ? 'today' : period;

  return (
    <EnergyImpactCard
      solarWh={displaySolarWh}
      period={period}
      displayPeriod={displayPeriod}
      onPeriodChange={setPeriod}
      isLoading={period === 'past12Months' && pastTwelveMonthsQuery.isFetching}
      errorText={period === 'past12Months' && pastTwelveMonthsQuery.error ? 'Past 12 months history unavailable.' : undefined}
      variant={variant}
      showPeriodControls={variant !== 'summary'}
      energyLinkParams={
        variant === 'summary'
          ? buildEnergyRouteParams({
              scope: 'all',
              preset: 'today',
              timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
              includeComparison: true,
              panel: 'impact'
            })
          : undefined
      }
    />
  );
}
