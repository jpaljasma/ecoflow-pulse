import { useMemo, type ComponentProps } from 'react';
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
import { formatKWh, formatW, formatWhAndKWh } from '@/features/telemetry/format';
import { CachedImage } from '@/shared/ui/CachedImage';
import { env } from '@/shared/config/env';
import { useFleetSummaryViewModel, type FleetTypeIcon } from '@/features/devices/view-model';
import { isMutedMetric } from '@/shared/ui/uiMappings';
import { useTelemetryFleetTrend, useTelemetrySnapshotsByIds } from '@/features/telemetry/hooks';
import { useAuthSession } from '@/features/auth/hooks';
import { useCurrentUser } from '@/features/profile/hooks';
import { useFleetPowerTrendHistory, useFleetSolarHistory } from '@/features/history/hooks';
import { mergeTrendPrefill } from '@/features/history/powerTrend';
import { resolveSolarHistoryWindow } from '@/features/history/solar';
import { useWeatherForecast } from '@/features/weather/hooks';
import { resolveProfileWeatherState } from '@/features/weather/model';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { IconLabel } from '@/shared/ui/IconLabel';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';

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

function formatDeltaSummary(deltaPct: number | null | undefined): string {
  if (deltaPct === null || deltaPct === undefined || Number.isNaN(deltaPct)) {
    return 'Compared with yesterday';
  }
  if (deltaPct > 0) {
    return `Up ${deltaPct.toFixed(1)}% vs yesterday`;
  }
  if (deltaPct < 0) {
    return `Down ${Math.abs(deltaPct).toFixed(1)}% vs yesterday`;
  }
  return 'Matching yesterday';
}

function describeSolarHistoryError(error: unknown): string {
  if (error instanceof Error && error.message.trim()) {
    return error.message.trim();
  }
  return 'Solar history unavailable right now.';
}

function FleetOverviewTile({
  icon,
  label,
  value,
  detail,
  accent
}: {
  icon: ComponentProps<typeof MaterialCommunityIcons>['name'];
  label: string;
  value: string;
  detail: string;
  accent: string;
}) {
  const semantics = useThemeSemantics();

  return (
    <YStack
      flexGrow={1}
      flexBasis={160}
      minWidth={150}
      gap="$2"
      padding="$3"
      borderRadius="$4"
      borderWidth={1}
      style={{
        backgroundColor: semantics.tileBackground,
        borderColor: semantics.tileBorder
      }}
    >
      <XStack alignItems="center" justifyContent="space-between" gap="$2">
        <Text fontSize="$2" fontWeight="700" textTransform="uppercase" letterSpacing={0.6} style={{ color: semantics.subtleStrongText }}>
          {label}
        </Text>
        <YStack
          width={32}
          height={32}
          borderRadius={999}
          alignItems="center"
          justifyContent="center"
          style={{ backgroundColor: `${accent}1f` }}
        >
          <MaterialCommunityIcons name={icon} size={16} color={accent} />
        </YStack>
      </XStack>
      <Text fontSize="$6" fontWeight="800" letterSpacing={-0.3}>
        {value}
      </Text>
      <Text fontSize="$2" numberOfLines={2} style={{ color: semantics.subtleText }}>
        {detail}
      </Text>
    </YStack>
  );
}

function FleetTypeChip({
  label,
  uri,
  fallback,
  icon,
  active
}: FleetTypeIcon) {
  const semantics = useThemeSemantics();

  return (
    <XStack
      alignItems="center"
      gap="$2"
      paddingHorizontal="$3"
      paddingVertical="$2"
      borderRadius={999}
      borderWidth={1}
      style={{
        backgroundColor: semantics.tileBackground,
        borderColor: active ? semantics.energyCardBorder : semantics.tileBorder,
        opacity: active ? 1 : 0.72
      }}
    >
      <YStack
        width={26}
        height={26}
        borderRadius={999}
        overflow="hidden"
        alignItems="center"
        justifyContent="center"
        style={{ backgroundColor: semantics.mutedPanelBackground }}
      >
        {uri ? (
          <CachedImage uri={uri} style={{ width: 24, height: 24 }} contentFit="cover" />
        ) : fallback ? (
          <ExpoImage source={fallback} style={{ width: 24, height: 24 }} contentFit="cover" />
        ) : (
          <MaterialCommunityIcons name={icon} size={16} color={active ? semantics.actionText : semantics.subtleStrongText} />
        )}
      </YStack>
      <Text fontSize="$2" fontWeight="700" style={{ color: semantics.subtleStrongText }}>
        {label}
      </Text>
    </XStack>
  );
}

