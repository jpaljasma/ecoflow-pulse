import type { ReactNode } from 'react';
import { Text, XStack, YStack } from 'tamagui';

export function TopBar({
  title,
  subtitle,
  left,
  right
}: {
  title: ReactNode;
  subtitle?: string;
  left?: ReactNode;
  right?: ReactNode;
}) {
  const titleNode =
    typeof title === 'string' || typeof title === 'number' ? (
      <Text fontFamily="$heading" fontSize="$8" fontWeight="800" letterSpacing={-0.25}>
        {title}
      </Text>
    ) : (
      <XStack>{title}</XStack>
    );

  return (
    <XStack
      alignItems="center"
      justifyContent="space-between"
      paddingHorizontal="$4"
      paddingVertical="$3"
      gap="$3"
    >
      {left ? <XStack>{left}</XStack> : null}
      <YStack gap="$1" flex={1}>
        {titleNode}
        {subtitle ? (
          <Text fontFamily="$body" fontSize="$4" opacity={0.78}>
            {subtitle}
          </Text>
        ) : null}
      </YStack>
      {right}
    </XStack>
  );
}
