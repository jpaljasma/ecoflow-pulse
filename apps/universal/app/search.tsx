import { useState } from 'react';
import { Animated, ScrollView, useWindowDimensions } from 'react-native';
import { useRouter } from 'expo-router';
import { AppMenu } from '@/shared/ui/AppMenu';
import { AppTextInput } from '@/shared/ui/AppTextInput';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { Card } from '@/shared/ui/Card';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { TopBar } from '@/shared/ui/TopBar';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { Text, XStack, YStack } from 'tamagui';

export default function SearchScreen() {
  const router = useRouter();
  const [query, setQuery] = useState('');
  const { width } = useWindowDimensions();
  const compactHeader = width < 430;
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);

  return (
    <Animated.View style={containerStyle} testID="screen-search">
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
        <ScrollView style={{ flex: 1 }} contentContainerStyle={{ paddingBottom: 16 }} showsVerticalScrollIndicator>
          <YStack gap="$4">
            <Card gap="$3">
              <XStack alignItems="center" gap="$2">
                <AppTextInput
                  flex={1}
                  value={query}
                  onChangeText={setQuery}
                  placeholder="Search"
                  placeholderTextColor="#a8adb8"
                />
                <XStack
                  width={52}
                  minHeight={52}
                  alignItems="center"
                  justifyContent="center"
                  borderWidth={1}
                  borderColor="rgba(120,120,128,0.3)"
                  borderRadius={20}
                >
                  <Text fontSize="$8">
                    ⌕
                  </Text>
                </XStack>
              </XStack>
            </Card>
          </YStack>
        </ScrollView>
      </YStack>
    </Animated.View>
  );
}
