import { Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';
import { Card } from '@/shared/ui/Card';
import { ChartSection } from '@/shared/ui/ChartSection';
import { DeviceHeroPanel } from '@/shared/ui/DeviceHeroPanel';
import { MetricsGrid, type MetricsGridItem } from '@/shared/ui/MetricsGrid';
import { PowerFlowGlyph } from '@/shared/ui/PowerFlowGlyph';
import { SocBar } from '@/shared/ui/SocBar';
import { Stat } from '@/shared/ui/Stat';
import { PowerTrendChart } from '@/shared/ui/PowerTrendChart';
import { SolarGeneratedChart } from '@/shared/ui/SolarGeneratedChart';
import { SolarTodayBadge } from '@/shared/ui/SolarTodayBadge';
import { DeviceEnergyImpactCard } from '@/features/energy-impact/DeviceEnergyImpactCard';
import { SOLAR_HISTORY_CHART_TITLE, SOLAR_HISTORY_POINTS } from '@/features/history/solar';
import { formatKWh } from '@/features/telemetry/format';
import type { DeviceDetailViewModel } from '@/features/device-detail/view-model';
import { BatteryPacksSection } from '@/features/device-detail/components/BatteryPacksSection';
import { SolarInputsSection } from '@/features/device-detail/components/SolarInputsSection';
import { SystemSignalsSection } from '@/features/device-detail/components/SystemSignalsSection';
import type { DeviceInsights } from '@/features/inference/api';

const DETAIL_TREND_POINTS = 60;
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
  batteryInsights?: DeviceInsights;
  batteryInsightsLoading?: boolean;
}) {
  const detailMetricItems: MetricsGridItem[] = vm.metricCells.map((cell) => {
    if (cell.kind === 'today') {
      return {
        key: cell.key,
        content: <SolarTodayBadge valueWh={cell.valueWh} deltaPct={cell.deltaPct} compact fitCell />
      };
    }
    return {
      key: cell.key,
      content: (
        <Stat label={cell.label} value={cell.value} tone={cell.tone ?? 'default'} />
      )
    };
  });

  const hasBatteryPacks = vm.batteryPacks.length > 0 || typeof vm.details?.bpCount === 'number';
  const hasSolarInputs = vm.solarPorts.length > 0;
  const hasSignals = vm.signalPills.length > 0;

  return (
    <YStack gap="$3">
      <Card gap="$3">
        <DeviceHeroPanel
          stacked={!isTablet}
          leftWidth={isTablet ? mediaColumnWidth : '100%'}
          imageWidth={isTablet ? mediaColumnWidth : mobileImageSize}
          imageHeight={mediaBoxHeight}
          imageScale={isTablet ? vm.desktopScale : 1.35}
          imageOffsetY={isTablet ? vm.desktopOffsetY : 0}
          imageUri={vm.deviceAsset?.uri}
          fallbackSource={vm.detailFallback}
          emojiFallback={vm.deviceAsset?.emoji}
          right={(
            <YStack gap="$3" flex={1} minWidth={0}>
              <XStack alignItems="flex-end" gap="$3">
                <XStack flex={1} minWidth={0}>
                  <SocBar value={snapshot?.metrics?.soc ?? device?.batteryPct} fullWidth />
                </XStack>
                <Text fontSize="$3" opacity={0.75} marginBottom="$1" flexShrink={0}>
                  {vm.capacityKWh !== null ? `🔋 ${formatKWh(vm.capacityKWh)}` : '🔋 n/a'}
                </Text>
              </XStack>

              <MetricsGrid items={detailMetricItems} columns={3} />

              <XStack justifyContent="flex-end" alignItems="center" gap="$2">
                {device ? (
                  <PowerFlowGlyph
                    status={vm.detailState}
                    pvW={vm.displayPvW ?? snapshot?.metrics?.pvW ?? device?.pvW}
                    loadW={snapshot?.metrics?.loadW ?? device?.loadW}
                    fontSize="$6"
                    lineHeight={24}
                  />
                ) : null}
                <Text fontSize="$3" opacity={0.9}>
                  {vm.connectionGlyph}
                </Text>
              </XStack>
            </YStack>
          )}
        />
      </Card>

      {isTablet ? (
        <XStack gap="$3" alignItems="stretch" flexWrap="nowrap">
          <YStack flexBasis="50%" minWidth="50%" maxWidth="50%">
            <ChartSection title={SOLAR_HISTORY_CHART_TITLE} subtitle="1m refresh">
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
          <YStack flexBasis="50%" minWidth="50%" maxWidth="50%">
            <ChartSection title="Power Trends">
              <PowerTrendChart
                solar={sparklinePV}
                ac={sparklineAC}
                dc={sparklineDC}
                load={sparklineLoad}
                points={DETAIL_TREND_POINTS}
              />
            </ChartSection>
          </YStack>
        </XStack>
      ) : (
        <YStack gap="$3">
          <ChartSection title={SOLAR_HISTORY_CHART_TITLE} subtitle="1m refresh">
            <SolarGeneratedChart
              valuesWh={solarGeneratedTrend}
              yesterdayValuesWh={solarGeneratedYesterdayTrend}
              todayWh={solarGeneratedTodayWh}
              yesterdayWh={solarGeneratedYesterdayWh}
              deltaPct={solarGeneratedDeltaPct}
              points={SOLAR_HISTORY_POINTS}
            />
          </ChartSection>
          <ChartSection title="Power Trends">
            <PowerTrendChart
              solar={sparklinePV}
              ac={sparklineAC}
              dc={sparklineDC}
              load={sparklineLoad}
              points={DETAIL_TREND_POINTS}
            />
          </ChartSection>
        </YStack>
      )}

      <DeviceEnergyImpactCard deviceId={device?.id} todaySolarWh={solarGeneratedTodayWh} />

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
    </YStack>
  );
}
