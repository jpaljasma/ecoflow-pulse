import { Image, Pressable } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import appIcon from '../../../assets/icon.png';

export function BrandLogo({
  compact = false,
  dense = false,
  onPress
}: {
  compact?: boolean;
  dense?: boolean;
  onPress?: () => void;
}) {
  const iconSize = dense ? 24 : compact ? 30 : 40;
  const pulseSize = dense ? 26 : compact ? 32 : 42;

  const content = (
    <XStack alignItems="center" gap="$3">
      <YStack
        width={iconSize}
        height={iconSize}
        alignItems="center"
        justifyContent="center"
        overflow="hidden"
      >
        <Image
          source={appIcon}
          style={{ width: iconSize, height: iconSize, borderRadius: iconSize * 0.28 }}
          resizeMode="contain"
        />
      </YStack>
      <Text
        fontFamily="$heading"
        fontSize={pulseSize}
        lineHeight={pulseSize}
        fontWeight="800"
        letterSpacing={-0.7}
      >
        Pulse
      </Text>
    </XStack>
  );

  if (!onPress) {
    return content;
  }

  return <Pressable onPress={onPress}>{content}</Pressable>;
}