export function SummaryPanel({
  devices
}: {
  devices: DeviceSummary[];
}) {
  const semantics = useThemeSemantics();
  const { contentWidth } = useNavigationShellMetrics();
  const isTabletUp = contentWidth >= 768;
  const isCompact = contentWidth < 720;
  const useRemoteImage = Boolean(env.assetBaseUrl);
  const deviceIds = devices.map((device) => device.id);
  const byId = useTelemetrySnapshotsByIds(deviceIds);
  const fleetTrend = useTelemetryFleetTrend();
  const { authConfigured, authReady, authKey, sessionValid, token } = useAuthSession();
  const historyEnabled = authReady && (!authConfigured || sessionValid) && deviceIds.length > 0;
  const profileEnabled = authReady && (!authConfigured || sessionValid);
  const currentUserQuery = useCurrentUser({
    token,
    authKey,
    enabled: profileEnabled
  });
  const resolvedWeatherState = resolveProfileWeatherState(currentUserQuery.data?.user);
  const weatherForecastQuery = useWeatherForecast({
    token,
    authKey,
    locationKey: resolvedWeatherState.locationKey,
    enabled: profileEnabled && resolvedWeatherState.enabled
  });
  const solarHistoryWindow = useMemo(
    () => resolveSolarHistoryWindow(weatherForecastQuery.data?.forecast),
    [weatherForecastQuery.data?.forecast]
  );
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
    maxSolarWattsByDeviceId,
    window: solarHistoryWindow
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
  const fleetSolarHistoryView = fleetSolarHistory.data;
  const fleetSolarHistoryErrorText =
    fleetSolarHistory.error && !fleetSolarHistoryView
      ? describeSolarHistoryError(fleetSolarHistory.error)
      : undefined;
  const liveDeviceCount = useMemo(
    () => devices.filter((device) => !byId[device.id]?.inactive).length,
    [byId, devices]
  );

  const telemetryItems = [
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
          label={<IconLabel icon="power-plug-outline" label="AC" />}
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
          label={<IconLabel icon="current-dc" label="DC" />}
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

  const overviewTiles = [
    {
      key: 'soc',
      icon: 'battery-heart-variant',
      label: 'Battery',
      value: formatPct(summary.avgSocPct),
      detail: summary.totalCapacityKWh !== null ? `${formatKWh(summary.totalCapacityKWh)} installed` : 'Capacity unavailable',
      accent: semantics.chartBatteryCharge
    },
    {
      key: 'load',
      icon: 'home-lightning-bolt-outline',
      label: 'Load',
      value: formatW(summary.loadW),
      detail: `${liveDeviceCount || devices.length} live systems in view`,
      accent: semantics.chartLoad
    },
    {
      key: 'net',
      icon: 'transmission-tower',
      label: 'Net',
      value: formatW(displayNetW),
      detail: suppressFleetSolar ? 'Solar idle, showing site balance' : 'PV against active site demand',
      accent: semantics.chartAc
    },
    {
      key: 'pv',
      icon: 'white-balance-sunny',
      label: 'PV now',
      value: formatW(displayPvW),
      detail: 'Instant fleet solar input',
      accent: semantics.chartSolar
    }
  ] as const;

  return (
    <YStack gap="$3">
      <Card
        gap="$4"
        padding={isTabletUp ? '$5' : '$4'}
        style={{
          backgroundColor: semantics.energyCardBackground,
          borderColor: semantics.energyCardBorder
        }}
      >
        <XStack alignItems="flex-start" justifyContent="space-between" gap="$4" flexWrap="wrap">
          <YStack gap="$3" flex={1} minWidth={280}>
            <Text fontSize="$2" fontWeight="700" textTransform="uppercase" letterSpacing={0.8} style={{ color: semantics.solarBadgeTitle }}>
              Pulse Fleet
            </Text>
            <YStack gap="$2">
              <Text fontSize="$4" fontWeight="700" style={{ color: semantics.subtleStrongText }}>
                Solar generation today
              </Text>
              <Text
                fontWeight="800"
                letterSpacing={-1.1}
                style={{ fontSize: isTabletUp ? 62 : 48, lineHeight: isTabletUp ? 66 : 52 }}
              >
                {formatWhAndKWh(fleetSolarHistoryView?.todayWh)}
              </Text>
              <Text fontSize="$3" style={{ color: semantics.subtleStrongText }}>
                {fleetSolarHistoryErrorText
                  ? fleetSolarHistoryErrorText
                  : `${formatDeltaSummary(fleetSolarHistoryView?.deltaPct)} · ${devices.length} devices in view`}
              </Text>
            </YStack>
            <XStack gap="$2" flexWrap="wrap">
              {uniqueTypes.map((item) => (
                <FleetTypeChip
                  key={item.key}
                  label={item.label}
                  uri={item.uri}
                  fallback={item.fallback}
                  icon={item.icon}
                  active={item.active}
                />
              ))}
            </XStack>
          </YStack>

          <YStack gap="$3" flex={1} minWidth={280} maxWidth={isTabletUp ? 440 : undefined}>
            <XStack gap="$3" flexWrap="wrap">
              {overviewTiles.map((tile) => (
                <FleetOverviewTile
                  key={tile.key}
                  icon={tile.icon}
                  label={tile.label}
                  value={tile.value}
                  detail={tile.detail}
                  accent={tile.accent}
                />
              ))}
            </XStack>

            <Button
              size="$4"
              borderRadius={999}
              borderWidth={1}
              paddingHorizontal="$4"
              minHeight={42}
              alignSelf="flex-start"
              style={{
                backgroundColor: semantics.actionBackground,
                borderColor: semantics.actionBorder
              }}
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
              <XStack alignItems="center" gap="$2">
                <MaterialCommunityIcons name="lightning-bolt-outline" size={18} color={semantics.actionText} />
                <Text style={{ color: semantics.actionText }} fontWeight="700">
                  Open Energy Dashboard
                </Text>
              </XStack>
            </Button>
          </YStack>
        </XStack>
      </Card>

      {isTabletUp ? (
        <XStack gap="$3" alignItems="stretch" flexWrap="nowrap">
          <YStack flex={1.2} minWidth={0}>
            <ChartSection title={solarHistoryWindow.title} subtitle="Today against yesterday">
              {fleetSolarHistoryErrorText ? (
                <Text style={{ color: semantics.subtleText }}>{fleetSolarHistoryErrorText}</Text>
              ) : (
                <SolarGeneratedChart
                  valuesWh={fleetSolarHistoryView?.seriesWh}
                  yesterdayValuesWh={fleetSolarHistoryView?.yesterdaySeriesWh}
                  todayWh={fleetSolarHistoryView?.todayWh}
                  yesterdayWh={fleetSolarHistoryView?.yesterdayWh}
                  yesterdayRunningWh={fleetSolarHistoryView?.yesterdayRunningWh}
                  deltaPct={fleetSolarHistoryView?.deltaPct}
                  points={solarHistoryWindow.points}
                  startMinutes={solarHistoryWindow.startMinutes}
                  endMinutes={solarHistoryWindow.endMinutes}
                />
              )}
            </ChartSection>
          </YStack>
          <YStack flex={0.95} minWidth={0} gap="$3">
            <ChartSection title="Live Power Profile" subtitle="Fleet load against supply">
              <PowerTrendChart
                solar={displayFleetTrendPv}
                ac={displayFleetTrend.ac}
                dc={displayFleetTrend.dc}
                load={displayFleetTrend.load}
                battery={displayFleetTrend.load.map(() => 0)}
                points={SUMMARY_TREND_POINTS}
              />
            </ChartSection>
            <Card gap="$3" padding="$4" backgroundColor="$backgroundElevated">
              <Text fontSize="$5" fontWeight="700">
                Current telemetry
              </Text>
              <MetricsGrid items={telemetryItems} columns={2} />
            </Card>
          </YStack>
        </XStack>
      ) : (
        <YStack gap="$3">
          <ChartSection title={solarHistoryWindow.title} subtitle="Today against yesterday">
            {fleetSolarHistoryErrorText ? (
              <Text style={{ color: semantics.subtleText }}>{fleetSolarHistoryErrorText}</Text>
            ) : (
              <SolarGeneratedChart
                valuesWh={fleetSolarHistoryView?.seriesWh}
                yesterdayValuesWh={fleetSolarHistoryView?.yesterdaySeriesWh}
                todayWh={fleetSolarHistoryView?.todayWh}
                yesterdayWh={fleetSolarHistoryView?.yesterdayWh}
                yesterdayRunningWh={fleetSolarHistoryView?.yesterdayRunningWh}
                deltaPct={fleetSolarHistoryView?.deltaPct}
                points={solarHistoryWindow.points}
                startMinutes={solarHistoryWindow.startMinutes}
                endMinutes={solarHistoryWindow.endMinutes}
              />
            )}
          </ChartSection>
          <ChartSection title="Live Power Profile" subtitle="Fleet load against supply">
            <PowerTrendChart
              solar={displayFleetTrendPv}
              ac={displayFleetTrend.ac}
              dc={displayFleetTrend.dc}
              load={displayFleetTrend.load}
              battery={displayFleetTrend.load.map(() => 0)}
              points={SUMMARY_TREND_POINTS}
            />
          </ChartSection>
          <Card gap="$3" padding="$4" backgroundColor="$backgroundElevated">
            <Text fontSize="$5" fontWeight="700">
              Current telemetry
            </Text>
            <MetricsGrid items={telemetryItems} columns={isCompact ? 2 : 3} />
          </Card>
        </YStack>
      )}
    </YStack>
  );
}
