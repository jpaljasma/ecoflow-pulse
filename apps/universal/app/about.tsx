import { useRouter } from 'expo-router';
import { Animated } from 'react-native';
import { Text, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { Card } from '@/shared/ui/Card';
import { AppMenu } from '@/shared/ui/AppMenu';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';

export default function AboutScreen() {
  const router = useRouter();
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);

  return (
    <Animated.View style={containerStyle}>
      <YStack flex={1} backgroundColor="$background" paddingHorizontal="$4" paddingVertical="$4" gap="$4">
      <TopBar
        left={<CloseToHomeButton onClose={closeToHome} />}
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
    </Animated.View>
  );
}
