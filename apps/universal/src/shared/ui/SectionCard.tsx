import type { ReactNode } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';

export function SectionCard({
  title,
  right,
  children,
  minWidth,
  flex = 1
}: {
  title: ReactNode;
  right?: ReactNode;
  children: ReactNode;
  minWidth?: number;
  flex?: number;
}) {
  return (
    <Card gap="$3" flex={flex} minWidth={minWidth}>
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

