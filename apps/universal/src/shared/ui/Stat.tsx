import { Text, YStack } from 'tamagui';

export function Stat({ label, value, tone = 'default' }: { label: string; value: string; tone?: 'default' | 'muted' }) {
  return (
    <YStack gap="$1" minWidth={88}>
      <Text color={tone === 'muted' ? '$colorHover' : '$color'} opacity={0.7} fontSize="$2">
        {label}
      </Text>
      <Text fontSize="$5" fontWeight="700" letterSpacing={-0.2}>
        {value}
      </Text>
    </YStack>
  );
}
