import { useMemo, type ComponentProps } from 'react';
import { router } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Image as ExpoImage } from 'expo-image';
import { View } from 'react-native';
import { Button, Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import { Card } from '@/shared/ui/Card';
import { ChartSection } from '@/shared/ui/ChartSection';
import { PowerTrendChart } from '@/shared/ui/PowerTrendChart';
import { SolarGeneratedChart } from '@/shared/ui/SolarGeneratedChart';
import { formatKWh, formatW, formatWhAndKWh } from '@/features/telemetry/format';
import { CachedImage } from '@/shared/ui/CachedImage';
import { env } from '@/shared/config/env';
import { useFleetSummaryViewModel, type FleetTypeIcon } from '@/features/devices/view-model';
import { useTelemetryFleetTrend, useTelemetrySnapshotsByIds } from '@/features/telemetry/hooks';
import { useAuthSession } from '@/features/auth/hooks';
import { useCurrentUser } from '@/features/profile/hooks';
import { useFleetPowerTrendHistory, useFleetSolarHistory } from '@/features/history/hooks';
import { mergePowerTrendPrefill } from '@/features/history/powerTrend';
import { resolveSolarHistoryWindow } from '@/features/history/solar';
import { useWeatherForecast } from '@/features/weather/hooks';
import { resolveProfileWeatherState } from '@/features/weather/model';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';
import { useLazySectionLoad } from '@/shared/ui/useLazySectionLoad';
import { PulseHeroBackground } from '@/shared/ui/PulseHeroBackground';
import { SocBar } from '@/shared/ui/SocBar';
import {
  buildFleetDevicePreview,
  resolveFleetDevicePreviewLimit,
  type FleetDevicePreviewChargeState,
  type FleetDevicePreviewItem
} from '@/features/devices/summaryPreview';
import { resolveDeviceVisualAssets } from '@/features/devices/deviceVisuals';
import { EnergyImpactCard } from '@/features/energy-impact/EnergyImpactCard';
import { buildEnergyRouteParams } from '@/features/energy/model';

const SUMMARY_TREND_POINTS = 60;
const FLEET_TILE_BASIS = 142;
const FLEET_TILE_MIN_WIDTH = 132;
const FLEET_TILE_MIN_HEIGHT = 120;

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
      flexBasis={FLEET_TILE_BASIS}
      minWidth={FLEET_TILE_MIN_WIDTH}
      minHeight={FLEET_TILE_MIN_HEIGHT}
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
      <Text fontSize="$6" fontWeight="800" letterSpacing={0}>
        {value}
      </Text>
      <Text fontSize="$2" numberOfLines={2} style={{ color: semantics.subtleText }}>
        {detail}
      </Text>
    </YStack>
  );
}

function FleetDeviceChargeIcon({ state }: { state: FleetDevicePreviewChargeState }) {
  const semantics = useThemeSemantics();
  if (state === 'neutral') return null;
  const isCharging = state === 'charging';
  const color = isCharging ? semantics.chartBatteryCharge : semantics.chartLoad;

  return (
    <YStack
      testID={`fleet-device-preview-${state}-icon`}
      width={24}
      height={24}
      borderRadius={999}
      alignItems="center"
      justifyContent="center"
      style={{ backgroundColor: `${color}1f` }}
    >
      <MaterialCommunityIcons
        name={isCharging ? 'arrow-up-bold' : 'arrow-down-bold'}
        size={15}
        color={color}
      />
    </YStack>
  );
}

