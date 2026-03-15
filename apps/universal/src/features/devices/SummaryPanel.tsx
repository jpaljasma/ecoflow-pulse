import { useMemo } from 'react';
import { useWindowDimensions } from 'react-native';
import { router } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Image as ExpoImage } from 'expo-image';
import { Button, Text, XStack, YStack } from 'tamagui';
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
import { useFleetPowerTrendHistory, useFleetSolarHistory } from '@/features/history/hooks';
import { mergeTrendPrefill } from '@/features/history/powerTrend';
import { SOLAR_HISTORY_CHART_TITLE, SOLAR_HISTORY_POINTS } from '@/features/history/solar';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { IconLabel } from '@/shared/ui/IconLabel';

const SUMMARY_TREND_POINTS = 60;

function getMaxSolarWatts(device: DeviceSummary): number | undefined {
  const total = device.details?.solarPorts?.reduce((sum, port) => sum + (port.maxWatts ?? 0), 0) ?? 0;
  return total > 0 ? total : undefined;
}

function formatPct(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return '—';
  return `${value.toFixed(1)}%`;
}

function currentSolarPortWatts(device: DeviceSummary): number | undefined {
  const ports = device.details?.solarPorts;
  if (!ports?.length) {
    return undefined;
  }
  return ports.reduce((sum, port) => sum + Math.max(0, port.watts ?? 0), 0);
}

function shouldSuppressFleetSolar(devices: DeviceSummary[]): boolean {
  if (!devices.length) {
    return false;
  }

  let hasSolarStateSignal = false;
  for (const device of devices) {
    const portWatts = currentSolarPortWatts(device);
    const solarChargingOn = device.details?.solarChargingOn;

    if (portWatts === undefined && solarChargingOn === undefined) {
      return false;
    }
    hasSolarStateSignal = true;

    if ((portWatts ?? 0) > 0 || solarChargingOn) {
      return false;
    }
  }
  return hasSolarStateSignal;
}

