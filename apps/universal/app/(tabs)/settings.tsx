import { useRouter } from 'expo-router';
import { Animated, ScrollView, useWindowDimensions } from 'react-native';
import { Text, YStack } from 'tamagui';
import { KeycloakPkceCard } from '@/features/auth/KeycloakPkceCard';
import { TopBar } from '@/shared/ui/TopBar';
import { Card } from '@/shared/ui/Card';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { AppMenu } from '@/shared/ui/AppMenu';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';

export default function SettingsScreen() {
  const router = useRouter();
  const { width } = useWindowDimensions();
  const compactHeader = width < 430;
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);

  return (
    <Animated.View style={containerStyle} testID="screen-settings">
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
        <ScrollView style={{ flex: 1 }} contentContainerStyle={{ paddingBottom: 16 }} showsVerticalScrollIndicator>
          <YStack gap="$4">
            <KeycloakPkceCard />
            <Card gap="$2">
              <Text fontSize="$5" fontWeight="700" testID="settings-api-endpoints">
                API Endpoints
              </Text>
              <Text opacity={0.75}>
                Set EXPO_PUBLIC_API_URL for non-default routing. EXPO_PUBLIC_WS_URL is optional and
                defaults to API /ws.
              </Text>
            </Card>
          </YStack>
        </ScrollView>
      </YStack>
    </Animated.View>
  );
}
