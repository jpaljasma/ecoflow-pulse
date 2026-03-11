import { useRouter } from 'expo-router';
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
  onPeriodChange,
  isLoading = false,
  errorText
}: {
  solarWh?: number;
  title?: string;
  minWidth?: number;
  period?: EnergyImpactPeriod;
  displayPeriod?: EnergyImpactPeriod;
  onPeriodChange?: (period: EnergyImpactPeriod) => void;
  isLoading?: boolean;
  errorText?: string;
}) {
  const router = useRouter();
  const semantics = useThemeSemantics();
  const impact = computeEnergyImpactFromSolarWh(solarWh ?? 0, DEFAULT_AVOIDED_EMISSIONS_FACTOR_KEY);
  const periodLabel = energyImpactPeriodLabel(displayPeriod);
  const periodButtons: Array<{ key: EnergyImpactPeriod; label: string }> = [
    { key: 'today', label: 'Today so far' },
    { key: 'past12Months', label: 'Past 12 months' }
  ];
  const statusMessage = errorText ?? (isLoading ? `Loading ${periodLabel} solar history…` : ' ');

  return (
    <Card gap="$3" minWidth={minWidth} style={{ backgroundColor: semantics.energyCardBackground, borderColor: semantics.energyCardBorder }}>
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
            <Text fontSize="$4" style={{ color: semantics.energyLeafText }}>
              🍃
            </Text>
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
      </XStack>

      <YStack gap="$1">
        <Text opacity={0.96}>
          Estimated avoided grid emissions from {formatWhAndKWh(impact.solarWh)} of solar generated over {periodLabel}.
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
        {buildEnergyImpactRows(impact, displayPeriod).map((metric) => {
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
