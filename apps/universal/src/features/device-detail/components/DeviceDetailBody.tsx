import { Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot, TelemetryEngineStatus } from '@/features/telemetry/engine/types';
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
import { formatKWh } from '@/features/telemetry/format';
import type { DeviceDetailViewModel } from '@/features/device-detail/view-model';
import { BatteryPacksSection } from '@/features/device-detail/components/BatteryPacksSection';
import { SolarInputsSection } from '@/features/device-detail/components/SolarInputsSection';
import { SystemSignalsSection } from '@/features/device-detail/components/SystemSignalsSection';

const DETAIL_TREND_POINTS = 60;
const SOLAR_GENERATED_POINTS = 72;

export function DeviceDetailBody({
  device,
  snapshot,
  telemetryConnectionStatus,
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
  solarGeneratedTrend
}: {
  device?: DeviceSummary;
  snapshot?: DeviceSnapshot;
  telemetryConnectionStatus: TelemetryEngineStatus;
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
}) {
  const detailMetricItems: MetricsGridItem[] = vm.metricCells.map((cell) => {
    if (cell.kind === 'today') {
      return {
        key: cell.key,
        content: <SolarTodayBadge valueWh={cell.valueWh} compact fitCell />
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
                    pvW={snapshot?.metrics?.pvW ?? device?.pvW}
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
            <ChartSection title="☼ Solar Generated (6am-6pm, 10m buckets)" subtitle="1m refresh">
              <SolarGeneratedChart valuesWh={solarGeneratedTrend} points={SOLAR_GENERATED_POINTS} />
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
          <ChartSection title="☼ Solar Generated (6am-6pm, 10m buckets)" subtitle="1m refresh">
            <SolarGeneratedChart valuesWh={solarGeneratedTrend} points={SOLAR_GENERATED_POINTS} />
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

      {hasBatteryPacks || hasSolarInputs ? (
        <XStack gap="$3" flexWrap="wrap">
          {hasBatteryPacks ? (
            <BatteryPacksSection
              packs={vm.batteryPacks}
              bpCount={vm.details?.bpCount}
              model={device?.model}
              serialNumber={device?.serialNumber}
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

      <Card gap="$2">
        <Text fontSize="$4" fontWeight="700">
          Connection
        </Text>
        <Text opacity={0.8}>Engine: {telemetryConnectionStatus}</Text>
        <Text opacity={0.8}>Staleness: {snapshot?.stale ? 'STALE (>5s)' : 'fresh'}</Text>
        <Text opacity={0.8}>Serial: {device?.serialNumber ?? '—'}</Text>
      </Card>
    </YStack>
  );
}
