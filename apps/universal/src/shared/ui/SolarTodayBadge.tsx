import { Platform } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import { formatWhAndKWh } from '@/features/telemetry/format';
import { useThemeSemantics } from '@/shared/theme/semantic';

export function SolarTodayBadge({
  valueWh,
  deltaPct,
  compact = false,
  fitCell = false
}: {
  valueWh: number | undefined | null;
  deltaPct?: number | null;
  compact?: boolean;
  fitCell?: boolean;
}) {
  const semantics = useThemeSemantics();
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
      <Text fontSize={compact ? '$2' : '$3'} fontWeight="800" style={{ color: semantics.solarBadgeTitle }}>
        ☼ Today
      </Text>
      <XStack alignItems="baseline" gap="$1" flexWrap="wrap">
        <Text fontSize={compact ? '$1' : '$2'} fontWeight="800" style={{ color: semantics.solarBadgeValue }}>
          {formatWhAndKWh(valueWh)}
        </Text>
        {deltaPct !== null && deltaPct !== undefined && Number.isFinite(deltaPct) ? (
          <Text fontSize={compact ? '$1' : '$2'} fontWeight="700" style={{ color: semantics.solarBadgeDelta }}>
            {formatDelta(deltaPct)}
          </Text>
        ) : null}
      </XStack>
    </YStack>
  );
}

function formatDelta(deltaPct: number | null | undefined): string {
  if (deltaPct === null || deltaPct === undefined || !Number.isFinite(deltaPct)) {
    return '';
  }
  const rounded = Math.round(deltaPct);
  const sign = rounded > 0 ? '+' : '';
  return `${sign}${rounded}%`;
}
