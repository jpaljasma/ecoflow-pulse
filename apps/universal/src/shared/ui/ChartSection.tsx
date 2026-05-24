import type { ReactNode } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import { useThemeSemantics } from '@/shared/theme/semantic';
import {
  PULSE_PANEL_PADDING,
  PULSE_PANEL_RADIUS
} from '@/shared/ui/navigationShellMetrics';

export function ChartSection({
  title,
  subtitle,
  right,
  children,
  fill = false
}: {
  title: ReactNode;
  subtitle?: ReactNode;
  right?: ReactNode;
  children: ReactNode;
  fill?: boolean;
}) {
  const semantics = useThemeSemantics();
  return (
    <YStack
      flex={fill ? 1 : undefined}
      gap="$2"
      height={fill ? '100%' : undefined}
      padding={PULSE_PANEL_PADDING}
      borderRadius={PULSE_PANEL_RADIUS}
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
