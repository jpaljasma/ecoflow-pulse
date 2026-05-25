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
  fitCell = false,
  railCompact = false
}: {
  valueWh: number | undefined | null;
  previousWh?: number | undefined | null;
  deltaPct?: number | null;
  compact?: boolean;
  fitCell?: boolean;
  railCompact?: boolean;
}) {
  const semantics = useThemeSemantics();
  const deltaText = formatSolarComparisonDeltaText(valueWh, previousWh, deltaPct);
  return (
    <YStack
      width={railCompact ? '100%' : undefined}
      maxWidth="100%"
      minWidth={0}
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
      gap={railCompact ? 2 : '$1'}
      justifyContent={fitCell ? 'center' : undefined}
      minHeight={railCompact ? 56 : fitCell ? 56 : undefined}
    >
      <XStack alignItems="center" gap="$1" minWidth={0}>
        <MaterialCommunityIcons
          name="white-balance-sunny"
          size={railCompact ? 13 : compact ? 14 : 16}
          color={semantics.solarBadgeTitle}
        />
        <Text
          fontSize={compact ? '$2' : '$3'}
          fontWeight="800"
          numberOfLines={1}
          flexShrink={1}
          minWidth={0}
          style={{ color: semantics.solarBadgeTitle }}
        >
          Today
        </Text>
      </XStack>
      <YStack gap={0} minWidth={0}>
        <Text
          fontSize={compact ? '$1' : '$2'}
          fontWeight="800"
          numberOfLines={1}
          minWidth={0}
          style={{ color: semantics.solarBadgeValue }}
        >
          {formatWhAndKWh(valueWh)}
        </Text>
        {deltaText ? (
          <Text
            fontSize={compact ? '$1' : '$2'}
            fontWeight="700"
            numberOfLines={1}
            minWidth={0}
            style={{ color: semantics.solarBadgeDelta }}
          >
            {deltaText}
          </Text>
        ) : null}
      </YStack>
    </YStack>
  );
}
