import type { ReactNode } from 'react';
import { Text, YStack } from 'tamagui';
import { useThemeSemantics } from '@/shared/theme/semantic';

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
  const semantics = useThemeSemantics();
  const resolvedColor =
    tone === 'muted'
      ? semantics.subtleText
      : tone === 'cold'
        ? semantics.metricCold
        : undefined;
  const labelOpacity = tone === 'default' ? 0.92 : 1;
  return (
    <YStack gap="$1" minWidth={96}>
      <Text
        fontFamily="$body"
        style={resolvedColor ? { color: resolvedColor } : undefined}
        opacity={labelOpacity}
        fontSize={compact ? '$1' : '$3'}
        fontWeight="600"
        numberOfLines={1}
      >
        {label}
      </Text>
      <Text
        fontFamily="$body"
        fontSize={compact ? '$3' : '$4'}
        fontWeight="800"
        letterSpacing={-0.1}
        style={resolvedColor ? { color: resolvedColor } : undefined}
        opacity={1}
        numberOfLines={1}
      >
        {value}
      </Text>
    </YStack>
  );
}
