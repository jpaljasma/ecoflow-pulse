import { useRouter } from 'expo-router';
import { Animated, useWindowDimensions } from 'react-native';
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
  const { width } = useWindowDimensions();
  const compactHeader = width < 430;
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
        title={<BrandLogo compact={compactHeader} dense onPress={() => router.push('/devices')} />}
        subtitle="Configuration and diagnostics"
        titleFlex={compactHeader ? 1 : 3}
        rightFlex={compactHeader ? 0 : 1}
        right={
          <YStack alignItems="flex-end">
            <AppMenu />
          </YStack>
        }
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
            size="$3"
            minHeight={40}
            paddingHorizontal="$4"
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
            <Text fontSize="$3" lineHeight={18} fontWeight="600">
              ASCII
            </Text>
          </Button>
          <Button
            size="$3"
            minHeight={40}
            paddingHorizontal="$4"
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
            <Text fontSize="$3" lineHeight={18} fontWeight="600">
              Premium
            </Text>
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
