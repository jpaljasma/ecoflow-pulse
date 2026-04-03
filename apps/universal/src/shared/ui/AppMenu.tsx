import { useState } from 'react';
import { router } from 'expo-router';
import { Image, Platform, ScrollView } from 'react-native';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Button, XStack, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { LogoutButton } from '@/features/auth/LogoutButton';
import { useCurrentUser } from '@/features/profile/hooks';
import { HeaderWeatherButton } from '@/features/weather/HeaderWeatherButton';
import { useProfileWeather } from '@/features/weather/hooks';
import { resolveProfileWeatherState } from '@/features/weather/model';
import { AppTextInput } from '@/shared/ui/AppTextInput';
import { Sheet } from '@/shared/ui/Sheet';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import appIcon from '../../../assets/icon.png';

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

  return (
    <>
      <XStack gap="$2" alignItems="flex-start">
        <HeaderWeatherButton
          forecast={profileWeather.forecastQuery.data?.forecast}
          solarOutlook={profileWeather.solarOutlook}
          showConfigure={showConfigureWeather}
          isLoading={headerWeatherLoading}
          errorText={headerWeatherError}
          onPress={() => router.push('/profile')}
        />

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
          <XStack width={26} height={26} alignItems="center" justifyContent="center">
            <Image source={appIcon} style={{ width: 20, height: 20, borderRadius: 6 }} resizeMode="contain" />
          </XStack>
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
