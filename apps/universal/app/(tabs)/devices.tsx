import { Animated, Platform, useWindowDimensions } from 'react-native';
import { useEffect, useMemo, useRef } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { useRequireAuth } from '@/features/auth/useRequireAuth';
import { TopBar } from '@/shared/ui/TopBar';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';
import { AppMenu } from '@/shared/ui/AppMenu';
import { useDevices } from '@/features/devices/hooks';
import {
  useTelemetryConnectionStatus,
  useTelemetrySubscription
} from '@/features/telemetry/hooks';
import { SummaryPanel } from '@/features/devices/SummaryPanel';
import { DeviceList } from '@/features/devices/DeviceList';
import { AvailableDevicesPanel } from '@/features/devices/AvailableDevicesPanel';
import { buildStormGuardBanner } from '@/features/devices/stormGuard';
import { FleetEnergyImpactCard } from '@/features/energy-impact/FleetEnergyImpactCard';
import { formatConnectionStatus } from '@/features/telemetry/status';
import { StormGuardBanner } from '@/shared/ui/StormGuardBanner';
import {
  getConnectionStatusColor,
  useThemeSemantics,
  type ConnectionStatus
} from '@/shared/theme/semantic';

export default function DevicesScreen() {
  const { width } = useWindowDimensions();
  const semantics = useThemeSemantics();
  const compactHeader = width < 430;
  const { authConfigured, authReady, authKey, token } = useAuthSession();
  const { allowed, waiting } = useRequireAuth();
  const devicesQuery = useDevices({
    token,
    authKey,
    enabled: authReady && allowed
  });
  const deviceIds = useMemo(
    () => devicesQuery.data?.devices.map((d) => d.id) ?? [],
    [devicesQuery.data?.devices]
  );
  const stormGuardBanner = useMemo(
    () => buildStormGuardBanner(devicesQuery.data?.devices),
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

  if (waiting || !allowed) {
    return <BrandedLoadingState minHeight={260} message="Checking session…" />;
  }

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

      {devicesQuery.isLoading && !devicesQuery.data ? (
        <BrandedLoadingState minHeight={240} message="Loading devices…" />
      ) : null}

      {devicesQuery.isError ? (
        <YStack paddingHorizontal="$4" paddingVertical="$6" gap="$2">
          <Text fontSize="$5" fontWeight="700">
            Failed to load devices
          </Text>
          <Text color="$colorMuted" opacity={0.96}>{String(devicesQuery.error)}</Text>
        </YStack>
      ) : null}

      {devicesQuery.data ? (
        <YStack flex={1} minHeight={0}>
          <DeviceList
            devices={devicesQuery.data.devices}
            connectionStatus={connectionStatus}
            header={(
              <YStack marginTop={10} marginBottom="$3" gap="$3">
                {stormGuardBanner ? <StormGuardBanner {...stormGuardBanner} /> : null}
                <SummaryPanel devices={devicesQuery.data.devices} />
                <FleetEnergyImpactCard devices={devicesQuery.data.devices} />
              </YStack>
            )}
            footer={(
              <AvailableDevicesPanel
                token={token}
                authKey={authKey}
                enabled={authReady && allowed}
              />
            )}
          />
        </YStack>
      ) : null}
    </YStack>
  );
}
