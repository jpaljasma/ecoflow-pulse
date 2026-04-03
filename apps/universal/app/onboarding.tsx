import { useMemo } from 'react';
import { Redirect, useRouter } from 'expo-router';
import { Animated, ScrollView } from 'react-native';
import { Button, Text, XStack, YStack } from 'tamagui';
import { useRequireAuth } from '@/features/auth/useRequireAuth';
import { useCurrentUser } from '@/features/profile/hooks';
import { resolveUserDisplayName } from '@/features/profile/model';
import { useAuthSession } from '@/features/auth/hooks';
import { AppMenu } from '@/shared/ui/AppMenu';
import { BreadcrumbTrail } from '@/shared/ui/BreadcrumbTrail';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { Card } from '@/shared/ui/Card';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { TopBar } from '@/shared/ui/TopBar';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { usePageLayoutMetrics } from '@/shared/ui/navigationShell';

const ONBOARDING_STEPS = [
  {
    title: 'Confirm your account',
    summary: 'We have your profile. Next we will verify the essentials before device setup starts.',
    status: 'active' as const
  },
  {
    title: 'Connect provider access',
    summary: 'This step will guide social account verification and provider credential linking.',
    status: 'upcoming' as const
  },
  {
    title: 'Discover and authorize devices',
    summary: 'We will find devices, confirm ownership, and apply device-level access rules.',
    status: 'upcoming' as const
  },
  {
    title: 'Personalize time and weather',
    summary: 'Timezone and weather-location consent will shape local-day views and future forecasts.',
    status: 'upcoming' as const
  }
];

export default function OnboardingScreen() {
  const router = useRouter();
  const semantics = useThemeSemantics();
  const { horizontalPadding, isSidebarMode, layoutMaxWidth } = usePageLayoutMetrics();
  const { authReady, allowed, waiting } = useRequireAuth();
  const { token, authKey } = useAuthSession();
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);
  const currentUserQuery = useCurrentUser({
    token,
    authKey,
    enabled: authReady && allowed
  });

  const greeting = useMemo(() => {
    const user = currentUserQuery.data?.user;
    if (!user) {
      return 'Welcome to Pulse';
    }
    return `Welcome, ${resolveUserDisplayName(user) || 'there'}`;
  }, [currentUserQuery.data?.user]);

  if (waiting || !allowed || currentUserQuery.isLoading) {
    return <BrandedLoadingState minHeight={260} message="Preparing onboarding…" />;
  }

  if ((currentUserQuery.data?.authorization.deviceCount ?? 0) > 0) {
    return <Redirect href="/devices" />;
  }

  return (
    <Animated.View style={containerStyle}>
      <YStack flex={1} backgroundColor="$background" paddingHorizontal={horizontalPadding} paddingVertical="$4" gap="$4">
        <TopBar
          left={isSidebarMode ? undefined : <CloseToHomeButton onClose={closeToHome} />}
          eyebrow={(
            <BreadcrumbTrail
              items={[
                { label: 'Home', href: '/devices', icon: 'home-outline', hideLabel: true },
                { label: 'Onboarding', current: true }
              ]}
            />
          )}
          title={<BrandLogo compact dense onPress={() => router.push('/devices')} />}
          subtitle="Onboarding wizard template"
          right={<AppMenu />}
        />
        <ScrollView contentContainerStyle={{ paddingBottom: 24, alignItems: 'center' }}>
          <YStack gap="$4" width="100%" maxWidth={Math.min(layoutMaxWidth, 960)} alignSelf="center">
            <Card
              gap="$3"
              style={{
                backgroundColor: semantics.energyCardBackground,
                borderColor: semantics.energyCardBorder
              }}
            >
              <Text fontSize="$9" fontWeight="800" letterSpacing={-0.5}>
                {greeting}
              </Text>
              <Text fontSize="$5" color="$colorMuted" lineHeight={28}>
                This is the onboarding shell we can wire up step by step. Right now it provides the route, layout, and step structure that post-login users can land on when they do not yet have devices.
              </Text>
            </Card>

            <XStack gap="$4" flexWrap="wrap" alignItems="stretch">
              <Card gap="$3" flex={2} minWidth={320}>
                <Text fontSize="$7" fontWeight="800">Wizard steps</Text>
                <YStack gap="$3">
                  {ONBOARDING_STEPS.map((step, index) => {
                    const active = step.status === 'active';
                    return (
                      <XStack
                        key={step.title}
                        gap="$3"
                        alignItems="flex-start"
                        padding="$3"
                        borderRadius="$3"
                        borderWidth={1}
                        style={{
                          borderColor: active ? semantics.energyCardBorder : semantics.mutedPanelBorder,
                          backgroundColor: active
                            ? semantics.energyCardBackground
                            : semantics.mutedPanelBackground
                        }}
                      >
                        <YStack
                          width={28}
                          height={28}
                          borderRadius={999}
                          alignItems="center"
                          justifyContent="center"
                          style={{
                            backgroundColor: active ? semantics.statusSuccess : semantics.mutedPanelBorder
                          }}
                        >
                          <Text color="$background" fontWeight="800">{index + 1}</Text>
                        </YStack>
                        <YStack gap="$1" flex={1}>
                          <Text fontSize="$5" fontWeight="700">{step.title}</Text>
                          <Text color="$colorMuted">{step.summary}</Text>
                        </YStack>
                      </XStack>
                    );
                  })}
                </YStack>
              </Card>

              <Card gap="$3" flex={1} minWidth={280}>
                <Text fontSize="$7" fontWeight="800">Next wiring</Text>
                <Text color="$colorMuted">
                  We will connect this flow to provider discovery, device authorization, and profile completion as the onboarding milestone gets defined.
                </Text>
                <YStack gap="$2">
                  <Text fontWeight="700">Current account</Text>
                  <Text color="$colorMuted">
                    {currentUserQuery.data?.user.email || 'Unknown email'}
                  </Text>
                  <Text color="$colorMuted">
                    Timezone: {currentUserQuery.data?.user.timezone || 'Not set'}
                  </Text>
                </YStack>
                <XStack gap="$3" flexWrap="wrap">
                  <Button size="$4" onPress={() => router.push('/profile')}>
                    Review profile
                  </Button>
                  <Button size="$4" onPress={() => router.push('/devices')}>
                    Go to devices
                  </Button>
                </XStack>
              </Card>
            </XStack>
          </YStack>
        </ScrollView>
      </YStack>
    </Animated.View>
  );
}
