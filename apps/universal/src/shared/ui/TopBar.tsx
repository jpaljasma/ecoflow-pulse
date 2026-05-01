import type { ReactNode } from 'react';
import { Platform } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Text, XStack, YStack } from 'tamagui';

export function TopBar({
  eyebrow,
  title,
  subtitle,
  left,
  right,
  titleFlex = 1,
  rightFlex
}: {
  eyebrow?: ReactNode;
  title: ReactNode;
  subtitle?: ReactNode;
  left?: ReactNode;
  right?: ReactNode;
  titleFlex?: number;
  rightFlex?: number;
}) {
  const insets = useSafeAreaInsets();
  const topInset = Platform.OS === 'web' ? 0 : insets.top;

  const titleNode =
    typeof title === 'string' || typeof title === 'number' ? (
      <Text
        fontFamily="$heading"
        fontSize="$8"
        fontWeight="800"
        letterSpacing={-0.25}
        numberOfLines={1}
        ellipsizeMode="tail"
      >
        {title}
      </Text>
    ) : (
      <XStack>{title}</XStack>
    );

  return (
    <XStack
      alignItems="flex-start"
      justifyContent="space-between"
      paddingHorizontal="$5"
      paddingVertical="$4"
      paddingTop={topInset > 0 ? topInset + 8 : 12}
      gap="$4"
    >
      {left ? <XStack>{left}</XStack> : null}
      <YStack gap={eyebrow ? '$2' : '$1'} flex={titleFlex} minWidth={0}>
        {eyebrow
          ? typeof eyebrow === 'string' || typeof eyebrow === 'number'
            ? (
              <Text
                fontFamily="$body"
                fontSize="$2"
                opacity={0.76}
                numberOfLines={1}
                ellipsizeMode="tail"
              >
                {eyebrow}
              </Text>
              )
            : eyebrow
          : null}
        {titleNode}
        {subtitle
          ? typeof subtitle === 'string' || typeof subtitle === 'number'
            ? (
              <Text
                fontFamily="$body"
                fontSize="$4"
                opacity={0.78}
                numberOfLines={1}
                ellipsizeMode="tail"
              >
                {subtitle}
              </Text>
              )
            : subtitle
          : null}
      </YStack>
      {right ? (
        <XStack flex={rightFlex} flexShrink={0} justifyContent="flex-end" alignItems="flex-start">
          {right}
        </XStack>
      ) : null}
    </XStack>
  );
}
