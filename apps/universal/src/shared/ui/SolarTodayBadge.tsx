import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Platform } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import { formatWhAndKWh } from '@/features/telemetry/format';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { formatSolarComparisonDeltaText } from '@/shared/ui/solarLegend';

export function SolarTodayBadge({
  valueWh,
  previousWh,
  deltaPct,
  compact = false,
  fitCell = false
}: {
  valueWh: number | undefined | null;
  previousWh?: number | undefined | null;
  deltaPct?: number | null;
  compact?: boolean;
  fitCell?: boolean;
}) {
  const semantics = useThemeSemantics();
  const deltaText = formatSolarComparisonDeltaText(valueWh, previousWh, deltaPct);
  return (
    <YStack
      paddingHorizontal={compact ? '$2' : '$3'}
      paddingVertical={compact ? '$2' : '$3'}
      borderRadius="$3"
      borderWidth={1}
      style={{
        borderColor: semantics.solarBadgeBorder,
        backgroundColor: semantics.solarBadgeBackground,
        ...(Platform.OS === 'web'
          ? {
              backgroundImage: `linear-gradient(135deg, ${semantics.solarBadgeGradientStart} 0%, ${semantics.solarBadgeGradientEnd} 100%)`
            }
          : undefined)
      }}
      gap="$1"
      justifyContent={fitCell ? 'center' : undefined}
      minHeight={fitCell ? 56 : undefined}
    >
      <XStack alignItems="center" gap="$1">
        <MaterialCommunityIcons
          name="white-balance-sunny"
          size={compact ? 14 : 16}
          color={semantics.solarBadgeTitle}
        />
        <Text fontSize={compact ? '$2' : '$3'} fontWeight="800" style={{ color: semantics.solarBadgeTitle }}>
          Today
        </Text>
      </XStack>
      <XStack alignItems="baseline" gap="$1" flexWrap="wrap">
        <Text fontSize={compact ? '$1' : '$2'} fontWeight="800" style={{ color: semantics.solarBadgeValue }}>
          {formatWhAndKWh(valueWh)}
        </Text>
        {deltaText ? (
          <Text fontSize={compact ? '$1' : '$2'} fontWeight="700" style={{ color: semantics.solarBadgeDelta }}>
            {deltaText}
          </Text>
        ) : null}
      </XStack>
    </YStack>
  );
}
