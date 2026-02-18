import { Image, Platform, Pressable } from 'react-native';
import { useColorScheme } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import { getEcoFlowLogoForTheme } from '@/shared/assets/ecoflowBrandAssets';

export function BrandLogo({
  compact = false,
  onPress
}: {
  compact?: boolean;
  onPress?: () => void;
}) {
  const scheme = useColorScheme();
  const theme = scheme === 'dark' ? 'dark' : 'light';
  const logoWidth = compact ? 150 : 210;
  const logoHeight = compact ? 26 : 36;
  const pulseSize = logoHeight;
  const logoSrc = getEcoFlowLogoForTheme(theme, 'wordmark');

  const content = (
    <XStack alignItems="center" gap="$2">
      <YStack
        width={logoWidth}
        height={logoHeight}
        alignItems="center"
        justifyContent="center"
        overflow="hidden"
      >
        {Platform.OS === 'web' ? (
          <Image
            source={{ uri: logoSrc }}
            style={{ width: logoWidth, height: logoHeight }}
            resizeMode="contain"
          />
        ) : (
          <Text fontSize={compact ? '$2' : '$3'}>⚡</Text>
        )}
      </YStack>
      <Text
        fontFamily="$heading"
        fontSize={pulseSize}
        lineHeight={pulseSize}
        fontWeight="800"
        fontStyle="italic"
        letterSpacing={-0.7}
        paddingLeft={12}
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
