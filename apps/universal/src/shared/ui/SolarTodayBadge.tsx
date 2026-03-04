import { Platform } from 'react-native';
import { Text, YStack } from 'tamagui';
import { formatWhAndKWh } from '@/features/telemetry/format';

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
  return (
    <YStack
      paddingHorizontal={compact ? '$2' : '$3'}
      paddingVertical={compact ? '$2' : '$3'}
      borderRadius="$3"
      borderWidth={1}
      borderColor="rgba(255,149,0,0.65)"
      backgroundColor="rgba(255,149,0,0.08)"
      style={
        Platform.OS === 'web'
          ? ({
              backgroundImage:
                'linear-gradient(135deg, rgba(255,149,0,0.16) 0%, rgba(255,149,0,0.05) 100%)'
            } as any)
          : undefined
      }
      gap="$1"
      justifyContent={fitCell ? 'center' : undefined}
      minHeight={fitCell ? 56 : undefined}
    >
      <Text fontSize={compact ? '$2' : '$3'} fontWeight="700" color="rgba(160,82,0,0.95)">
        ☼ Today
      </Text>
      <Text fontSize={compact ? '$1' : '$2'} fontWeight="600" color="rgba(120,64,0,0.95)">
        {formatWhAndKWh(valueWh)}{formatDelta(deltaPct)}
      </Text>
    </YStack>
  );
}

function formatDelta(deltaPct: number | null | undefined): string {
  if (deltaPct === null || deltaPct === undefined || !Number.isFinite(deltaPct)) {
    return '';
  }
  const rounded = Math.round(deltaPct);
  const sign = rounded > 0 ? '+' : '';
  return ` (${sign}${rounded}%)`;
}
