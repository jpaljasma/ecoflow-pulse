import { useRouter } from 'expo-router';
import { Button, Text, XStack, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';
import { formatWhAndKWh } from '@/features/telemetry/format';
import {
  computeEnergyImpactFromSolarWh,
  DEFAULT_AVOIDED_EMISSIONS_FACTOR_KEY,
  energyImpactPeriodLabel,
  type EnergyImpactPeriod
} from '@/features/energy-impact/model';

const CARD_BG = 'rgba(20,184,166,0.05)';
const CARD_BORDER = 'rgba(45,212,191,0.32)';
const LEAF_BADGE_BG = 'rgba(45,212,191,0.14)';
const LEAF_BADGE_BORDER = 'rgba(45,212,191,0.30)';
const LEAF_BADGE_TEXT = '#0f766e';
const ACTION_BG = 'rgba(59,130,246,0.08)';
const ACTION_BORDER = 'rgba(59,130,246,0.24)';
const ACTION_TEXT = '#155e75';
const PERIOD_ACTIVE_BG = 'rgba(13,148,136,0.14)';
const PERIOD_ACTIVE_BORDER = 'rgba(13,148,136,0.34)';
const PERIOD_ACTIVE_TEXT = '#0f766e';
const PERIOD_IDLE_BG = 'rgba(120,120,128,0.08)';
const PERIOD_IDLE_BORDER = 'rgba(120,120,128,0.24)';
const PERIOD_IDLE_TEXT = 'rgba(70,70,74,0.92)';
const PERIOD_BUTTON_WIDTH = 122;

function badgeColors(metricKey: 'co2e' | 'nox' | 'so2' | 'trees') {
  switch (metricKey) {
    case 'co2e':
      return { bg: 'rgba(34,197,94,0.16)', color: '#15803d' };
    case 'nox':
      return { bg: 'rgba(20,184,166,0.16)', color: '#0f766e' };
    case 'so2':
      return { bg: 'rgba(59,130,246,0.16)', color: '#1d4ed8' };
    case 'trees':
      return { bg: 'rgba(16,185,129,0.18)', color: '#047857' };
  }
}

export function EnergyImpactCard({
  solarWh,
  title = 'Energy Impact',
  minWidth,
  period = 'today',
  onPeriodChange,
  isLoading = false,
  errorText
}: {
  solarWh?: number;
  title?: string;
  minWidth?: number;
  period?: EnergyImpactPeriod;
  onPeriodChange?: (period: EnergyImpactPeriod) => void;
  isLoading?: boolean;
  errorText?: string;
}) {
  const router = useRouter();
  const impact = computeEnergyImpactFromSolarWh(solarWh ?? 0, DEFAULT_AVOIDED_EMISSIONS_FACTOR_KEY);
  const periodLabel = energyImpactPeriodLabel(period);
  const periodButtons: Array<{ key: EnergyImpactPeriod; label: string }> = [
    { key: 'today', label: 'Today so far' },
    { key: 'past12Months', label: 'Past 12 months' }
  ];
  const statusMessage = errorText ?? (isLoading ? `Loading ${periodLabel} solar history…` : ' ');

  return (
    <Card gap="$3" minWidth={minWidth} style={{ backgroundColor: CARD_BG, borderColor: CARD_BORDER }}>
      <XStack justifyContent="space-between" alignItems="flex-start" gap="$2">
        <XStack alignItems="center" gap="$2">
          <YStack
            width={38}
            height={38}
            borderRadius={18}
            alignItems="center"
            justifyContent="center"
            borderWidth={1}
            style={{ backgroundColor: LEAF_BADGE_BG, borderColor: LEAF_BADGE_BORDER }}
          >
            <Text fontSize="$4" style={{ color: LEAF_BADGE_TEXT }}>
              🍃
            </Text>
          </YStack>
          <YStack gap="$1">
            <Text fontSize="$4" fontWeight="700">
              {title}
            </Text>
            <Text fontSize="$1" opacity={0.62}>
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
                  backgroundColor: active ? PERIOD_ACTIVE_BG : PERIOD_IDLE_BG,
                  borderColor: active ? PERIOD_ACTIVE_BORDER : PERIOD_IDLE_BORDER,
                  opacity: isLoading && item.key === 'past12Months' && active ? 0.74 : 1
                }}
                onPress={() => {
                  onPeriodChange?.(item.key);
                }}
              >
                <Text
                  fontWeight="700"
                  style={{ color: active ? PERIOD_ACTIVE_TEXT : PERIOD_IDLE_TEXT }}
                >
                  {item.label}
                </Text>
              </Button>
            );
          })}
        </XStack>
      </XStack>

      <YStack gap="$1">
        <Text opacity={0.84}>
          Estimated avoided grid emissions from {formatWhAndKWh(impact.solarWh)} of solar generated over {periodLabel}.
        </Text>
        <Text fontSize="$1" opacity={0.6}>
          Avoided pollutants use {impact.factor.label} default factors from {impact.factor.source} ({impact.factorKey}). Tree equivalent uses a separate conservative lifecycle benchmark.
        </Text>
        <Text fontSize="$1" opacity={statusMessage.trim() ? 0.68 : 0} minHeight={16}>
          {statusMessage}
        </Text>
      </YStack>

      <YStack gap="$2" opacity={isLoading ? 0.86 : 1}>
        {impact.metrics.map((metric) => {
          const colors = badgeColors(metric.key);
          return (
            <XStack
              key={metric.key}
              alignItems="center"
              gap="$3"
              padding="$2"
              borderRadius="$3"
              borderWidth={1}
              borderColor="rgba(120,120,128,0.24)"
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
                  {metric.label}
                </Text>
                <Text fontSize="$2" opacity={0.68}>
                  {metric.detail}
                </Text>
              </YStack>

              <Button
                size="$3"
                minWidth={84}
                borderRadius="$5"
                borderWidth={1}
                style={{ borderColor: ACTION_BORDER, backgroundColor: ACTION_BG }}
                onPress={() => {
                  router.push({
                    pathname: '/energy-impact',
                    params: { focus: metric.key }
                  });
                }}
              >
                <Text fontWeight="700" style={{ color: ACTION_TEXT }}>
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
