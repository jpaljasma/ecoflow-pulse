import { useState } from 'react';
import { router } from 'expo-router';
import { Platform, ScrollView } from 'react-native';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Button, Text, XStack, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { LogoutButton } from '@/features/auth/LogoutButton';
import { useCurrentUser } from '@/features/profile/hooks';
import { buildEnergyRouteParams, detectLocalTimezone } from '@/features/energy/model';
import { HeaderWeatherButton } from '@/features/weather/HeaderWeatherButton';
import { useProfileWeather } from '@/features/weather/hooks';
import { resolveProfileWeatherState } from '@/features/weather/model';
import { ConnectionProfileHint, ConnectionProfileSwitcher } from '@/shared/ui/ConnectionProfileSwitcher';
import { AppTextInput } from '@/shared/ui/AppTextInput';
import { Sheet } from '@/shared/ui/Sheet';
import { PulseMark } from '@/shared/ui/PulseMark';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';

function describeQueryError(error: unknown): string | undefined {
  if (!error) {
    return undefined;
  }
  return error instanceof Error ? error.message : String(error);
}

export function AppMenu({
  weatherScope = 'all',
  weatherDeviceId
}: {
  weatherScope?: 'all' | 'device';
  weatherDeviceId?: string;
} = {}) {
  const [open, setOpen] = useState(false);
  const [searchText, setSearchText] = useState('');
  const { token, authKey, authReady } = useAuthSession();
  const semantics = useThemeSemantics();
  const { spec } = useAppTheme();
  const { contentWidth } = useNavigationShellMetrics();
  const showHeaderWeather = contentWidth >= 560;
  const currentUserQuery = useCurrentUser({
    token,
    authKey,
    enabled: authReady && Boolean(token)
  });
  const resolvedWeatherState = resolveProfileWeatherState(currentUserQuery.data?.user);
  const profileWeather = useProfileWeather({
    token,
    authKey,
    locationKey: resolvedWeatherState.locationKey,
    enabled:
      authReady &&
      Boolean(token) &&
      resolvedWeatherState.enabled &&
      (weatherScope === 'all' || Boolean(weatherDeviceId)),
    scope: weatherScope,
    deviceId: weatherScope === 'device' ? weatherDeviceId : undefined
  });
  const showConfigureWeather =
    Boolean(currentUserQuery.data?.user) && !resolvedWeatherState.enabled;
  const headerWeatherError =
    describeQueryError(profileWeather.forecastQuery.error) ??
    describeQueryError(profileWeather.solarOutlookQuery.error);
  const headerWeatherLoading =
    currentUserQuery.isLoading ||
    profileWeather.forecastQuery.isLoading ||
    profileWeather.solarOutlookQuery.isLoading;
  const headerSolarScope = weatherScope === 'device' && weatherDeviceId ? 'device' : 'all';
  const profileTimezone =
    typeof currentUserQuery.data?.user.timezone === 'string' && currentUserQuery.data.user.timezone
      ? currentUserQuery.data.user.timezone
      : undefined;
  const headerSolarRouteParams = buildEnergyRouteParams({
    scope: headerSolarScope,
    deviceId: headerSolarScope === 'device' ? weatherDeviceId : undefined,
    preset: 'today',
    timezone: profileTimezone ?? detectLocalTimezone(),
    includeComparison: true,
    panel: 'solar'
  });

  return (
    <>
      <XStack gap="$2" alignItems="flex-start">
        {showHeaderWeather ? (
          <HeaderWeatherButton
            forecast={profileWeather.forecastQuery.data?.forecast}
            solarOutlook={profileWeather.solarOutlook}
            showConfigure={showConfigureWeather}
            isLoading={headerWeatherLoading}
            errorText={headerWeatherError}
            onPress={() =>
              router.push({
                pathname: '/(tabs)/energy',
                params: headerSolarRouteParams
              })
            }
          />
        ) : null}

        <Button
          size="$4"
          onPress={() => setOpen(true)}
          width={46}
          height={46}
          minWidth={46}
          paddingHorizontal="$0"
          paddingVertical="$0"
          alignSelf="flex-start"
          borderWidth={1}
          borderRadius={23}
          alignItems="center"
          justifyContent="center"
          pressStyle={{ opacity: 0.85 }}
          style={{
            backgroundColor: semantics.mutedPanelBackground,
            borderColor: semantics.mutedPanelBorder,
            ...(Platform.OS === 'web' ? ({ paddingTop: 0, paddingBottom: 0 } as any) : {})
          }}
          aria-label="Open menu"
        >
          <PulseMark size={22} />
        </Button>
      </XStack>

      <Sheet open={open} onOpenChange={setOpen} title="Menu">
        <YStack minHeight={360} maxHeight={520} justifyContent="space-between">
          <ScrollView showsVerticalScrollIndicator>
            <YStack gap="$3" paddingRight="$1">
              <Button
                size="$5"
                justifyContent="flex-start"
                onPress={() => {
                  setOpen(false);
                  router.push('/profile');
                }}
              >
                Profile
              </Button>
              <Button
                size="$5"
                justifyContent="flex-start"
                onPress={() => {
                  setOpen(false);
                  router.push('/devices');
                }}
              >
                Devices
              </Button>
              <Button
                size="$5"
                justifyContent="flex-start"
                onPress={() => {
                  setOpen(false);
                  router.push('/(tabs)/energy');
                }}
              >
                Energy
              </Button>
              <Button
                size="$5"
                justifyContent="flex-start"
                onPress={() => {
                  setOpen(false);
                  router.push('/settings');
                }}
              >
                Settings
              </Button>
              <Button
                size="$5"
                justifyContent="flex-start"
                onPress={() => {
                  setOpen(false);
                  router.push('/(tabs)/search');
                }}
              >
                Search
              </Button>
              <Button
                size="$5"
                justifyContent="flex-start"
                onPress={() => {
                  setOpen(false);
                  router.push('/(tabs)/about');
                }}
              >
                About
              </Button>
              <LogoutButton
                onComplete={() => {
                  setOpen(false);
                }}
              />
              <YStack
                gap="$3"
                marginTop="$2"
                padding="$3"
                borderRadius="$4"
                style={{
                  backgroundColor: semantics.sectionBackgroundStrong,
                  borderColor: semantics.sectionBorder
                }}
                borderWidth={1}
              >
                <YStack gap="$2">
                  <XStack alignItems="center" gap="$2">
                    <MaterialCommunityIcons
                      name="server-network-outline"
                      size={18}
                      color={semantics.subtleStrongText}
                    />
                    <Text fontSize="$4" fontWeight="800" color="$color">
                      Data source
                    </Text>
                  </XStack>
                  <Text fontSize="$2" color="$colorMuted">
                    Switch this frontend between local k3d and cloud without leaving the menu.
                  </Text>
                </YStack>
                <ConnectionProfileSwitcher variant="compact" />
                <ConnectionProfileHint />
                <Button
                  chromeless
                  padding="$0"
                  height="auto"
                  justifyContent="flex-start"
                  onPress={() => {
                    setOpen(false);
                    router.push('/settings');
                  }}
                >
                  Open full connection settings
                </Button>
              </YStack>
            </YStack>
          </ScrollView>

          <YStack gap="$3" paddingTop="$3">
            <XStack height={1} style={{ backgroundColor: semantics.railBorder }} />
            <XStack alignItems="center" gap="$2" width="100%" maxWidth={360}>
              <AppTextInput
                flex={1}
                value={searchText}
                onChangeText={setSearchText}
                placeholder="Search"
                placeholderTextColor={spec.colors.colorMuted}
              />
              <XStack
                width={52}
                minHeight={52}
                alignItems="center"
                justifyContent="center"
                borderWidth={1}
                borderRadius={20}
                style={{
                  backgroundColor: semantics.sectionBackgroundStrong,
                  borderColor: semantics.sectionBorder
                }}
              >
                <MaterialCommunityIcons name="magnify" size={22} color={semantics.subtleStrongText} />
              </XStack>
            </XStack>
          </YStack>
        </YStack>
      </Sheet>
    </>
  );
}
