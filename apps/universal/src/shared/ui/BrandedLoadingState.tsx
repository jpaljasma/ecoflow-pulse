import { ActivityIndicator, Image } from 'react-native';
import { Text, YStack } from 'tamagui';
import { useThemeSemantics } from '@/shared/theme/semantic';
import appIcon from '../../../assets/icon.png';

export function BrandedLoadingState({
  message = 'Loading…',
  minHeight = 200
}: {
  message?: string;
  minHeight?: number;
}) {
  const semantics = useThemeSemantics();

  return (
    <YStack minHeight={minHeight} alignItems="center" justifyContent="center" gap="$4">
      <YStack
        width={68}
        height={68}
        borderRadius="$4"
        alignItems="center"
        justifyContent="center"
        borderWidth={1}
        style={{
          backgroundColor: semantics.mutedPanelBackground,
          borderColor: semantics.mutedPanelBorder
        }}
      >
        <Image source={appIcon} style={{ width: 34, height: 34 }} resizeMode="contain" />
      </YStack>
      <ActivityIndicator size="large" />
      <Text color="$color" opacity={0.96} fontSize="$5" fontWeight="700">
        {message}
      </Text>
    </YStack>
  );
}
