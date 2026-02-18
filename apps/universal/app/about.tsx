import { useRouter } from 'expo-router';
import { Text, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { Card } from '@/shared/ui/Card';
import { AppMenu } from '@/shared/ui/AppMenu';

export default function AboutScreen() {
  const router = useRouter();
  return (
    <YStack flex={1} backgroundColor="$background" paddingHorizontal="$4" paddingVertical="$4" gap="$4">
      <TopBar
        title={<BrandLogo onPress={() => router.push('/devices')} />}
        subtitle="About EcoFlow Pulse"
        right={<AppMenu />}
        titleFlex={3}
        rightFlex={1}
      />
      <Card gap="$2">
        <Text fontSize="$6" fontWeight="700">
          EcoFlow Pulse
        </Text>
        <Text opacity={0.8}>Universal telemetry dashboard for EcoFlow devices.</Text>
      </Card>
    </YStack>
  );
}
