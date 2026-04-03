import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'expo-router';
import { Image as ExpoImage } from 'expo-image';
import { Animated, ScrollView } from 'react-native';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Button, Text, XStack, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { useRequireAuth } from '@/features/auth/useRequireAuth';
import { buildEnergyRouteParams } from '@/features/energy/model';
import { useEnergySettingsStore } from '@/features/energy/store';
import { detectCurrentWeatherLocation } from '@/features/profile/location';
import { formatAuthMethodLabel, resolveUserDisplayName } from '@/features/profile/model';
import { TimezoneSelect } from '@/features/profile/TimezoneSelect';
import { useCurrentUser, useRefreshCurrentUserIdentity, useUpdateCurrentUser } from '@/features/profile/hooks';
import { detectCurrentTimeZone, resolveProfileTimezone } from '@/features/profile/timezone';
import { AppMenu } from '@/shared/ui/AppMenu';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';
import { BreadcrumbTrail } from '@/shared/ui/BreadcrumbTrail';
import { Card } from '@/shared/ui/Card';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { AppTextInput } from '@/shared/ui/AppTextInput';
import { TopBar } from '@/shared/ui/TopBar';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { usePageLayoutMetrics } from '@/shared/ui/navigationShell';
import { useThemeSemantics } from '@/shared/theme/semantic';