export function SummaryPanel({
  devices
}: {
  devices: DeviceSummary[];
}) {
  const semantics = useThemeSemantics();
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
  const fleetPowerTrendHistory = useFleetPowerTrendHistory(deviceIds, {
    token,
    authKey,
    enabled: historyEnabled
  });

  const { summary, uniqueTypes } = useFleetSummaryViewModel({
    devices,
    byId,
    useRemoteImage
  });
  const suppressFleetSolar = useMemo(() => shouldSuppressFleetSolar(devices), [devices]);
  const displayPvW = suppressFleetSolar ? 0 : summary.pvW;
  const displayNetW =
    suppressFleetSolar && typeof summary.netW === 'number' && typeof summary.pvW === 'number'
      ? summary.netW - summary.pvW
      : summary.netW;
  const displayFleetTrend = useMemo(
    () => ({
      load: mergeTrendPrefill(
        fleetPowerTrendHistory.data.load,
        fleetTrend.load,
        fleetTrend.filledPoints
      ),
      pv: mergeTrendPrefill(
        fleetPowerTrendHistory.data.solar,
        fleetTrend.pv,
        fleetTrend.filledPoints
      ),
      ac: mergeTrendPrefill(
        fleetPowerTrendHistory.data.ac,
        fleetTrend.ac,
        fleetTrend.filledPoints
      ),
      dc: mergeTrendPrefill(
        fleetPowerTrendHistory.data.dc,
        fleetTrend.dc,
        fleetTrend.filledPoints
      )
    }),
    [fleetPowerTrendHistory.data, fleetTrend]
  );
  const displayFleetTrendPv = useMemo(
    () => (suppressFleetSolar ? displayFleetTrend.pv.map(() => 0) : displayFleetTrend.pv),
    [displayFleetTrend.pv, suppressFleetSolar]
  );

  const metricItems = [
    {
      key: 'battery',
      content: (
        <Stat
          label={<IconLabel icon="battery-high" label="Battery" />}
          value={summary.totalCapacityKWh !== null ? formatKWh(summary.totalCapacityKWh) : '—'}
          compact={isCompact}
        />
      )
    },
    {
      key: 'soc',
      content: <Stat label={<IconLabel icon="gauge" label="SOC" />} value={formatPct(summary.avgSocPct)} compact={isCompact} />
    },
    {
      key: 'net',
      content: <Stat label={<IconLabel icon="scale-balance" label="Net" />} value={formatW(displayNetW)} compact={isCompact} />
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
          label={<IconLabel icon="white-balance-sunny" label="PV" />}
          value={formatW(displayPvW)}
          tone={isMutedMetric(displayPvW) ? 'muted' : 'default'}
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
          previousWh={fleetSolarHistory.data.yesterdayWh}
          deltaPct={fleetSolarHistory.data.deltaPct}
          compact={isCompact}
        />
      )
    },
    {
      key: 'load',
      content: (
        <Stat
          label={<IconLabel icon="home-outline" label="Load" />}
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
        <XStack alignItems="center" justifyContent="space-between" gap="$3" flexWrap="wrap">
          <Text fontSize="$5" fontWeight="700">
            Fleet Summary
          </Text>
          <Button
            size="$2"
            borderRadius={999}
            borderWidth={1}
            paddingHorizontal="$3"
            minHeight={32}
            backgroundColor="rgba(10,132,255,0.08)"
            borderColor="rgba(10,132,255,0.18)"
            onPress={() =>
              router.push({
                pathname: '/(tabs)/energy',
                params: {
                  device: 'all',
                  preset: 'today',
                  compare: '1'
                }
              })
            }
          >
            <XStack alignItems="center" gap="$1">
              <MaterialCommunityIcons name="lightning-bolt-outline" size={16} color={semantics.actionText} />
              <Text style={{ color: semantics.actionText }} fontWeight="700">Energy</Text>
            </XStack>
          </Button>
        </XStack>
        <XStack gap="$2" alignItems="center" flexWrap="wrap">
          {uniqueTypes.map((item) => (
            <YStack
              key={item.key}
              width={40}
              height={40}
              borderRadius="$2"
              overflow="hidden"
              style={{
                backgroundColor: semantics.mutedPanelBackground,
                borderColor: semantics.mutedPanelBorder
              }}
              borderWidth={1}
              alignItems="center"
              justifyContent="center"
              opacity={item.active ? 1 : 0.6}
            >
              {item.uri ? (
                <CachedImage uri={item.uri} style={{ width: 34, height: 34 }} contentFit="cover" />
              ) : item.fallback ? (
                <ExpoImage source={item.fallback} style={{ width: 34, height: 34 }} contentFit="cover" />
              ) : (
                <MaterialCommunityIcons name={item.icon} size={22} color="rgba(28, 43, 45, 0.7)" />
              )}
            </YStack>
          ))}
        </XStack>
        <MetricsGrid items={metricItems} columns={isCompact ? 3 : 9} />
        <YStack height={1} style={{ backgroundColor: semantics.sectionBorder }} />

        {isTabletUp ? (
          <XStack gap="$3" alignItems="stretch" flexWrap="nowrap">
            <YStack flexBasis="50%" minWidth="50%" maxWidth="50%">
              <ChartSection title={SOLAR_HISTORY_CHART_TITLE} subtitle="1m refresh">
                <SolarGeneratedChart
                  valuesWh={fleetSolarHistory.data.seriesWh}
                  yesterdayValuesWh={fleetSolarHistory.data.yesterdaySeriesWh}
                  todayWh={fleetSolarHistory.data.todayWh}
                  yesterdayWh={fleetSolarHistory.data.yesterdayWh}
                  deltaPct={fleetSolarHistory.data.deltaPct}
                  points={SOLAR_HISTORY_POINTS}
                />
              </ChartSection>
            </YStack>
            <YStack flexBasis="50%" minWidth="50%" maxWidth="50%">
              <ChartSection title="Power Trends">
                <PowerTrendChart
                  solar={displayFleetTrendPv}
                  ac={displayFleetTrend.ac}
                  dc={displayFleetTrend.dc}
                  load={displayFleetTrend.load}
                  battery={displayFleetTrend.load.map(() => 0)}
                  points={SUMMARY_TREND_POINTS}
                />
              </ChartSection>
            </YStack>
          </XStack>
        ) : (
          <YStack gap="$3">
            <ChartSection title={SOLAR_HISTORY_CHART_TITLE} subtitle="1m refresh">
              <SolarGeneratedChart
                valuesWh={fleetSolarHistory.data.seriesWh}
                yesterdayValuesWh={fleetSolarHistory.data.yesterdaySeriesWh}
                todayWh={fleetSolarHistory.data.todayWh}
                yesterdayWh={fleetSolarHistory.data.yesterdayWh}
                deltaPct={fleetSolarHistory.data.deltaPct}
                points={SOLAR_HISTORY_POINTS}
              />
            </ChartSection>
            <ChartSection title="Power Trends">
              <PowerTrendChart
                solar={displayFleetTrendPv}
                ac={displayFleetTrend.ac}
                dc={displayFleetTrend.dc}
                load={displayFleetTrend.load}
                battery={displayFleetTrend.load.map(() => 0)}
                points={SUMMARY_TREND_POINTS}
              />
            </ChartSection>
          </YStack>
        )}
      </YStack>
    </Card>
  );
}
