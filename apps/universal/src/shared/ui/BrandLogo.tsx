import { Image, Platform, Pressable } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import { getBundledBrandWordmark } from '@/shared/assets/brandBundled';
import { useAppTheme } from '@/shared/theme/useAppTheme';

export function BrandLogo({
  compact = false,
  dense = false,
  onPress
}: {
  compact?: boolean;
  dense?: boolean;
  onPress?: () => void;
}) {
  const { isDark } = useAppTheme();
  const theme = isDark ? 'dark' : 'light';
  const logoWidth = dense ? 122 : compact ? 150 : 210;
  const logoHeight = dense ? 22 : compact ? 26 : 36;
  const pulseSize = logoHeight;
  const logoSrc = getBundledBrandWordmark(theme);

  const content = (
    <XStack alignItems="center" gap="$2">
      <YStack
        width={logoWidth}
        height={logoHeight}
        alignItems="center"
        justifyContent="center"
        overflow="hidden"
      >
        <Image
          source={logoSrc}
          style={{ width: logoWidth, height: logoHeight }}
          resizeMode="contain"
        />
      </YStack>
      <Text
        fontFamily="$heading"
        fontSize={pulseSize}
        lineHeight={pulseSize}
        fontWeight="800"
        fontStyle="italic"
        letterSpacing={-0.7}
        paddingLeft={dense ? 8 : 12}
        style={Platform.OS === 'web' ? ({ fontSize: '2em' } as any) : undefined}
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
