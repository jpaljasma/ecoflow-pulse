import { Text, XStack, YStack } from 'tamagui';
import { Card } from '@/shared/ui/Card';
import { useThemeSemantics } from '@/shared/theme/semantic';

export type StatusBannerProps = {
  iconText: string;
  headline: string;
  detail?: string;
  footnote?: string;
  statusLabel?: string;
};

export function StatusBanner({
  iconText,
  headline,
  detail,
  footnote,
  statusLabel
}: StatusBannerProps) {
  const semantics = useThemeSemantics();

  return (
    <Card
      gap="$3"
      style={{
        backgroundColor: semantics.actionBackground,
        borderColor: semantics.actionBorder
      }}
    >
      <XStack justifyContent="space-between" alignItems="flex-start" gap="$3" flexWrap="wrap">
        <XStack gap="$3" alignItems="flex-start" flex={1}>
          <YStack
            width={38}
            height={38}
            borderRadius={999}
            alignItems="center"
            justifyContent="center"
            style={{
              backgroundColor: semantics.statusWarning
            }}
          >
            <Text
              fontSize="$5"
              fontWeight="900"
              style={{ color: semantics.periodActiveText }}
            >
              {iconText}
            </Text>
          </YStack>

          <YStack gap="$1" flex={1}>
            <Text fontSize="$5" fontWeight="800">
              {headline}
            </Text>
            {detail ? <Text color="$colorMuted">{detail}</Text> : null}
          </YStack>
        </XStack>

        {statusLabel ? (
          <YStack
            paddingHorizontal="$3"
            paddingVertical="$2"
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
        <Text color="$colorMuted" fontSize="$3">
          {footnote}
        </Text>
      ) : null}
    </Card>
  );
}
