import type { ReactNode } from 'react';
import type { ComponentProps } from 'react';
import { Platform } from 'react-native';
import { Text, YStack } from 'tamagui';
import { useThemeSemantics } from '@/shared/theme/semantic';

function valueTextStyle(color?: string): ComponentProps<typeof Text>['style'] {
  return {
    ...(Platform.OS === 'web' ? { fontVariantNumeric: 'tabular-nums' } : undefined),
    ...(color ? { color } : undefined)
  } as ComponentProps<typeof Text>['style'];
}

export function Stat({
  label,
  value,
  tone = 'default',
  compact = false,
  minWidth = 96
}: {
  label: ReactNode;
  value: ReactNode;
  tone?: 'default' | 'muted' | 'cold';
  compact?: boolean;
  minWidth?: number;
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
    <YStack gap="$1" minWidth={minWidth}>
      {typeof label === 'string' ? (
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
      ) : (
        label
      )}
      <Text
        fontFamily="$body"
        fontSize={compact ? '$3' : '$4'}
        fontWeight="800"
        letterSpacing={0}
        style={valueTextStyle(resolvedColor)}
        opacity={1}
        numberOfLines={1}
      >
        {value}
      </Text>
    </YStack>
  );
}