export default function ProfileScreen() {
  const router = useRouter();
  const { compactHeader, horizontalPadding, isSidebarMode, layoutMaxWidth } = usePageLayoutMetrics();
  const { authReady, allowed, waiting } = useRequireAuth();
  const { token, authKey } = useAuthSession();
  const semantics = useThemeSemantics();
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);
  const currentUserQuery = useCurrentUser({
    token,
    authKey,
    enabled: authReady && allowed
  });
  const updateCurrentUser = useUpdateCurrentUser({
    token,
    authKey
  });
  const refreshCurrentUserIdentity = useRefreshCurrentUserIdentity({
    token,
    authKey
  });
  const detectedTimeZone = detectCurrentTimeZone();
  const [displayName, setDisplayName] = useState('');
  const [timezone, setTimezone] = useState('');
  const [weatherLocationEnabled, setWeatherLocationEnabled] = useState(false);
  const [weatherLocation, setWeatherLocation] = useState<null | { label?: string; latitude: number; longitude: number }>(null);
  const [locationStatus, setLocationStatus] = useState('');
  const [identityRefreshAttempted, setIdentityRefreshAttempted] = useState(false);
  const gridPricePerKwh = useEnergySettingsStore((state) => state.gridPricePerKwh);
  const currency = useEnergySettingsStore((state) => state.currency);
  const setGridPricePerKwh = useEnergySettingsStore((state) => state.setGridPricePerKwh);
  const setCurrency = useEnergySettingsStore((state) => state.setCurrency);

  useEffect(() => {
    const user = currentUserQuery.data?.user;
    if (!user) {
      return;
    }
    setDisplayName(resolveUserDisplayName(user));
    setTimezone(resolveProfileTimezone(user.timezone, detectedTimeZone));
    setWeatherLocationEnabled(user.weatherLocationEnabled);
    setWeatherLocation(user.weatherLocation);
  }, [currentUserQuery.data?.user, detectedTimeZone]);

  useEffect(() => {
    setIdentityRefreshAttempted(false);
  }, [authKey, currentUserQuery.data?.user?.id]);

  useEffect(() => {
    const user = currentUserQuery.data?.user;
    if (!user || user.avatarUrl || identityRefreshAttempted || refreshCurrentUserIdentity.isPending) {
      return;
    }
    setIdentityRefreshAttempted(true);
    refreshCurrentUserIdentity.mutate();
  }, [currentUserQuery.data?.user, identityRefreshAttempted, refreshCurrentUserIdentity]);

  const dirty = useMemo(() => {
    const user = currentUserQuery.data?.user;
    if (!user) {
      return false;
    }
    return (
      displayName !== resolveUserDisplayName(user) ||
      timezone !== resolveProfileTimezone(user.timezone, detectedTimeZone) ||
      weatherLocationEnabled !== user.weatherLocationEnabled ||
      JSON.stringify(weatherLocation ?? null) !== JSON.stringify(user.weatherLocation ?? null)
    );
  }, [currentUserQuery.data?.user, detectedTimeZone, displayName, timezone, weatherLocation, weatherLocationEnabled]);

  if (waiting || !allowed) {
    return <BrandedLoadingState minHeight={260} message="Checking session…" />;
  }

  if (currentUserQuery.isLoading || !currentUserQuery.data) {
    return <BrandedLoadingState minHeight={260} message="Loading profile…" />;
  }

  const user = currentUserQuery.data.user;
  const authMethodLabel = formatAuthMethodLabel(user.authMethod);
  const preferredDisplayName = resolveUserDisplayName(user);
  return (
    <Animated.View style={containerStyle}>
      <YStack flex={1} backgroundColor="$background" paddingHorizontal={horizontalPadding} paddingVertical="$4" gap="$4">
        <TopBar
          left={isSidebarMode ? undefined : <CloseToHomeButton onClose={closeToHome} />}
          eyebrow={(
            <BreadcrumbTrail
              items={[
                {
                  label: 'Home',
                  href: '/(tabs)/devices',
                  icon: 'home-variant-outline',
                  hideLabel: true
                },
                {
                  label: 'Profile',
                  current: true
                }
              ]}
            />
          )}
          title="Profile"
          subtitle="Your Pulse profile and consent preferences"
          right={<AppMenu />}
        />
        <ScrollView
          contentContainerStyle={{ paddingBottom: 24, alignItems: 'center' }}
        >
          <YStack gap="$4" width="100%" maxWidth={Math.min(layoutMaxWidth, 920)} alignSelf="center">
            <Card gap="$3">
              <XStack justifyContent="space-between" alignItems="flex-start" gap="$3" flexWrap="wrap">
                <YStack gap="$1" flex={1} minWidth={220}>
                  <Text fontSize="$7" fontWeight="800">Profile</Text>
                  <Text color="$colorMuted">
                    Pulse profile details with social sign-in and local preferences.
                  </Text>
                </YStack>
                <XStack
                  alignItems="center"
                  gap="$3"
                  padding="$2"
                  borderWidth={1}
                  borderColor="$borderColor"
                  borderRadius="$5"
                  minWidth={220}
                  justifyContent="flex-end"
                  style={{ alignSelf: compactHeader ? 'stretch' : 'flex-start' }}
                >
                  <YStack alignItems="flex-end" flex={1} gap={2}>
                    <Text fontWeight="700">Signed in with {authMethodLabel}</Text>
                    <XStack alignItems="center" gap="$2">
                      <Text color="$colorMuted" numberOfLines={1}>
                        {user.email}
                      </Text>
                      {user.emailVerified ? (
                        <XStack
                          alignItems="center"
                          gap={6}
                          paddingHorizontal="$2"
                          paddingVertical="$1"
                          borderRadius={999}
                          style={{ backgroundColor: 'rgba(22, 163, 74, 0.14)' }}
                        >
                          <Text fontWeight="800" style={{ color: '#16a34a' }}>✓</Text>
                          <Text fontWeight="700" style={{ color: '#16a34a' }}>Verified</Text>
                        </XStack>
                      ) : null}
                    </XStack>
                  </YStack>
                  {user.avatarUrl ? (
                    <ExpoImage
                      source={{ uri: user.avatarUrl }}
                      style={{
                        width: 56,
                        height: 56,
                        borderRadius: 28,
                        backgroundColor: 'rgba(16, 120, 88, 0.08)',
                        borderWidth: 1,
                        borderColor: 'rgba(16, 120, 88, 0.18)'
                      }}
                      contentFit="cover"
                    />
                  ) : (
                    <XStack
                      width={56}
                      height={56}
                      borderRadius={28}
                      alignItems="center"
                      justifyContent="center"
                      backgroundColor="$background"
                      borderWidth={1}
                      borderColor="$borderColor"
                    >
                      <Text fontSize="$6" fontWeight="800">
                        {(preferredDisplayName || 'P').trim().charAt(0).toUpperCase()}
                      </Text>
                    </XStack>
                  )}
                </XStack>
              </XStack>
              <YStack gap="$2">
                <Text fontWeight="700">Display name</Text>
                <AppTextInput value={displayName} onChangeText={setDisplayName} placeholder="Display name" />
              </YStack>
              <YStack gap="$2">
                <Text fontWeight="700">Email</Text>
                <XStack gap="$2" alignItems="center">
                  <AppTextInput value={user.email} editable={false} opacity={0.72} flex={1} />
                  {user.emailVerified ? (
                    <Text fontWeight="800" style={{ color: '#16a34a' }}>✓</Text>
                  ) : null}
                </XStack>
              </YStack>
              <YStack gap="$2">
                <Text fontWeight="700">Authentication</Text>
                <AppTextInput value={authMethodLabel} editable={false} opacity={0.72} />
              </YStack>
              <YStack gap="$2">
                <Text fontWeight="700">Timezone</Text>
                <TimezoneSelect
                  value={timezone}
                  onChange={setTimezone}
                  suggestedValue={detectedTimeZone || undefined}
                />
                <Text color="$colorMuted">
                  Choose from the IANA timezone list. Suggested from this device: {detectedTimeZone || 'Unavailable'}.
                </Text>
              </YStack>
              <YStack gap="$2">
                <Text fontWeight="700">Local energy price</Text>
                <Text color="$colorMuted">
                  Used by the Energy dashboard for estimated value and AC input cost. Saved locally on this browser or device.
                </Text>
                <XStack gap="$3" flexWrap="wrap" alignItems="center">
                  <AppTextInput
                    compact
                    width={120}
                    value={gridPricePerKwh}
                    onChangeText={setGridPricePerKwh}
                    keyboardType="decimal-pad"
                    placeholder="0.30"
                    borderRadius={999}
                    borderWidth={1}
                  />
                  <Button
                    size="$3"
                    borderWidth={1}
                    onPress={() => setCurrency('USD')}
                    style={{
                      backgroundColor: currency === 'USD' ? semantics.periodActiveBackground : semantics.periodIdleBackground,
                      borderColor: currency === 'USD' ? semantics.periodActiveBorder : semantics.periodIdleBorder,
                      color: currency === 'USD' ? semantics.periodActiveText : semantics.periodIdleText
                    }}
                  >
                    USD
                  </Button>
                  <Button
                    size="$3"
                    borderWidth={1}
                    onPress={() => setCurrency('CAD')}
                    style={{
                      backgroundColor: currency === 'CAD' ? semantics.periodActiveBackground : semantics.periodIdleBackground,
                      borderColor: currency === 'CAD' ? semantics.periodActiveBorder : semantics.periodIdleBorder,
                      color: currency === 'CAD' ? semantics.periodActiveText : semantics.periodIdleText
                    }}
                  >
                    CAD
                  </Button>
                  <Button
                    size="$3"
                    borderWidth={1}
                    onPress={() => setCurrency('EUR')}
                    style={{
                      backgroundColor: currency === 'EUR' ? semantics.periodActiveBackground : semantics.periodIdleBackground,
                      borderColor: currency === 'EUR' ? semantics.periodActiveBorder : semantics.periodIdleBorder,
                      color: currency === 'EUR' ? semantics.periodActiveText : semantics.periodIdleText
                    }}
                  >
                    EUR
                  </Button>
                </XStack>
              </YStack>
            </Card>

            <Card gap="$3">
              <Text fontSize="$7" fontWeight="800">Weather location consent</Text>
              <Text color="$colorMuted">
                Pulse only stores a one-shot foreground location when you ask it to, so forecast-aware features can use the right place and local time.
              </Text>
              <XStack gap="$3" flexWrap="wrap">
                <Button
                  size="$4"
                  onPress={() => {
                    setWeatherLocationEnabled((value) => !value);
                    if (weatherLocationEnabled) {
                      setWeatherLocation(null);
                      setLocationStatus('');
                    }
                  }}
                >
                  {weatherLocationEnabled ? 'Consent enabled' : 'Enable consent'}
                </Button>
                <Button
                  size="$4"
                  onPress={() => {
                    void (async () => {
                      setLocationStatus('Detecting current location…');
                      try {
                        const detected = await detectCurrentWeatherLocation();
                        setWeatherLocationEnabled(true);
                        setWeatherLocation(detected);
                        setLocationStatus(`Using ${detected.label}`);
                      } catch (error) {
                        setLocationStatus(error instanceof Error ? error.message : 'Location detection failed');
                      }
                    })();
                  }}
                  disabled={!weatherLocationEnabled && updateCurrentUser.isPending}
                >
                  Detect current location
                </Button>
                <Button
                  size="$4"
                  onPress={() => {
                    setWeatherLocationEnabled(false);
                    setWeatherLocation(null);
                    setLocationStatus('Weather location cleared');
                  }}
                >
                  Revoke consent
                </Button>
              </XStack>
              {weatherLocation ? (
                <Text color="$colorMuted">
                  {weatherLocation.label || 'Saved location'} · {weatherLocation.latitude.toFixed(3)}, {weatherLocation.longitude.toFixed(3)}
                </Text>
              ) : (
                <Text color="$colorMuted">No weather location stored.</Text>
              )}
              {locationStatus ? <Text color="$colorMuted">{locationStatus}</Text> : null}
            </Card>

            <Card gap="$3">
              <XStack justifyContent="space-between" alignItems="flex-start" gap="$3" flexWrap="wrap">
                <YStack gap="$1" flex={1} minWidth={220}>
                  <Text fontSize="$7" fontWeight="800">Solar moved to Energy</Text>
                  <Text color="$colorMuted">
                    Solar forecast, weather verification, and the larger Energy Impact pane now live under Energy so site and device deep links share one consistent flow.
                  </Text>
                </YStack>
                <Button
                  size="$4"
                  borderRadius={999}
                  borderWidth={1}
                  paddingHorizontal="$4"
                  minHeight={42}
                  style={{
                    backgroundColor: semantics.actionBackground,
                    borderColor: semantics.actionBorder
                  }}
                  onPress={() => {
                    router.push({
                      pathname: '/(tabs)/energy',
                      params: buildEnergyRouteParams({
                        scope: 'all',
                        preset: 'today',
                        timezone: timezone || detectedTimeZone || 'UTC',
                        includeComparison: true,
                        panel: 'solar'
                      })
                    });
                  }}
                >
                  <XStack alignItems="center" gap="$2">
                    <MaterialCommunityIcons name="weather-sunny" size={18} color={semantics.actionText} />
                    <Text style={{ color: semantics.actionText }} fontWeight="700">
                      Open Solar
                    </Text>
                  </XStack>
                </Button>
              </XStack>
            </Card>

            {currentUserQuery.isError ? (
              <Card gap="$2">
                <Text fontSize="$5" fontWeight="700">Failed to load profile</Text>
                <Text color="$colorMuted">{String(currentUserQuery.error)}</Text>
              </Card>
            ) : null}

            {updateCurrentUser.isError ? (
              <Card gap="$2">
                <Text fontSize="$5" fontWeight="700">Failed to save profile</Text>
                <Text color="$colorMuted">{String(updateCurrentUser.error)}</Text>
              </Card>
            ) : null}

            <XStack justifyContent="flex-end" gap="$3" flexWrap="wrap">
              <Button
                size="$5"
                onPress={() => {
                  const user = currentUserQuery.data?.user;
                  if (!user) {
                    return;
                  }
                  setDisplayName(resolveUserDisplayName(user));
                  setTimezone(resolveProfileTimezone(user.timezone, detectedTimeZone));
                  setWeatherLocationEnabled(user.weatherLocationEnabled);
                  setWeatherLocation(user.weatherLocation);
                  setLocationStatus('');
                }}
                disabled={!dirty}
              >
                Reset
              </Button>
              <Button
                size="$5"
                onPress={() => {
                  void updateCurrentUser.mutateAsync({
                    displayName,
                    timezone,
                    weatherLocationEnabled,
                    weatherLocation: weatherLocationEnabled ? weatherLocation : null
                  });
                }}
                disabled={!dirty || updateCurrentUser.isPending}
              >
                {updateCurrentUser.isPending ? 'Saving…' : 'Save profile'}
              </Button>
            </XStack>
          </YStack>
        </ScrollView>
      </YStack>
    </Animated.View>
  );
}
