import type { ReactNode } from 'react';
import { Text, XStack, YStack } from 'tamagui';

export function TopBar({
  title,
  subtitle,
  left,
  right,
  titleFlex = 1,
  rightFlex
}: {
  title: ReactNode;
  subtitle?: string;
  left?: ReactNode;
  right?: ReactNode;
  titleFlex?: number;
  rightFlex?: number;
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
      alignItems="flex-start"
      justifyContent="space-between"
      paddingHorizontal="$4"
      paddingVertical="$3"
      gap="$3"
    >
      {left ? <XStack>{left}</XStack> : null}
      <YStack gap="$1" flex={titleFlex}>
        {titleNode}
        {subtitle ? (
          <Text fontFamily="$body" fontSize="$4" opacity={0.78}>
            {subtitle}
          </Text>
        ) : null}
      </YStack>
      {right ? (
        <XStack flex={rightFlex} justifyContent="flex-end" alignItems="flex-start">
          {right}
        </XStack>
      ) : null}
    </XStack>
  );
}