function FleetDeviceTile({
  item,
  useRemoteImage
}: {
  item: FleetDevicePreviewItem;
  useRemoteImage: boolean;
}) {
  const semantics = useThemeSemantics();
  const { contentWidth } = useNavigationShellMetrics();
  const isCompact = contentWidth < 640;
  const { match, imageUri, fallbackSource } = resolveDeviceVisualAssets(item.device, {
    useRemoteImage,
    imageContext: 'list'
  });
  const imageWellHeight = isCompact ? 60 : 66;
  const imageSize = isCompact ? 70 : 78;
  const imageStyle = {
    width: imageSize,
    height: imageSize,
    transform: [{ scale: isCompact ? 1.08 : 1.14 }]
  };

  return (
    <Card
      testID={`fleet-device-preview-${item.id}`}
      flexGrow={1}
      flexBasis={FLEET_TILE_BASIS}
      minWidth={FLEET_TILE_MIN_WIDTH}
      minHeight={FLEET_TILE_MIN_HEIGHT}
      gap="$3"
      padding="$3"
      justifyContent="space-between"
      pressStyle={{ scale: 0.995, opacity: 0.95 }}
      hoverStyle={{
        transform: [{ translateY: -2 }],
        shadowOpacity: 0.12,
        borderColor: '$accentColor'
      }}
      onPress={() => router.push(`/device/${item.id}`)}
      role="button"
      cursor="pointer"
      style={{
        backgroundColor: semantics.tileBackground,
        borderColor: semantics.tileBorder
      }}
    >
      <YStack
        height={imageWellHeight}
        alignItems="center"
        justifyContent="center"
        overflow="hidden"
        style={{ backgroundColor: semantics.mutedPanelBackground }}
        borderRadius={12}
      >
        <YStack
          width={imageSize}
          height={imageSize}
          borderRadius={12}
          overflow="hidden"
          alignItems="center"
          justifyContent="center"
        >
          {imageUri ? (
            <CachedImage uri={imageUri} style={imageStyle} contentFit="contain" />
          ) : fallbackSource ? (
            <ExpoImage source={fallbackSource} style={imageStyle} contentFit="contain" />
          ) : (
            <MaterialCommunityIcons name={match.glyph.icon} size={22} color={semantics.subtleStrongText} />
          )}
        </YStack>
      </YStack>
      <YStack gap="$2" minWidth={0}>
        <XStack alignItems="center" justifyContent="space-between" gap="$2">
          <Text fontSize={isCompact ? '$2' : '$3'} fontWeight="800" numberOfLines={2} flex={1}>
            {item.name}
          </Text>
          <FleetDeviceChargeIcon state={item.chargeState} />
        </XStack>
        <SocBar value={item.batteryPct} fullWidth />
      </YStack>
    </Card>
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

function ChartSkeleton({ minHeight = 210 }: { minHeight?: number }) {
  const semantics = useThemeSemantics();

  return (
    <YStack
      minHeight={minHeight}
      borderRadius="$4"
      borderWidth={1}
      style={{
        backgroundColor: semantics.tileBackground,
        borderColor: semantics.tileBorder
      }}
    />
  );
}

function AnalyticsPlaceholder({ isTabletUp }: { isTabletUp: boolean }) {
  return (
    <YStack testID="devices-analytics-placeholder" gap="$3">
      <ChartSection title="Solar Generation" subtitle="Today against yesterday">
        <ChartSkeleton />
      </ChartSection>
      {isTabletUp ? (
        <XStack gap="$3" alignItems="stretch" flexWrap="nowrap">
          <YStack flex={1} minWidth={0} alignSelf="stretch">
            <ChartSection title="Live Power Profile" subtitle="Fleet load against supply" fill>
              <ChartSkeleton />
            </ChartSection>
          </YStack>
          <YStack flex={1} minWidth={0} alignSelf="stretch">
            <ChartSkeleton minHeight={280} />
          </YStack>
        </XStack>
      ) : (
        <YStack gap="$3">
          <ChartSection title="Live Power Profile" subtitle="Fleet load against supply">
            <ChartSkeleton />
          </ChartSection>
          <ChartSkeleton minHeight={280} />
        </YStack>
      )}
    </YStack>
  );
}

export function SummaryPanel({
  devices,
  onAllDevicesPress
}: {
  devices: DeviceSummary[];
  onAllDevicesPress?: () => void;
}) {
  const semantics = useThemeSemantics();
  const { contentWidth } = useNavigationShellMetrics();
  const isTabletUp = contentWidth >= 768;
  const isPhone = contentWidth < 640;
  const { ref: analyticsPanelRef, shouldLoad: analyticsShouldLoad } = useLazySectionLoad();
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
    enabled: historyEnabled && analyticsShouldLoad
  });

  const { summary, uniqueTypes } = useFleetSummaryViewModel({
    devices,
    byId,
    useRemoteImage
  });
  const previewLimit = resolveFleetDevicePreviewLimit(contentWidth);
  const previewDevices = useMemo(
    () => buildFleetDevicePreview(devices, { maxItems: previewLimit, snapshotsById: byId }),
    [byId, devices, previewLimit]
  );
  const suppressFleetSolar = useMemo(() => shouldSuppressFleetSolar(devices), [devices]);
  const displayPvW = suppressFleetSolar ? 0 : summary.pvW;
  const displayNetW =
    suppressFleetSolar && typeof summary.netW === 'number' && typeof summary.pvW === 'number'
      ? summary.netW - summary.pvW
      : summary.netW;
  const displayFleetTrend = useMemo(
    () =>
      mergePowerTrendPrefill(
        fleetPowerTrendHistory.data,
        {
          solar: fleetTrend.pv,
          ac: fleetTrend.ac,
          dc: fleetTrend.dc,
          load: fleetTrend.load
        },
        fleetTrend.filledPoints
      ),
    [fleetPowerTrendHistory.data, fleetTrend]
  );
  const displayFleetTrendPv = useMemo(
    () => (suppressFleetSolar ? displayFleetTrend.solar.map(() => 0) : displayFleetTrend.solar),
    [displayFleetTrend.solar, suppressFleetSolar]
  );
  const fleetSolarHistoryView = fleetSolarHistory.data;
  const fleetSolarHistoryErrorText =
    fleetSolarHistory.error && !fleetSolarHistoryView
      ? describeSolarHistoryError(fleetSolarHistory.error)
      : undefined;
  const fleetSolarHistoryLoading = fleetSolarHistory.isFetching && !fleetSolarHistoryView;
  const liveDeviceCount = useMemo(
    () => devices.filter((device) => !byId[device.id]?.inactive).length,
    [byId, devices]
  );

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

  const solarHistorySection = (
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
  );
  const livePowerSection = (
    <ChartSection title="Live Power Profile" subtitle="Fleet load against supply" fill>
      <PowerTrendChart
        solar={displayFleetTrendPv}
        ac={displayFleetTrend.ac}
        dc={displayFleetTrend.dc}
        load={displayFleetTrend.load}
        battery={displayFleetTrend.load.map(() => 0)}
        points={SUMMARY_TREND_POINTS}
      />
    </ChartSection>
  );
  const energyImpactSection = (
    <YStack testID="home-energy-impact" flex={1} height="100%">
      <EnergyImpactCard
        solarWh={fleetSolarHistoryView?.todayWh}
        period="today"
        displayPeriod="today"
        isLoading={fleetSolarHistoryLoading}
        errorText={fleetSolarHistoryErrorText}
        variant="summary"
        fill
        showPeriodControls={false}
        energyLinkParams={buildEnergyRouteParams({
          scope: 'all',
          preset: 'today',
          timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
          includeComparison: true,
          panel: 'impact'
        })}
      />
    </YStack>
  );

  return (
    <YStack gap="$3">
      <Card
        gap={isPhone ? '$3' : '$4'}
        padding={isTabletUp ? '$4' : '$3'}
        position="relative"
        overflow="hidden"
        style={{
          backgroundColor: semantics.energyCardBackground,
          borderColor: semantics.energyCardBorder
        }}
      >
        <PulseHeroBackground variant="fleet" sizes="(min-width: 1600px) 2000px, 100vw" />
        <XStack
          alignItems="stretch"
          justifyContent="space-between"
          gap={isPhone ? '$3' : '$4'}
          flexWrap="wrap"
          style={{ position: 'relative', zIndex: 1 }}
        >
          <YStack gap={isPhone ? '$2' : '$3'} flex={1.1} minWidth={250} maxWidth={isTabletUp ? 500 : undefined}>
            <Text fontSize="$2" fontWeight="700" textTransform="uppercase" letterSpacing={0.8} style={{ color: semantics.solarBadgeTitle }}>
              Pulse Fleet
            </Text>
            <YStack gap="$2">
              <Text fontSize="$4" fontWeight="700" style={{ color: semantics.subtleStrongText }}>
                Solar generation today
              </Text>
              <Text
                fontWeight="800"
                letterSpacing={0}
                style={{ fontSize: isTabletUp ? 56 : 42, lineHeight: isTabletUp ? 60 : 46 }}
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

          <YStack gap="$3" flex={1} minWidth={250} maxWidth={isTabletUp ? 420 : undefined} justifyContent="space-between">
            <XStack gap="$3" flexWrap="wrap">
              {previewDevices.map((item) => (
                <FleetDeviceTile key={item.id} item={item} useRemoteImage={useRemoteImage} />
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
              onPress={onAllDevicesPress}
            >
              <XStack alignItems="center" gap="$2">
                <MaterialCommunityIcons name="format-list-bulleted" size={18} color={semantics.actionText} />
                <Text style={{ color: semantics.actionText }} fontWeight="700">
                  All Devices
                </Text>
              </XStack>
            </Button>
          </YStack>

          <YStack gap="$3" flex={1} minWidth={250} maxWidth={isTabletUp ? 420 : undefined} justifyContent="space-between">
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

      <View ref={analyticsPanelRef} testID="devices-analytics-panel">
        {analyticsShouldLoad ? (
          isTabletUp ? (
            <YStack gap="$3">
              {solarHistorySection}
              <XStack gap="$3" alignItems="stretch" flexWrap="nowrap">
                <YStack flex={1} minWidth={0} alignSelf="stretch">
                  {livePowerSection}
                </YStack>
                <YStack flex={1} minWidth={0} alignSelf="stretch">
                  {energyImpactSection}
                </YStack>
              </XStack>
            </YStack>
          ) : (
            <YStack gap="$3">
              {solarHistorySection}
              {livePowerSection}
              {energyImpactSection}
            </YStack>
          )
        ) : (
          <AnalyticsPlaceholder isTabletUp={isTabletUp} />
        )}
      </View>
    </YStack>
  );
}
