import { useState } from 'react';
import { Platform } from 'react-native';
import { useRouter } from 'expo-router';
import { Input, Text, XStack, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { Card } from '@/shared/ui/Card';
import { AppMenu } from '@/shared/ui/AppMenu';

export default function SearchScreen() {
  const router = useRouter();
  const [query, setQuery] = useState('');

  return (
    <YStack flex={1} backgroundColor="$background" paddingHorizontal="$4" paddingVertical="$4" gap="$4">
      <TopBar
        title={<BrandLogo onPress={() => router.push('/devices')} />}
        subtitle="Search devices and telemetry"
        right={<AppMenu />}
        titleFlex={3}
        rightFlex={1}
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
  );
}
