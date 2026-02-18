import { Text, YStack } from 'tamagui';

export function Stat({ label, value, tone = 'default' }: { label: string; value: string; tone?: 'default' | 'muted' }) {
  return (
    <YStack gap="$1" minWidth={96}>
      <Text fontFamily="$body" color={tone === 'muted' ? '$colorHover' : '$color'} opacity={0.75} fontSize="$3" fontWeight="500">
        {label}
      </Text>
      <Text fontFamily="$body" fontSize="$4" fontWeight="700" letterSpacing={-0.1}>
        {value}
      </Text>
    </YStack>
  );
}
