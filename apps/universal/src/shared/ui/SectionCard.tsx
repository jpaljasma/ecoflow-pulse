import type { ReactNode } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';

export function SectionCard({
  title,
  right,
  children,
  minWidth,
  flex = 1,
  fullWidth = false
}: {
  title: ReactNode;
  right?: ReactNode;
  children: ReactNode;
  minWidth?: number;
  flex?: number;
  fullWidth?: boolean;
}) {
  return (
    <Card gap="$3" flex={fullWidth ? undefined : flex} minWidth={minWidth} width={fullWidth ? '100%' : undefined} alignSelf={fullWidth ? 'stretch' : undefined}>
      <XStack justifyContent="space-between" alignItems="center">
        <Text fontSize="$4" fontWeight="700">
          {title}
        </Text>
        {right}
      </XStack>
      <YStack gap="$2">{children}</YStack>
    </Card>
  );
}
