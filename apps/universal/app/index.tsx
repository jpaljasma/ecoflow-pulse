import { Redirect, router } from 'expo-router';
import { ScrollView, useWindowDimensions } from 'react-native';
import { Button, Text, XStack, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { resolvePostLoginTarget } from '@/features/auth/useReturnTo';
import { useCurrentUser } from '@/features/profile/hooks';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';
import { Card } from '@/shared/ui/Card';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { useThemeSemantics } from '@/shared/theme/semantic';

export default function WelcomeScreen() {
  const { width } = useWindowDimensions();
  const compact = width < 430;
  const semantics = useThemeSemantics();
  const { authReady, authKey, sessionValid, token } = useAuthSession();
  const currentUserQuery = useCurrentUser({
    token,
    authKey,
    enabled: authReady && sessionValid
  });
  const postLoginTarget = resolvePostLoginTarget(
    null,
    currentUserQuery.data?.authorization.deviceCount
  );

  if (sessionValid && (currentUserQuery.isLoading || !authReady)) {
    return <BrandedLoadingState minHeight={260} message="Loading your workspace…" />;
  }

  if (sessionValid && currentUserQuery.data) {
    return <Redirect href={postLoginTarget} />;
  }

  return (
    <YStack flex={1} backgroundColor="$background" padding="$4">
      <ScrollView contentContainerStyle={{ flexGrow: 1, justifyContent: 'center' }}>
        <YStack gap="$5" alignItems="center" width="100%" maxWidth={840} alignSelf="center">
          <BrandLogo compact={compact} />
          <Card
            width="100%"
            gap="$4"
            style={{
              backgroundColor: semantics.energyCardBackground,
              borderColor: semantics.energyCardBorder
            }}
          >
            <YStack gap="$2">
              <Text fontSize="$10" fontWeight="800" letterSpacing={-0.5}>
                Pulse for always-on energy visibility
              </Text>
              <Text fontSize="$5" color="$colorMuted" lineHeight={28}>
                Monitor your energy systems, compare solar performance, and keep profile and device access locked to your account.
              </Text>
            </YStack>
            <YStack gap="$2">
              <Text fontSize="$5" fontWeight="700">What you get</Text>
              <Text color="$colorMuted">Real-time device views, energy comparisons, and profile-driven local time behavior.</Text>
              <Text color="$colorMuted">Secure Google sign-in through Keycloak with device authorization enforced server-side.</Text>
              <Text color="$colorMuted">Optional weather-location consent for future forecast-aware experiences.</Text>
            </YStack>
            <XStack gap="$3" flexWrap="wrap">
              <Button size="$5" onPress={() => router.push('/login')}>
                Log in
              </Button>
              <Button size="$5" onPress={() => router.push('/about')}>
                Learn more
              </Button>
            </XStack>
          </Card>
        </YStack>
      </ScrollView>
    </YStack>
  );
}
