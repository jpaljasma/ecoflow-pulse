import type { ReactNode } from 'react';
import { Text, YStack } from 'tamagui';

export function Stat({
  label,
  value,
  tone = 'default',
  compact = false
}: {
  label: string;
  value: ReactNode;
  tone?: 'default' | 'muted' | 'cold';
  compact?: boolean;
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
        fontSize={compact ? '$1' : '$3'}
        fontWeight="500"
        numberOfLines={1}
      >
        {label}
      </Text>
      <Text
        fontFamily="$body"
        fontSize={compact ? '$3' : '$4'}
        fontWeight="700"
        letterSpacing={-0.1}
        color={resolvedColor}
        opacity={1}
        numberOfLines={1}
      >
        {value}
      </Text>
    </YStack>
  );
}
