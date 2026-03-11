import { ActivityIndicator, Animated, Image, Platform, useWindowDimensions } from 'react-native';
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
import { FleetEnergyImpactCard } from '@/features/energy-impact/FleetEnergyImpactCard';
import { formatConnectionStatus } from '@/features/telemetry/status';
import { getBundledBrandMark } from '@/shared/assets/brandBundled';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import {
  getConnectionStatusColor,
  useThemeSemantics,
  type ConnectionStatus
} from '@/shared/theme/semantic';

export default function DevicesScreen() {
  const { width } = useWindowDimensions();
  const { isDark } = useAppTheme();
  const semantics = useThemeSemantics();
  const loadingMark = getBundledBrandMark(isDark ? 'dark' : 'light');
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
    <YStack flex={1} backgroundColor="$background" testID="screen-devices">
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
          <Text fontSize={11} color="$colorMuted" opacity={0.92} numberOfLines={1}>
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
                backgroundColor: getConnectionStatusColor(connectionStatus as ConnectionStatus, semantics),
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
            style={{
              backgroundColor: semantics.mutedPanelBackground,
              borderColor: semantics.mutedPanelBorder
            }}
            borderWidth={1}
          >
            <Image source={loadingMark} style={{ width: 34, height: 34 }} resizeMode="contain" />
          </YStack>
          <ActivityIndicator size="large" />
          <Text color="$color" opacity={0.96} fontSize="$5" fontWeight="700">
            Loading...
          </Text>
        </YStack>
      ) : null}

      {devicesQuery.isError ? (
        <YStack paddingHorizontal="$4" paddingVertical="$6" gap="$2">
          <Text fontSize="$5" fontWeight="700">
            Failed to load devices
          </Text>
          <Text color="$colorMuted" opacity={0.96}>{String(devicesQuery.error)}</Text>
        </YStack>
      ) : null}

      {!devicesQuery.data && !devicesQuery.isLoading && !devicesQuery.isError && authConfigured && authReady && !sessionValid ? (
        <YStack paddingHorizontal="$4" paddingVertical="$6" gap="$2">
          <Text fontSize="$5" fontWeight="700">
            Sign in required
          </Text>
          <Text color="$colorMuted" opacity={0.96}>
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
              <YStack marginTop={10} marginBottom="$3" gap="$3">
                <SummaryPanel devices={devicesQuery.data.devices} />
                <FleetEnergyImpactCard devices={devicesQuery.data.devices} />
              </YStack>
            )}
          />
        </YStack>
      ) : null}
    </YStack>
  );
}
