import { useMemo } from 'react';
import { useWindowDimensions } from 'react-native';
import { Image as ExpoImage } from 'expo-image';
import { Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import { Card } from '@/shared/ui/Card';
import { ChartSection } from '@/shared/ui/ChartSection';
import { MetricsGrid, type MetricsGridItem } from '@/shared/ui/MetricsGrid';
import { PowerTrendChart } from '@/shared/ui/PowerTrendChart';
import { SolarGeneratedChart } from '@/shared/ui/SolarGeneratedChart';
import { Stat } from '@/shared/ui/Stat';
import { formatKWh, formatW } from '@/features/telemetry/format';
import { SolarTodayBadge } from '@/shared/ui/SolarTodayBadge';
import { CachedImage } from '@/shared/ui/CachedImage';
import { env } from '@/shared/config/env';
import { useFleetSummaryViewModel } from '@/features/devices/view-model';
import { isMutedMetric } from '@/shared/ui/uiMappings';
import { useTelemetryFleetTrend, useTelemetrySnapshotsByIds } from '@/features/telemetry/hooks';
import { useAuthSession } from '@/features/auth/hooks';
import { useFleetSolarHistory } from '@/features/history/hooks';

const SUMMARY_TREND_POINTS = 60;
const SOLAR_GENERATED_POINTS = 72;

function getMaxSolarWatts(device: DeviceSummary): number | undefined {
  const total = device.details?.solarPorts?.reduce((sum, port) => sum + (port.maxWatts ?? 0), 0) ?? 0;
  return total > 0 ? total : undefined;
}

function formatPct(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return '—';
  return `${value.toFixed(1)}%`;
}

export function SummaryPanel({
  devices
}: {
  devices: DeviceSummary[];
}) {
  const { width } = useWindowDimensions();
  const isTabletUp = width >= 768;
  const isCompact = width < 720;
  const useRemoteImage = Boolean(env.assetBaseUrl);
  const deviceIds = devices.map((device) => device.id);
  const byId = useTelemetrySnapshotsByIds(deviceIds);
  const fleetTrend = useTelemetryFleetTrend();
  const { authConfigured, authReady, authKey, sessionValid, token } = useAuthSession();
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

  const { summary, uniqueTypes } = useFleetSummaryViewModel({
    devices,
    byId,
    useRemoteImage
  });

  const metricItems = [
    {
      key: 'battery',
      content: (
        <Stat
          label="🔋 Battery"
          value={summary.totalCapacityKWh !== null ? formatKWh(summary.totalCapacityKWh) : '—'}
          compact={isCompact}
        />
      )
    },
    {
      key: 'soc',
      content: <Stat label="⏲️ SOC" value={formatPct(summary.avgSocPct)} compact={isCompact} />
    },
    {
      key: 'net',
      content: <Stat label="⚖️ Net" value={formatW(summary.netW)} compact={isCompact} />
    },
    {
      key: 'ac',
      content: (
        <Stat
          label="∿ AC"
          value={formatW(summary.acInW)}
          tone={isMutedMetric(summary.acInW) ? 'muted' : 'default'}
          compact={isCompact}
        />
      )
    },
    {
      key: 'dc',
      content: (
        <Stat
          label="⎓ DC"
          value={formatW(summary.dcW)}
          tone={isMutedMetric(summary.dcW) ? 'muted' : 'default'}
          compact={isCompact}
        />
      )
    },
    {
      key: 'pv',
      content: (
        <Stat
          label="☼ PV"
          value={formatW(summary.pvW)}
          tone={isMutedMetric(summary.pvW) ? 'muted' : 'default'}
          compact={isCompact}
        />
      )
    },
    {
      key: 'today',
      span: isCompact ? 3 : 2,
      content: (
        <SolarTodayBadge
          valueWh={fleetSolarHistory.data.todayWh}
          deltaPct={fleetSolarHistory.data.deltaPct}
          compact={isCompact}
        />
      )
    },
    {
      key: 'load',
      content: (
        <Stat
          label="⌂ Load"
          value={formatW(summary.loadW)}
          tone={isMutedMetric(summary.loadW) ? 'muted' : 'default'}
          compact={isCompact}
        />
      )
    }
  ] satisfies MetricsGridItem[];

  return (
    <Card>
      <YStack gap="$3">
        <Text fontSize="$5" fontWeight="700">
          Fleet Summary
        </Text>
        <XStack gap="$2" alignItems="center" flexWrap="wrap">
          {uniqueTypes.map((item) => (
            <YStack
              key={item.key}
              width={40}
              height={40}
              borderRadius="$2"
              overflow="hidden"
              backgroundColor="rgba(120,120,128,0.12)"
              alignItems="center"
              justifyContent="center"
              opacity={item.active ? 1 : 0.42}
            >
              {item.uri ? (
                <CachedImage uri={item.uri} style={{ width: 34, height: 34 }} contentFit="cover" />
              ) : item.fallback ? (
                <ExpoImage source={item.fallback} style={{ width: 34, height: 34 }} contentFit="cover" />
              ) : (
                <Text>{item.emoji}</Text>
              )}
            </YStack>
          ))}
        </XStack>
        <MetricsGrid items={metricItems} columns={isCompact ? 3 : 9} />
        <YStack height={1} backgroundColor="rgba(120,120,128,0.24)" />

        {isTabletUp ? (
          <XStack gap="$3" alignItems="stretch" flexWrap="nowrap">
            <YStack flexBasis="50%" minWidth="50%" maxWidth="50%">
              <ChartSection title="☼ Solar Generated (6am-6pm, 10m buckets)" subtitle="1m refresh">
                <SolarGeneratedChart
                  valuesWh={fleetSolarHistory.data.seriesWh}
                  points={SOLAR_GENERATED_POINTS}
                />
              </ChartSection>
            </YStack>
            <YStack flexBasis="50%" minWidth="50%" maxWidth="50%">
              <ChartSection title="Power Trends">
                <PowerTrendChart
                  solar={fleetTrend.pv}
                  ac={fleetTrend.ac}
                  dc={fleetTrend.dc}
                  load={fleetTrend.load}
                  points={SUMMARY_TREND_POINTS}
                />
              </ChartSection>
            </YStack>
          </XStack>
        ) : (
          <YStack gap="$3">
            <ChartSection title="☼ Solar Generated (6am-6pm, 10m buckets)" subtitle="1m refresh">
              <SolarGeneratedChart
                valuesWh={fleetSolarHistory.data.seriesWh}
                points={SOLAR_GENERATED_POINTS}
              />
            </ChartSection>
            <ChartSection title="Power Trends">
              <PowerTrendChart
                solar={fleetTrend.pv}
                ac={fleetTrend.ac}
                dc={fleetTrend.dc}
                load={fleetTrend.load}
                points={SUMMARY_TREND_POINTS}
              />
            </ChartSection>
          </YStack>
        )}
      </YStack>
    </Card>
  );
}
