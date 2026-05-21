import { useRouter } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Button, Text, XStack, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';
import { formatWhAndKWh } from '@/features/telemetry/format';
import {
  buildEnergyImpactRows,
  computeEnergyImpactFromSolarWh,
  DEFAULT_AVOIDED_EMISSIONS_FACTOR_KEY,
  energyImpactPeriodLabel,
  type EnergyImpactPeriod
} from '@/features/energy-impact/model';
import { getEnergyImpactBadgeColors, useThemeSemantics } from '@/shared/theme/semantic';
const PERIOD_BUTTON_WIDTH = 122;

export function EnergyImpactCard({
  solarWh,
  title = 'Energy Impact',
  minWidth,
  period = 'today',
  displayPeriod = period,
  displayPeriodLabel,
  onPeriodChange,
  isLoading = false,
  errorText,
  showPeriodControls = true,
  variant = 'detailed',
  energyLinkParams,
  fill = false
}: {
  solarWh?: number;
  title?: string;
  minWidth?: number;
  period?: EnergyImpactPeriod;
  displayPeriod?: EnergyImpactPeriod;
  displayPeriodLabel?: string;
  onPeriodChange?: (period: EnergyImpactPeriod) => void;
  isLoading?: boolean;
  errorText?: string;
  showPeriodControls?: boolean;
  variant?: 'detailed' | 'summary';
  energyLinkParams?: Record<string, string>;
  fill?: boolean;
}) {
  const router = useRouter();
  const semantics = useThemeSemantics();
  const hasSolarWh = typeof solarWh === 'number' && Number.isFinite(solarWh);
  const impact = computeEnergyImpactFromSolarWh(hasSolarWh ? solarWh : 0, DEFAULT_AVOIDED_EMISSIONS_FACTOR_KEY);
  const periodLabel = displayPeriodLabel || energyImpactPeriodLabel(displayPeriod);
  const periodButtons: Array<{ key: EnergyImpactPeriod; label: string }> = [
    { key: 'today', label: 'Today so far' },
    { key: 'past12Months', label: 'Past 12 months' }
  ];
  const statusMessage = errorText ?? (isLoading ? `Loading ${periodLabel} solar history…` : ' ');
  const energyRows = buildEnergyImpactRows(impact, displayPeriod, displayPeriodLabel);

  if (variant === 'summary') {
    return (
      <Card
        flex={fill ? 1 : undefined}
        gap="$3"
        height={fill ? '100%' : undefined}
        justifyContent={fill ? 'space-between' : undefined}
        minWidth={minWidth}
        minHeight={280}
        style={{ backgroundColor: semantics.energyCardBackground, borderColor: semantics.energyCardBorder }}
      >
        <XStack justifyContent="space-between" alignItems="flex-start" gap="$3">
          <XStack alignItems="center" gap="$2" flex={1} minWidth={0}>
            <YStack
              width={38}
              height={38}
              borderRadius={18}
              alignItems="center"
              justifyContent="center"
              borderWidth={1}
              style={{ backgroundColor: semantics.energyLeafBackground, borderColor: semantics.energyLeafBorder }}
            >
              <MaterialCommunityIcons name="leaf" size={18} color={semantics.energyLeafText} />
            </YStack>
            <YStack gap="$1" flex={1} minWidth={0}>
              <Text fontSize="$4" fontWeight="700" numberOfLines={1}>
                {title}
              </Text>
              <Text fontSize="$1" opacity={0.82} numberOfLines={1}>
                {periodLabel}
              </Text>
            </YStack>
          </XStack>

          {energyLinkParams ? (
            <Button
              size="$3"
              borderRadius="$5"
              borderWidth={1}
              minHeight={36}
              style={{ borderColor: semantics.actionBorder, backgroundColor: semantics.actionBackground }}
              onPress={() => {
                router.push({
                  pathname: '/(tabs)/energy',
                  params: energyLinkParams
                });
              }}
            >
              <XStack alignItems="center" gap="$2">
                <MaterialCommunityIcons name="lightning-bolt-outline" size={16} color={semantics.actionText} />
                <Text fontWeight="700" style={{ color: semantics.actionText }}>
                  Energy
                </Text>
              </XStack>
            </Button>
          ) : null}
        </XStack>

        <YStack gap="$1">
          <Text fontSize="$8" fontWeight="800" letterSpacing={-0.7}>
            {hasSolarWh ? (energyRows[0]?.detail.split(' CO2e')[0] ?? '0 g') : '—'}
          </Text>
          <Text fontSize="$2" style={{ color: semantics.subtleStrongText }}>
            {hasSolarWh
              ? `Avoided grid CO2e from ${formatWhAndKWh(impact.solarWh)} over ${periodLabel}.`
              : `Waiting for ${periodLabel} solar history.`}
          </Text>
          <Text
            fontSize="$1"
            style={{ color: semantics.subtleStrongText, opacity: statusMessage.trim() ? 1 : 0 }}
            minHeight={16}
          >
            {statusMessage}
          </Text>
        </YStack>

        <XStack gap="$2" flexWrap="wrap">
          {energyRows.slice(1, 4).map((metric) => {
            const colors = getEnergyImpactBadgeColors(metric.key, semantics);
            return (
              <YStack
                key={metric.key}
                flex={1}
                minWidth={112}
                padding="$3"
                borderRadius="$4"
                borderWidth={1}
                style={{
                  borderColor: semantics.mutedPanelBorder,
                  backgroundColor: semantics.mutedPanelBackground
                }}
              >
                <Text fontSize="$1" fontWeight="700" style={{ color: colors.color }} textTransform="uppercase" letterSpacing={0.6}>
                  {metric.badge}
                </Text>
                <Text fontSize="$3" fontWeight="700" numberOfLines={2}>
                  {metric.headline}
                </Text>
                <Text fontSize="$2" style={{ color: semantics.subtleText }} numberOfLines={2}>
                  {metric.detail}
                </Text>
              </YStack>
            );
          })}
        </XStack>
      </Card>
    );
  }

  return (
    <Card
      flex={fill ? 1 : undefined}
      gap="$3"
      height={fill ? '100%' : undefined}
      minWidth={minWidth}
      style={{ backgroundColor: semantics.energyCardBackground, borderColor: semantics.energyCardBorder }}
    >
      <XStack justifyContent="space-between" alignItems="flex-start" gap="$2">
        <XStack alignItems="center" gap="$2">
          <YStack
            width={38}
            height={38}
            borderRadius={18}
            alignItems="center"
            justifyContent="center"
            borderWidth={1}
            style={{ backgroundColor: semantics.energyLeafBackground, borderColor: semantics.energyLeafBorder }}
          >
            <MaterialCommunityIcons name="leaf" size={18} color={semantics.energyLeafText} />
          </YStack>
          <YStack gap="$1">
            <Text fontSize="$4" fontWeight="700">
              {title}
            </Text>
            <Text fontSize="$1" opacity={0.82}>
              Pollution avoided + lifecycle equivalents
            </Text>
          </YStack>
        </XStack>
        {showPeriodControls ? (
          <XStack gap="$2" flexWrap="wrap" justifyContent="flex-end" maxWidth={260}>
            {periodButtons.map((item) => {
              const active = period === item.key;
              return (
                <Button
                  key={item.key}
                  size="$3"
                  minWidth={PERIOD_BUTTON_WIDTH}
                  borderRadius="$5"
                  borderWidth={1}
                  style={{
                    backgroundColor: active ? semantics.periodActiveBackground : semantics.periodIdleBackground,
                    borderColor: active ? semantics.periodActiveBorder : semantics.periodIdleBorder,
                    opacity: isLoading && item.key === 'past12Months' && active ? 0.74 : 1
                  }}
                  onPress={() => {
                    onPeriodChange?.(item.key);
                  }}
                >
                  <Text
                    fontWeight="700"
                    style={{ color: active ? semantics.periodActiveText : semantics.periodIdleText }}
                  >
                    {item.label}
                  </Text>
                </Button>
              );
            })}
          </XStack>
        ) : null}
      </XStack>

      <YStack gap="$1">
        <Text opacity={0.96}>
          {hasSolarWh
            ? `Estimated avoided grid emissions from ${formatWhAndKWh(impact.solarWh)} of solar generated over ${periodLabel}.`
            : `Waiting for ${periodLabel} solar history before estimating avoided grid emissions.`}
        </Text>
        <Text fontSize="$1" style={{ color: semantics.subtleStrongText }}>
          Avoided pollutants use {impact.factor.label} default factors from {impact.factor.source} ({impact.factorKey}). Tree equivalent uses a separate conservative lifecycle benchmark.
        </Text>
        <Text
          fontSize="$1"
          style={{ color: semantics.subtleStrongText, opacity: statusMessage.trim() ? 1 : 0 }}
          minHeight={16}
        >
          {statusMessage}
        </Text>
      </YStack>

      <YStack gap="$2" opacity={isLoading ? 0.86 : 1}>
        {energyRows.map((metric) => {
          const colors = getEnergyImpactBadgeColors(metric.key, semantics);
          return (
            <XStack
              key={metric.key}
              alignItems="center"
              gap="$3"
              padding="$2"
              borderRadius="$3"
              borderWidth={1}
              style={{ borderColor: semantics.mutedPanelBorder }}
            >
              <YStack
                width={50}
                height={50}
                borderRadius="$3"
                alignItems="center"
                justifyContent="center"
                flexShrink={0}
                style={{ backgroundColor: colors.bg }}
              >
                <Text
                  fontSize={metric.badge.length > 4 ? '$1' : '$2'}
                  fontWeight="800"
                  style={{ color: colors.color }}
                >
                  {metric.badge}
                </Text>
              </YStack>

              <YStack flex={1} minWidth={0} gap="$1">
                <Text fontWeight="700" numberOfLines={1}>
                  {metric.headline}
                </Text>
                <Text fontSize="$2" style={{ color: semantics.subtleStrongText }}>
                  {metric.detail}
                </Text>
              </YStack>

              <Button
                size="$3"
                minWidth={84}
                borderRadius="$5"
                borderWidth={1}
                style={{ borderColor: semantics.actionBorder, backgroundColor: semantics.actionBackground }}
                onPress={() => {
                  router.push({
                    pathname: '/energy-impact',
                    params: { focus: metric.key }
                  });
                }}
              >
                <Text fontWeight="700" style={{ color: semantics.actionText }}>
                  Explain
                </Text>
              </Button>
            </XStack>
          );
        })}
      </YStack>
    </Card>
  );
}
