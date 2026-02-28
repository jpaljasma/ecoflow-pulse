import { ActivityIndicator, Animated, Image, Platform, useColorScheme, useWindowDimensions } from 'react-native';
import { useEffect, useMemo, useRef } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { TopBar } from '@/shared/ui/TopBar';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { AppMenu } from '@/shared/ui/AppMenu';
import { useDevices } from '@/features/devices/hooks';
import {
  useTelemetryConnectionStatus,
  useTelemetrySubscription
} from '@/features/telemetry/hooks';
import { SummaryPanel } from '@/features/devices/SummaryPanel';
import { DeviceList } from '@/features/devices/DeviceList';
import { formatConnectionStatus } from '@/features/telemetry/status';
import { getBundledBrandMark } from '@/shared/assets/brandBundled';

function statusDotColor(status: string): string {
  if (status === 'connected') return '#30d158';
  if (status === 'auth_required') return '#ff9f0a';
  if (status === 'reconnecting' || status === 'connecting') return '#ff9f0a';
  return '#ff453a';
}

export default function DevicesScreen() {
  const { width } = useWindowDimensions();
  const scheme = useColorScheme();
  const loadingMark = getBundledBrandMark(scheme === 'dark' ? 'dark' : 'light');
  const compactHeader = width < 430;
  const { authConfigured, authReady, authKey, sessionValid, token } = useAuthSession();
  const devicesQuery = useDevices({
    token,
    authKey,
    enabled: authReady && (!authConfigured || sessionValid)
  });
  const deviceIds = useMemo(
    () => devicesQuery.data?.devices.map((d) => d.id) ?? [],
    [devicesQuery.data?.devices]
  );
  useTelemetrySubscription(deviceIds);
  const connectionStatus = useTelemetryConnectionStatus();
  const pulse = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    const anim = Animated.loop(
      Animated.sequence([
        Animated.timing(pulse, {
          toValue: 1,
          duration: 900,
          useNativeDriver: Platform.OS !== 'web'
        }),
        Animated.timing(pulse, {
          toValue: 0,
          duration: 900,
          useNativeDriver: Platform.OS !== 'web'
        })
      ])
    );
    anim.start();
    return () => {
      anim.stop();
    };
  }, [pulse]);

  const dotScale = pulse.interpolate({
    inputRange: [0, 1],
    outputRange: [1, 1.24]
  });
  const dotOpacity = pulse.interpolate({
    inputRange: [0, 1],
    outputRange: [0.75, 1]
  });

  return (
    <YStack flex={1} backgroundColor="$background">
      <TopBar
        title={
          <BrandLogo
            compact={compactHeader}
            onPress={() => {
              void devicesQuery.refetch();
            }}
          />
        }
        subtitle={
          <Text fontSize={10} opacity={0.5} numberOfLines={1}>
            {authConfigured && !authReady
              ? 'Restoring session…'
              : formatConnectionStatus(connectionStatus)}
          </Text>
        }
        titleFlex={compactHeader ? 1 : 3}
        rightFlex={compactHeader ? 0 : 1}
        right={
          <XStack alignItems="center" gap="$1" marginLeft="$1">
            <Animated.View
              style={{
                width: compactHeader ? 12 : 14,
                height: compactHeader ? 12 : 14,
                borderRadius: compactHeader ? 6 : 7,
                marginTop: compactHeader ? 2 : 1,
                backgroundColor: statusDotColor(connectionStatus),
                transform: [{ scale: dotScale }],
                opacity: dotOpacity
              }}
            />
            <AppMenu />
          </XStack>
        }
      />

      {devicesQuery.isLoading ? (
        <YStack flex={1} alignItems="center" justifyContent="center" gap="$4">
          <YStack
            width={68}
            height={68}
            borderRadius="$4"
            alignItems="center"
            justifyContent="center"
            backgroundColor="rgba(120,120,128,0.12)"
          >
            <Image source={loadingMark} style={{ width: 34, height: 34 }} resizeMode="contain" />
          </YStack>
          <ActivityIndicator size="large" />
          <Text opacity={0.82} fontSize="$5" fontWeight="600">
            Loading...
          </Text>
        </YStack>
      ) : null}

      {devicesQuery.isError ? (
        <YStack paddingHorizontal="$4" paddingVertical="$6" gap="$2">
          <Text fontSize="$5" fontWeight="700">
            Failed to load devices
          </Text>
          <Text opacity={0.75}>{String(devicesQuery.error)}</Text>
        </YStack>
      ) : null}

      {!devicesQuery.data && !devicesQuery.isLoading && !devicesQuery.isError && authConfigured && authReady && !sessionValid ? (
        <YStack paddingHorizontal="$4" paddingVertical="$6" gap="$2">
          <Text fontSize="$5" fontWeight="700">
            Sign in required
          </Text>
          <Text opacity={0.75}>
            Open Settings and sign in to load devices and live telemetry.
          </Text>
        </YStack>
      ) : null}

      {devicesQuery.data ? (
        <YStack flex={1} minHeight={0}>
          <DeviceList
            devices={devicesQuery.data.devices}
            connectionStatus={connectionStatus}
            header={(
              <YStack marginTop={10} marginBottom="$3">
                <SummaryPanel devices={devicesQuery.data.devices} />
              </YStack>
            )}
          />
        </YStack>
      ) : null}
    </YStack>
  );
}
