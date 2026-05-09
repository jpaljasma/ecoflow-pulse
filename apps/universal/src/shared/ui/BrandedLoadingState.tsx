import { ActivityIndicator } from 'react-native';
import { Text, YStack } from 'tamagui';
import { PulseMark } from '@/shared/ui/PulseMark';

export function BrandedLoadingState({
  message = 'Loading…',
  minHeight = 200
}: {
  message?: string;
  minHeight?: number;
}) {
  return (
    <YStack minHeight={minHeight} alignItems="center" justifyContent="center" gap="$4">
      <YStack
        width={68}
        height={68}
        alignItems="center"
        justifyContent="center"
        borderRadius="$4"
      >
        <PulseMark size={58} />
      </YStack>
      <ActivityIndicator size="large" />
      <Text color="$color" opacity={0.96} fontSize="$5" fontWeight="700">
        {message}
      </Text>
    </YStack>
  );
}
