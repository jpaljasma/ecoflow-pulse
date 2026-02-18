import { useWindowDimensions } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';

function clamp(value: number): number {
  if (Number.isNaN(value)) return 0;
  return Math.max(0, Math.min(100, value));
}

export function SocBar({ value }: { value: number | null | undefined }) {
  const { width } = useWindowDimensions();
  const isTabletUp = width >= 768;
  const pct = clamp(value ?? 0);

  return (
    <YStack gap="$2" width={isTabletUp ? '50%' : '100%'} minWidth={isTabletUp ? 220 : 0}>
      <XStack alignItems="center" justifyContent="space-between">
        <Text fontFamily="$body" fontSize="$3" opacity={0.78} fontWeight="500">
          SOC
        </Text>
        <Text fontFamily="$body" fontSize="$3" fontWeight="700">
          {Number.isFinite(value as number) ? `${pct.toFixed(1)}%` : '—'}
        </Text>
      </XStack>
      <XStack
        height={10}
        borderRadius="$5"
        overflow="hidden"
        backgroundColor="rgba(120,120,128,0.20)"
      >
        <XStack
          height="100%"
          width={`${pct}%` as `${number}%`}
          backgroundColor={pct >= 60 ? '#30d158' : pct >= 30 ? '#ff9f0a' : '#ff453a'}
        />
      </XStack>
    </YStack>
  );
}
