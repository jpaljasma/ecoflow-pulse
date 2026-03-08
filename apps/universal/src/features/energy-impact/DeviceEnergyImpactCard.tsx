import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ApiError } from '@/shared/api/restClient';
import { useAuthSession } from '@/features/auth/hooks';
import { fetchDeviceHistory } from '@/features/history/api';
import { EnergyImpactCard } from '@/features/energy-impact/EnergyImpactCard';
import {
  buildPastTwelveMonthsBounds,
  ENERGY_IMPACT_HISTORY_GC_MS,
  ENERGY_IMPACT_HISTORY_STALE_MS,
  type EnergyImpactPeriod
} from '@/features/energy-impact/model';

function sumSolarWh(points: Array<{ metrics: { solarGeneratedWh: number } }>): number {
  return points.reduce((total, point) => total + Math.max(0, point.metrics.solarGeneratedWh ?? 0), 0);
}

function isHistoryNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

export function DeviceEnergyImpactCard({
  deviceId,
  todaySolarWh
}: {
  deviceId?: string;
  todaySolarWh?: number;
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
        return sumSolarWh(series.points);
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

  const displaySolarWh =
    period === 'today'
      ? Math.max(0, todaySolarWh ?? 0)
      : pastTwelveMonthsQuery.data ?? Math.max(0, todaySolarWh ?? 0);

  return (
    <EnergyImpactCard
      solarWh={displaySolarWh}
      period={period}
      onPeriodChange={setPeriod}
      isLoading={period === 'past12Months' && pastTwelveMonthsQuery.isFetching}
      errorText={period === 'past12Months' && pastTwelveMonthsQuery.error ? 'Past 12 months history unavailable.' : undefined}
    />
  );
}
