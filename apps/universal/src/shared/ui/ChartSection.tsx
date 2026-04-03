import type { ReactNode } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import { useThemeSemantics } from '@/shared/theme/semantic';

export function ChartSection({
  title,
  subtitle,
  right,
  children
}: {
  title: ReactNode;
  subtitle?: ReactNode;
  right?: ReactNode;
  children: ReactNode;
}) {
  const semantics = useThemeSemantics();
  return (
    <YStack
      gap="$2"
      padding="$4"
      borderRadius="$4"
      borderWidth={1}
      style={{
        borderColor: semantics.surfaceRaisedBorder,
        backgroundColor: semantics.sectionBackgroundStrong
      }}
    >
      <XStack alignItems="center" justifyContent="space-between" gap="$2">
        <Text fontSize="$5" fontWeight="700" letterSpacing={-0.2}>
          {title}
        </Text>
        {right}
      </XStack>
      {subtitle ? (
        <Text fontSize="$2" color="$colorMuted" opacity={0.94}>
          {subtitle}
        </Text>
      ) : null}
      {children}
    </YStack>
  );
}
