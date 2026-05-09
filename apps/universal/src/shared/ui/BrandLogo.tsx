import { Pressable } from 'react-native';
import { Text, XStack } from 'tamagui';
import { PulseMark } from '@/shared/ui/PulseMark';

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
      <PulseMark size={iconSize} />
      <Text
        fontFamily="$heading"
        fontSize={pulseSize}
        lineHeight={pulseSize}
        fontWeight="800"
        letterSpacing={0}
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
