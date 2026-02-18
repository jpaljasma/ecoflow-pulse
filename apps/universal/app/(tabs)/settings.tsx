import { useRouter } from 'expo-router';
import { Text, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { Card } from '@/shared/ui/Card';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { AppMenu } from '@/shared/ui/AppMenu';

export default function SettingsScreen() {
  const router = useRouter();
  return (
    <YStack
      flex={1}
      backgroundColor="$background"
      paddingHorizontal="$4"
      paddingVertical="$4"
      gap="$4"
    >
      <TopBar
        title={<BrandLogo onPress={() => router.push('/devices')} />}
        subtitle="Configuration and diagnostics"
        titleFlex={3}
        rightFlex={1}
        right={<AppMenu />}
      />
      <Card gap="$2">
        <Text fontSize="$5" fontWeight="700">
          API Endpoints
        </Text>
        <Text opacity={0.75}>Set EXPO_PUBLIC_API_URL and EXPO_PUBLIC_WS_URL in your environment.</Text>
      </Card>
    </YStack>
  );
}
