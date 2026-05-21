import { Text, XStack, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';
import { useThemeSemantics } from '@/shared/theme/semantic';

export type StatusBannerProps = {
  iconText: string;
  headline: string;
  detail?: string;
  footnote?: string;
  statusLabel?: string;
  compact?: boolean;
};

export function StatusBanner({
  iconText,
  headline,
  detail,
  footnote,
  statusLabel,
  compact = false
}: StatusBannerProps) {
  const semantics = useThemeSemantics();

  return (
    <Card
      gap={compact ? '$2' : '$3'}
      padding={compact ? '$3' : '$5'}
      style={{
        backgroundColor: semantics.actionBackground,
        borderColor: semantics.actionBorder
      }}
    >
      <XStack justifyContent="space-between" alignItems="flex-start" gap={compact ? '$2' : '$3'} flexWrap="wrap">
        <XStack gap={compact ? '$2' : '$3'} alignItems="flex-start" flex={1}>
          <YStack
            width={compact ? 30 : 38}
            height={compact ? 30 : 38}
            borderRadius={999}
            alignItems="center"
            justifyContent="center"
            style={{
              backgroundColor: semantics.statusWarning
            }}
          >
            <Text
              fontSize={compact ? '$3' : '$5'}
              fontWeight="900"
              style={{ color: semantics.periodActiveText }}
            >
              {iconText}
            </Text>
          </YStack>

          <YStack gap="$1" flex={1} minWidth={0}>
            <Text fontSize={compact ? '$4' : '$5'} fontWeight="800" numberOfLines={compact ? 1 : undefined}>
              {headline}
            </Text>
            {detail ? <Text color="$colorMuted" numberOfLines={compact ? 1 : undefined}>{detail}</Text> : null}
          </YStack>
        </XStack>

        {statusLabel ? (
          <YStack
            paddingHorizontal={compact ? '$2' : '$3'}
            paddingVertical={compact ? '$1' : '$2'}
            borderRadius={999}
            style={{
              backgroundColor: semantics.periodActiveBackground,
              borderColor: semantics.periodActiveBorder
            }}
            borderWidth={1}
          >
            <Text
              fontWeight="800"
              style={{ color: semantics.periodActiveText }}
            >
              {statusLabel}
            </Text>
          </YStack>
        ) : null}
      </XStack>

      {footnote ? (
        <Text color="$colorMuted" fontSize={compact ? '$2' : '$3'} numberOfLines={compact ? 1 : undefined}>
          {footnote}
        </Text>
      ) : null}
    </Card>
  );
}
