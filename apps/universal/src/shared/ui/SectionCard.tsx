import type { ComponentProps, ReactNode } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';

export function SectionCard({
  eyebrow,
  title,
  subtitle,
  right,
  children,
  minWidth,
  flex = 1,
  fullWidth = false,
  contentGap = '$2'
}: {
  eyebrow?: ReactNode;
  title: ReactNode;
  subtitle?: ReactNode;
  right?: ReactNode;
  children: ReactNode;
  minWidth?: number;
  flex?: number;
  fullWidth?: boolean;
  contentGap?: ComponentProps<typeof YStack>['gap'];
}) {
  return (
    <Card
      gap="$3"
      flex={fullWidth ? undefined : flex}
      minWidth={minWidth}
      width={fullWidth ? '100%' : undefined}
      alignSelf={fullWidth ? 'stretch' : undefined}
    >
      <XStack justifyContent="space-between" alignItems="flex-start" gap="$3" flexWrap="wrap">
        <YStack gap="$2" flex={1} minWidth={0}>
          {eyebrow ? renderEyebrow(eyebrow) : null}
          {renderTitle(title)}
          {subtitle ? renderSubtitle(subtitle) : null}
        </YStack>
        {right ? <YStack alignItems="flex-end">{right}</YStack> : null}
      </XStack>
      <YStack gap={contentGap}>{children}</YStack>
    </Card>
  );
}

function renderEyebrow(content: ReactNode) {
  if (typeof content === 'string') {
    return (
      <Text fontSize="$2" fontWeight="700" textTransform="uppercase" letterSpacing={0.7} color="$colorMuted">
        {content}
      </Text>
    );
  }
  return content;
}

function renderTitle(content: ReactNode) {
  if (typeof content === 'string') {
    return (
      <Text fontSize="$5" fontWeight="700" letterSpacing={-0.2}>
        {content}
      </Text>
    );
  }
  return content;
}

function renderSubtitle(content: ReactNode) {
  if (typeof content === 'string') {
    return (
      <Text color="$colorMuted" lineHeight={22}>
        {content}
      </Text>
    );
  }
  return content;
}
