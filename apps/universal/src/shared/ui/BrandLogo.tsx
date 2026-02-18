import { Text, XStack, YStack } from 'tamagui';

export function BrandLogo({ compact = false }: { compact?: boolean }) {
  return (
    <XStack alignItems="center" gap="$2">
      <YStack
        width={compact ? 24 : 28}
        height={compact ? 24 : 28}
        borderRadius="$5"
        alignItems="center"
        justifyContent="center"
        backgroundColor="rgba(10,132,255,0.18)"
        borderWidth={1}
        borderColor="rgba(10,132,255,0.35)"
      >
        <Text fontSize={compact ? '$2' : '$3'}>⚡</Text>
      </YStack>
      <Text fontFamily="$heading" fontSize={compact ? '$6' : '$7'} fontWeight="800" letterSpacing={-0.45}>
        EcoFlow Pulse
      </Text>
    </XStack>
  );
}
