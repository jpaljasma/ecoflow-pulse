import type { ComponentProps } from 'react';
import { router } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Platform } from 'react-native';
import { Button, Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';
import { DeviceEnergyImpactCard } from '@/features/energy-impact/DeviceEnergyImpactCard';
import { buildStormGuardLabel } from '@/features/devices/stormGuard';
import { BatteryPacksSection } from '@/features/device-detail/components/BatteryPacksSection';
import { DiagnosticsSection } from '@/features/device-detail/components/DiagnosticsSection';
import { DeviceSolarForecastCard } from '@/features/device-detail/components/DeviceSolarForecastCard';
import { SolarInputsSection } from '@/features/device-detail/components/SolarInputsSection';
import { SystemSignalsSection } from '@/features/device-detail/components/SystemSignalsSection';
import type { DeviceDetailViewModel } from '@/features/device-detail/view-model';
import { SOLAR_HISTORY_CHART_TITLE, SOLAR_HISTORY_POINTS } from '@/features/history/solar';
import type { DeviceInsights } from '@/features/inference/api';
import { formatKWh, formatSoc, formatWhAndKWh } from '@/features/telemetry/format';
import type { SolarOutlook } from '@/features/weather/model';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { Card } from '@/shared/ui/Card';
import { ChartSection } from '@/shared/ui/ChartSection';
import { DeviceHeroPanel } from '@/shared/ui/DeviceHeroPanel';
import { IconLabel } from '@/shared/ui/IconLabel';
import { MetricsGrid, type MetricsGridItem } from '@/shared/ui/MetricsGrid';
import { PowerFlowGlyph } from '@/shared/ui/PowerFlowGlyph';
import { PowerTrendChart } from '@/shared/ui/PowerTrendChart';
import { SocBar } from '@/shared/ui/SocBar';
import { SolarGeneratedChart } from '@/shared/ui/SolarGeneratedChart';
import { Stat } from '@/shared/ui/Stat';
import { StormGuardChip } from '@/shared/ui/StormGuardChip';

const DETAIL_TREND_POINTS = 60;

function metricLabel(label: string) {
  switch (label) {
    case 'AC':
      return <IconLabel icon="power-plug-outline" label="AC" />;
    case 'DC':
      return <IconLabel icon="current-dc" label="DC" />;
    case 'PV':
      return <IconLabel icon="white-balance-sunny" label="PV" />;
    case 'Load':
      return <IconLabel icon="home-outline" label="Load" />;
    case 'Net':
      return <IconLabel icon="scale-balance" label="Net" />;
    case 'Battery':
      return <IconLabel icon="battery-high" label="Battery" />;
    case 'Temp':
      return <IconLabel icon="thermometer" label="Temp" />;
    case 'State':
      return <IconLabel icon="checkbox-blank-circle-outline" label="State" />;
    case 'ETA':
      return <IconLabel icon="timer-sand" label="ETA" />;
    default:
      return label;
  }
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

function detailStateLabel(state: DeviceDetailViewModel['detailState']): string {
  if (state === 'charging') return 'Charging reserve';
  if (state === 'discharging') return 'Serving load';
  return 'Standing by';
}

function connectionSummary(snapshot?: DeviceSnapshot): string {
  if (!snapshot) return 'Waiting for telemetry';
  if (snapshot.inactive || snapshot.stale || !snapshot.online) return 'Live data paused';
  return 'Live telemetry active';
}

function resolveDetailSoc(
  details: DeviceSummary['details'] | undefined,
  snapshot: DeviceSnapshot | undefined,
  device: DeviceSummary | undefined
): number | undefined {
  if (typeof details?.overallSocPct === 'number' && Number.isFinite(details.overallSocPct)) {
    return details.overallSocPct;
  }

  const packSocs = (details?.packs ?? [])
    .map((pack) => pack.socPct)
    .filter((value): value is number => typeof value === 'number' && Number.isFinite(value));
  if (packSocs.length > 0) {
    return packSocs.reduce((total, value) => total + value, 0) / packSocs.length;
  }

  if (typeof snapshot?.metrics?.soc === 'number' && Number.isFinite(snapshot.metrics.soc)) {
    return snapshot.metrics.soc;
  }

  if (typeof device?.batteryPct === 'number' && Number.isFinite(device.batteryPct)) {
    return device.batteryPct;
  }

  return undefined;
}

function netSummary(state: DeviceDetailViewModel['detailState']): string {
  if (state === 'charging') return 'Solar is rebuilding reserve';
  if (state === 'discharging') return 'Battery is supporting demand';
  return 'System balance is steady';
}

function detailMetricAccent(
  key: 'today' | 'pv' | 'load' | 'net',
  semantics: ReturnType<typeof useThemeSemantics>
): string {
  switch (key) {
    case 'today':
    case 'pv':
      return semantics.chartSolar;
    case 'load':
      return semantics.chartLoad;
    case 'net':
      return semantics.chartAc;
  }
}

function DetailHeroTile({
  icon,
  label,
  value,
  detail,
  accent,
  semantics
}: {
  icon: ComponentProps<typeof MaterialCommunityIcons>['name'];
  label: string;
  value: string;
  detail: string;
  accent: string;
  semantics: ReturnType<typeof useThemeSemantics>;
}) {
  return (
    <YStack
      flexGrow={1}
      flexBasis={152}
      minWidth={148}
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
        <Text
          fontSize="$2"
          fontWeight="700"
          textTransform="uppercase"
          letterSpacing={0.6}
          style={{ color: semantics.subtleStrongText }}
        >
          {label}
        </Text>
        <YStack
          width={30}
          height={30}
          borderRadius={999}
          alignItems="center"
          justifyContent="center"
          style={{ backgroundColor: `${accent}20` }}
        >
          <MaterialCommunityIcons name={icon} size={16} color={accent} />
        </YStack>
      </XStack>
      <Text fontSize="$6" fontWeight="800" letterSpacing={-0.4} numberOfLines={1}>
        {value}
      </Text>
      <Text fontSize="$2" numberOfLines={2} style={{ color: semantics.subtleText }}>
        {detail}
      </Text>
    </YStack>
  );
}

export function DeviceDetailBody({
  device,
  snapshot,
  vm,
  isTablet,
  isDesktop,
  mediaColumnWidth,
  mediaBoxHeight,
  mobileImageSize,
  sparklineLoad,
  sparklinePV,
  sparklineAC,
  sparklineDC,
  solarGeneratedTrend,
  solarGeneratedYesterdayTrend,
  solarGeneratedTodayWh,
  solarGeneratedYesterdayWh,
  solarGeneratedDeltaPct,
  solarOutlook,
  solarOutlookLoading,
  solarOutlookErrorText,
  batteryInsights,
  batteryInsightsLoading
}: {
  device?: DeviceSummary;
  snapshot?: DeviceSnapshot;
  vm: DeviceDetailViewModel;
  isTablet: boolean;
  isDesktop: boolean;
  mediaColumnWidth: number;
  mediaBoxHeight: number;
  mobileImageSize: number;
  sparklineLoad: number[];
  sparklinePV: number[];
  sparklineAC: number[];
  sparklineDC: number[];
  solarGeneratedTrend: number[];
  solarGeneratedYesterdayTrend: number[];
  solarGeneratedTodayWh?: number;
  solarGeneratedYesterdayWh?: number;
  solarGeneratedDeltaPct?: number | null;
  solarOutlook?: SolarOutlook;
  solarOutlookLoading?: boolean;
  solarOutlookErrorText?: string;
  batteryInsights?: DeviceInsights;
  batteryInsightsLoading?: boolean;
}) {
  const semantics = useThemeSemantics();
  const statCellByKey = new Map(
    vm.metricCells
      .filter((cell): cell is Extract<DeviceDetailViewModel['metricCells'][number], { kind: 'stat' }> => cell.kind === 'stat')
      .map((cell) => [cell.key, cell])
  );

  const resolvedSoc = resolveDetailSoc(vm.details, snapshot, device);
  const heroSoc = formatSoc(resolvedSoc);
  const heroEta = statCellByKey.get('eta')?.value;
  const stormGuardLabel = buildStormGuardLabel(vm.details);
  const capacitySummary = [
    vm.capacityKWh !== null ? `${formatKWh(vm.capacityKWh)} installed` : 'Capacity unavailable',
    vm.batterySummaryText
  ]
    .filter(Boolean)
    .join(' · ');
  const stateAccent =
    vm.detailState === 'charging'
      ? semantics.chartBatteryCharge
      : vm.detailState === 'discharging'
        ? semantics.chartLoad
        : semantics.chartAc;
  const stateLabel = detailStateLabel(vm.detailState);
  const liveSummary = connectionSummary(snapshot);

  const detailMetricItems: MetricsGridItem[] = vm.metricCells.flatMap((cell) => {
    if (
      cell.kind === 'today' ||
      cell.key === 'pv' ||
      cell.key === 'load' ||
      cell.key === 'net' ||
      cell.key === 'state'
    ) {
      return [];
    }

    return [
      {
        key: cell.key,
        content: (
          <Stat label={metricLabel(cell.label)} value={cell.value} tone={cell.tone ?? 'default'} />
        )
      }
    ];
  });

  const heroTiles = [
    {
      key: 'today',
      icon: 'white-balance-sunny',
      label: 'Solar today',
      value: formatWhAndKWh(solarGeneratedTodayWh),
      detail: formatDeltaSummary(solarGeneratedDeltaPct),
      accent: detailMetricAccent('today', semantics)
    },
    {
      key: 'pv',
      icon: 'white-balance-sunny',
      label: 'PV now',
      value: statCellByKey.get('pv')?.value ?? '—',
      detail: 'Current photovoltaic input',
      accent: detailMetricAccent('pv', semantics)
    },
    {
      key: 'load',
      icon: 'home-lightning-bolt-outline',
      label: 'Load',
      value: statCellByKey.get('load')?.value ?? '—',
      detail: 'Active site demand',
      accent: detailMetricAccent('load', semantics)
    },
    {
      key: 'net',
      icon: 'transmission-tower',
      label: 'Net balance',
      value: statCellByKey.get('net')?.value ?? '—',
      detail: netSummary(vm.detailState),
      accent: detailMetricAccent('net', semantics)
    }
  ] as const;

  const hasBatteryPacks = vm.batteryPacks.length > 0 || typeof vm.details?.bpCount === 'number';
  const hasSolarInputs = vm.solarPorts.length > 0;
  const hasSignals = vm.signalPills.length > 0;
  const hasDiagnostics = vm.diagnosticPills.length > 0;

  return (
    <YStack gap="$3">
      <Card
        gap="$4"
        padding={isTablet ? '$5' : '$4'}
        style={
          Platform.OS === 'web'
            ? {
                backgroundImage: `${semantics.heroBackground}, radial-gradient(circle at 74% 20%, ${semantics.heroGlow} 0%, rgba(0,0,0,0) 46%)`,
                borderColor: semantics.heroBorder
              }
            : {
                backgroundColor: semantics.surfaceRaised,
                borderColor: semantics.heroBorder
              }
        }
      >
        <DeviceHeroPanel
          stacked={!isTablet}
          leftWidth={isTablet ? mediaColumnWidth : '100%'}
          imageWidth={isTablet ? mediaColumnWidth : mobileImageSize}
          imageHeight={mediaBoxHeight}
          imageScale={isTablet ? vm.desktopScale : 1.35}
          imageOffsetY={isTablet ? vm.desktopOffsetY : 0}
          imageUri={vm.deviceAsset?.uri}
          fallbackSource={vm.detailFallback}
          iconFallback={vm.deviceAsset?.icon}
          leftFooter={(
            <YStack gap="$2" alignItems="center">
              {device ? (
                <PowerFlowGlyph
                  status={vm.detailState}
                  pvW={vm.displayPvW ?? snapshot?.metrics?.pvW ?? device?.pvW}
                  loadW={snapshot?.metrics?.loadW ?? device?.loadW}
                  fontSize={isDesktop ? '$8' : '$7'}
                  lineHeight={isDesktop ? 30 : 26}
                />
              ) : null}
              <Text fontSize="$2" fontWeight="700" style={{ color: semantics.subtleStrongText }} textAlign="center">
                {device?.model ?? 'Energy system'}
              </Text>
            </YStack>
          )}
          right={(
            <YStack gap="$4" flex={1} minWidth={0}>
              <XStack alignItems="flex-start" justifyContent="space-between" gap="$3" flexWrap="wrap">
                <XStack gap="$2" flexWrap="wrap" flex={1}>
                  <XStack
                    alignItems="center"
                    gap="$2"
                    paddingHorizontal="$3"
                    paddingVertical="$2"
                    borderRadius={999}
                    borderWidth={1}
                    style={{
                      backgroundColor: semantics.tileBackground,
                      borderColor: stateAccent
                    }}
                  >
                    <MaterialCommunityIcons name="battery-heart-variant" size={16} color={stateAccent} />
                    <Text fontSize="$2" fontWeight="700" style={{ color: stateAccent }}>
                      {stateLabel}
                    </Text>
                  </XStack>
                  {stormGuardLabel ? (
                    <StormGuardChip label={stormGuardLabel} />
                  ) : null}
                  <XStack
                    alignItems="center"
                    gap="$2"
                    paddingHorizontal="$3"
                    paddingVertical="$2"
                    borderRadius={999}
                    borderWidth={1}
                    style={{
                      backgroundColor: semantics.mutedPanelBackground,
                      borderColor: semantics.mutedPanelBorder
                    }}
                  >
                    <MaterialCommunityIcons name={vm.connectionGlyph} size={16} color={semantics.subtleStrongText} />
                    <Text fontSize="$2" fontWeight="700" style={{ color: semantics.subtleStrongText }}>
                      {liveSummary}
                    </Text>
                  </XStack>
                </XStack>

                {device ? (
                  <Button
                    size="$3"
                    borderRadius={999}
                    borderWidth={1}
                    paddingHorizontal="$4"
                    minHeight={42}
                    style={{
                      backgroundColor: semantics.actionBackground,
                      borderColor: semantics.actionBorder
                    }}
                    onPress={() =>
                      router.push({
                        pathname: '/(tabs)/energy',
                        params: {
                          device: device.id,
                          preset: 'today',
                          compare: '1'
                        }
                      })
                    }
                  >
                    <XStack alignItems="center" gap="$2">
                      <MaterialCommunityIcons name="lightning-bolt-outline" size={18} color={semantics.actionText} />
                      <Text style={{ color: semantics.actionText }} fontWeight="700">
                        Open Energy
                      </Text>
                    </XStack>
                  </Button>
                ) : null}
              </XStack>

              <YStack gap="$2">
                <Text
                  fontSize="$2"
                  fontWeight="700"
                  textTransform="uppercase"
                  letterSpacing={0.8}
                  style={{ color: semantics.subtleStrongText }}
                >
                  Battery reserve
                </Text>
                <XStack alignItems="flex-end" gap="$3" flexWrap="wrap">
                  <Text
                    fontWeight="800"
                    letterSpacing={-1}
                    style={{ fontSize: isTablet ? 60 : 46, lineHeight: isTablet ? 62 : 48 }}
                  >
                    {heroSoc}
                  </Text>
                  {heroEta && heroEta !== '—' ? (
                    <YStack
                      gap={2}
                      marginBottom="$2"
                      paddingHorizontal="$3"
                      paddingVertical="$2"
                      borderRadius="$4"
                      borderWidth={1}
                      style={{
                        backgroundColor: semantics.tileBackground,
                        borderColor: semantics.tileBorder
                      }}
                    >
                      <Text
                        fontSize="$2"
                        fontWeight="700"
                        textTransform="uppercase"
                        letterSpacing={0.6}
                        style={{ color: semantics.subtleStrongText }}
                      >
                        ETA
                      </Text>
                      <Text fontSize="$4" fontWeight="700">
                        {heroEta}
                      </Text>
                    </YStack>
                  ) : null}
                </XStack>
                <Text fontSize="$3" style={{ color: semantics.subtleStrongText }}>
                  {capacitySummary}
                </Text>
              </YStack>

              <SocBar value={resolvedSoc} fullWidth />

              <XStack gap="$3" flexWrap="wrap">
                {heroTiles.map((tile) => (
                  <DetailHeroTile
                    key={tile.key}
                    icon={tile.icon}
                    label={tile.label}
                    value={tile.value}
                    detail={tile.detail}
                    accent={tile.accent}
                    semantics={semantics}
                  />
                ))}
              </XStack>
            </YStack>
          )}
        />
      </Card>

      {isTablet ? (
        <XStack gap="$3" alignItems="stretch" flexWrap="nowrap">
          <YStack flex={1.1} minWidth={0}>
            <ChartSection title={SOLAR_HISTORY_CHART_TITLE} subtitle="Today against yesterday">
              <SolarGeneratedChart
                valuesWh={solarGeneratedTrend}
                yesterdayValuesWh={solarGeneratedYesterdayTrend}
                todayWh={solarGeneratedTodayWh}
                yesterdayWh={solarGeneratedYesterdayWh}
                deltaPct={solarGeneratedDeltaPct}
                points={SOLAR_HISTORY_POINTS}
              />
            </ChartSection>
          </YStack>
          <YStack flex={0.95} minWidth={0}>
            <ChartSection title="Power profile" subtitle="Live supply against demand">
              <PowerTrendChart
                solar={sparklinePV}
                ac={sparklineAC}
                dc={sparklineDC}
                load={sparklineLoad}
                battery={sparklineLoad.map(() => 0)}
                points={DETAIL_TREND_POINTS}
              />
            </ChartSection>
          </YStack>
        </XStack>
      ) : (
        <YStack gap="$3">
          <ChartSection title={SOLAR_HISTORY_CHART_TITLE} subtitle="Today against yesterday">
            <SolarGeneratedChart
              valuesWh={solarGeneratedTrend}
              yesterdayValuesWh={solarGeneratedYesterdayTrend}
              todayWh={solarGeneratedTodayWh}
              yesterdayWh={solarGeneratedYesterdayWh}
              deltaPct={solarGeneratedDeltaPct}
              points={SOLAR_HISTORY_POINTS}
            />
          </ChartSection>
          <ChartSection title="Power profile" subtitle="Live supply against demand">
            <PowerTrendChart
              solar={sparklinePV}
              ac={sparklineAC}
              dc={sparklineDC}
              load={sparklineLoad}
              battery={sparklineLoad.map(() => 0)}
              points={DETAIL_TREND_POINTS}
            />
          </ChartSection>
        </YStack>
      )}

      <Card gap="$3" padding="$4" backgroundColor="$backgroundElevated">
        <Text fontSize="$5" fontWeight="700">
          Live telemetry
        </Text>
        <Text fontSize="$3" style={{ color: semantics.subtleText }}>
          AC, DC, battery, thermal, and runtime signals for the current device state.
        </Text>
        <MetricsGrid items={detailMetricItems} columns={isDesktop ? 3 : isTablet ? 3 : 2} />
      </Card>

      {isTablet ? (
        <XStack gap="$3" alignItems="stretch" flexWrap="wrap">
          <YStack flex={1} minWidth={isDesktop ? 360 : 320}>
            <DeviceEnergyImpactCard deviceId={device?.id} todaySolarWh={solarGeneratedTodayWh} />
          </YStack>
          <YStack flex={1} minWidth={isDesktop ? 360 : 320}>
            <DeviceSolarForecastCard
              deviceName={device?.name}
              deviceId={device?.id}
              solarOutlook={solarOutlook}
              isLoading={solarOutlookLoading}
              errorText={solarOutlookErrorText}
            />
          </YStack>
        </XStack>
      ) : (
        <YStack gap="$3">
          <DeviceEnergyImpactCard deviceId={device?.id} todaySolarWh={solarGeneratedTodayWh} />
          <DeviceSolarForecastCard
            deviceName={device?.name}
            deviceId={device?.id}
            solarOutlook={solarOutlook}
            isLoading={solarOutlookLoading}
            errorText={solarOutlookErrorText}
          />
        </YStack>
      )}

      {hasBatteryPacks || hasSolarInputs ? (
        <XStack gap="$3" flexWrap="wrap">
          {hasBatteryPacks ? (
            <BatteryPacksSection
              packs={vm.batteryPacks}
              bpCount={vm.details?.bpCount}
              summaryText={vm.batterySummaryText}
              model={device?.model}
              batteryInsights={batteryInsights}
              batteryInsightsLoading={batteryInsightsLoading}
              minWidth={isDesktop ? 320 : 280}
            />
          ) : null}
          {hasSolarInputs ? (
            <SolarInputsSection ports={vm.solarPorts} minWidth={isDesktop ? 320 : 280} />
          ) : null}
        </XStack>
      ) : null}

      {hasSignals ? (
        <XStack gap="$3" flexWrap="wrap">
          <SystemSignalsSection pills={vm.signalPills} minWidth={isDesktop ? 360 : 280} />
        </XStack>
      ) : null}

      {hasDiagnostics ? (
        <XStack gap="$3" flexWrap="wrap">
          <DiagnosticsSection pills={vm.diagnosticPills} minWidth={isDesktop ? 360 : 280} />
        </XStack>
      ) : null}
    </YStack>
  );
}
