import { useState } from 'react';
import { Animated, Platform, useWindowDimensions } from 'react-native';
import { useRouter } from 'expo-router';
import { Input, Text, XStack, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { Card } from '@/shared/ui/Card';
import { AppMenu } from '@/shared/ui/AppMenu';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';

export default function SearchScreen() {
  const router = useRouter();
  const [query, setQuery] = useState('');
  const { width } = useWindowDimensions();
  const compactHeader = width < 430;
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);

  return (
    <Animated.View style={containerStyle}>
      <YStack flex={1} backgroundColor="$background" paddingHorizontal="$4" paddingVertical="$4" gap="$4">
      <TopBar
        left={<CloseToHomeButton onClose={closeToHome} />}
        title={<BrandLogo compact={compactHeader} dense onPress={() => router.push('/devices')} />}
        subtitle="Search devices and telemetry"
        right={
          <YStack alignItems="flex-end">
            <AppMenu />
          </YStack>
        }
        titleFlex={compactHeader ? 1 : 3}
        rightFlex={compactHeader ? 0 : 1}
      />
      <Card gap="$3">
        <XStack alignItems="center" gap="$2">
          <Input
            flex={1}
            value={query}
            onChangeText={setQuery}
            placeholder="Search"
            size="$5"
            minHeight={56}
            paddingHorizontal={16}
            placeholderTextColor="#a8adb8"
            style={Platform.OS === 'web' ? ({ height: '2em' } as any) : undefined}
          />
          <XStack
            width={56}
            minHeight={56}
            alignItems="center"
            justifyContent="center"
            borderWidth={1}
            borderColor="rgba(120,120,128,0.3)"
            borderRadius={24}
          >
            <Text fontSize="$9" style={Platform.OS === 'web' ? ({ fontSize: '2.4em' } as any) : undefined}>
              ⌕
            </Text>
          </XStack>
        </XStack>
      </Card>
      </YStack>
    </Animated.View>
  );
}
