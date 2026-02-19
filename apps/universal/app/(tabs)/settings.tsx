import { useRouter } from 'expo-router';
import { Animated } from 'react-native';
import { Button, Text, XStack, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { Card } from '@/shared/ui/Card';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { AppMenu } from '@/shared/ui/AppMenu';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { useChartPrefs } from '@/shared/ui/chartPrefs';

export default function SettingsScreen() {
  const router = useRouter();
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);
  const trendChartStyle = useChartPrefs((s) => s.trendChartStyle);
  const setTrendChartStyle = useChartPrefs((s) => s.setTrendChartStyle);

  return (
    <Animated.View style={containerStyle}>
      <YStack
        flex={1}
        backgroundColor="$background"
        paddingHorizontal="$4"
        paddingVertical="$4"
        gap="$4"
      >
      <TopBar
        left={<CloseToHomeButton onClose={closeToHome} />}
        title={<BrandLogo onPress={() => router.push('/devices')} />}
        subtitle="Configuration and diagnostics"
        titleFlex={3}
        rightFlex={1}
        right={<AppMenu />}
      />
      <Card gap="$2">
        <Text fontSize="$5" fontWeight="700">
          Chart Style
        </Text>
        <Text opacity={0.75}>
          Choose how trend charts render across the app.
        </Text>
        <XStack gap="$2" marginTop="$2">
          <Button
            size="$4"
            borderRadius="$5"
            borderWidth={1}
            borderColor={
              trendChartStyle === 'ascii' ? 'rgba(255,159,10,0.55)' : 'rgba(120,120,128,0.30)'
            }
            backgroundColor={
              trendChartStyle === 'ascii' ? 'rgba(255,159,10,0.15)' : 'rgba(120,120,128,0.10)'
            }
            onPress={() => setTrendChartStyle('ascii')}
          >
            ASCII
          </Button>
          <Button
            size="$4"
            borderRadius="$5"
            borderWidth={1}
            borderColor={
              trendChartStyle === 'premium' ? 'rgba(255,159,10,0.55)' : 'rgba(120,120,128,0.30)'
            }
            backgroundColor={
              trendChartStyle === 'premium'
                ? 'rgba(255,159,10,0.15)'
                : 'rgba(120,120,128,0.10)'
            }
            onPress={() => setTrendChartStyle('premium')}
          >
            Premium
          </Button>
        </XStack>
      </Card>
      <Card gap="$2">
        <Text fontSize="$5" fontWeight="700">
          API Endpoints
        </Text>
        <Text opacity={0.75}>Set EXPO_PUBLIC_API_URL and EXPO_PUBLIC_WS_URL in your environment.</Text>
      </Card>
      </YStack>
    </Animated.View>
  );
}
