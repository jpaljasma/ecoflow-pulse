import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ApiError } from '@/shared/api/restClient';
import { useAuthSession } from '@/features/auth/hooks';
import { buildEnergyRouteParams } from '@/features/energy/model';
import { fetchDeviceHistory } from '@/features/history/api';
import { EnergyImpactCard } from '@/features/energy-impact/EnergyImpactCard';
import {
  buildPastTwelveMonthsBounds,
  ENERGY_IMPACT_HISTORY_GC_MS,
  ENERGY_IMPACT_HISTORY_STALE_MS,
  resolveEnergyImpactDisplayState,
  sumSolarGeneratedWh,
  type EnergyImpactPeriod
} from '@/features/energy-impact/model';

function isHistoryNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

export function DeviceEnergyImpactCard({
  deviceId,
  todaySolarWh,
  variant = 'summary',
  fill = false
}: {
  deviceId?: string;
  todaySolarWh?: number;
  variant?: 'detailed' | 'summary';
  fill?: boolean;
}) {
  const [period, setPeriod] = useState<EnergyImpactPeriod>('today');
  const { authConfigured, authReady, authKey, sessionValid, token } = useAuthSession();
  const enabled = authReady && (!authConfigured || sessionValid) && Boolean(deviceId);

  const pastTwelveMonthsQuery = useQuery<number>({
    queryKey: ['device-energy-impact-12m', deviceId, authKey],
    enabled: enabled && period === 'past12Months',
    queryFn: async () => {
      const { from, to } = buildPastTwelveMonthsBounds();
      try {
        const series = await fetchDeviceHistory({
          deviceId: deviceId ?? '',
          resolution: 'day',
          fromIso: from.toISOString(),
          toIso: to.toISOString(),
          token
        });
        return sumSolarGeneratedWh(series.points);
      } catch (error) {
        if (isHistoryNotFound(error)) {
          return 0;
        }
        throw error;
      }
    },
    staleTime: ENERGY_IMPACT_HISTORY_STALE_MS,
    gcTime: ENERGY_IMPACT_HISTORY_GC_MS,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false
  });

  const displayState = resolveEnergyImpactDisplayState({
    period,
    todaySolarWh,
    pastTwelveMonthsSolarWh: pastTwelveMonthsQuery.data,
    pastTwelveMonthsFetching: pastTwelveMonthsQuery.isFetching
  });

  return (
    <EnergyImpactCard
      solarWh={displayState.solarWh}
      period={period}
      displayPeriod={displayState.displayPeriod}
      onPeriodChange={setPeriod}
      isLoading={displayState.isLoading}
      errorText={period === 'past12Months' && pastTwelveMonthsQuery.error ? 'Past 12 months history unavailable.' : undefined}
      variant={variant}
      fill={fill}
      showPeriodControls={variant !== 'summary'}
      energyLinkParams={
        variant === 'summary'
          ? buildEnergyRouteParams({
              scope: deviceId ? 'device' : 'all',
              deviceId,
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
