import type { ReactNode } from 'react';
import { Text, YStack } from 'tamagui';

export function Stat({
  label,
  value,
  tone = 'default'
}: {
  label: string;
  value: ReactNode;
  tone?: 'default' | 'muted' | 'cold';
}) {
  const mutedColor = 'rgba(168,168,176,0.95)';
  const coldColor = '#2f80ed';
  const resolvedColor = tone === 'muted' ? mutedColor : tone === 'cold' ? coldColor : '$color';
  const labelOpacity = tone === 'default' ? 0.75 : 1;
  return (
    <YStack gap="$1" minWidth={96}>
      <Text
        fontFamily="$body"
        color={resolvedColor}
        opacity={labelOpacity}
        fontSize="$3"
        fontWeight="500"
      >
        {label}
      </Text>
      <Text
        fontFamily="$body"
        fontSize="$4"
        fontWeight="700"
        letterSpacing={-0.1}
        color={resolvedColor}
        opacity={1}
      >
        {value}
      </Text>
    </YStack>
  );
}
