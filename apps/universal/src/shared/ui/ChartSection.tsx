import type { ReactNode } from 'react';
import { Text, XStack, YStack } from 'tamagui';

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
  return (
    <YStack
      gap="$2"
      padding="$3"
      borderRadius="$3"
      borderWidth={1}
      borderColor="rgba(120,120,128,0.24)"
    >
      <XStack alignItems="center" justifyContent="space-between" gap="$2">
        <Text fontSize="$4" fontWeight="700">
          {title}
        </Text>
        {right}
      </XStack>
      {subtitle ? (
        <Text fontSize="$2" opacity={0.72}>
          {subtitle}
        </Text>
      ) : null}
      {children}
    </YStack>
  );
}
