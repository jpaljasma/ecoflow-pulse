import { ActivityIndicator, Animated, Image, Platform, useColorScheme, useWindowDimensions } from 'react-native';
import { useEffect, useMemo, useRef } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { AppMenu } from '@/shared/ui/AppMenu';
import { useDevices } from '@/features/devices/hooks';
import {
  useTelemetryConnectionStatus,
  useTelemetryLastUpdatedAt,
  useTelemetrySubscription
} from '@/features/telemetry/hooks';
import { SummaryPanel } from '@/features/devices/SummaryPanel';
import { DeviceList } from '@/features/devices/DeviceList';
import { formatAgo } from '@/features/telemetry/format';
import { getBundledBrandMark } from '@/shared/assets/brandBundled';

function statusDotColor(status: string): string {
  if (status === 'connected') return '#30d158';
  if (status === 'reconnecting' || status === 'connecting') return '#ff9f0a';
  return '#ff453a';
}

export default function DevicesScreen() {
  const { width } = useWindowDimensions();
  const scheme = useColorScheme();
  const loadingMark = getBundledBrandMark(scheme === 'dark' ? 'dark' : 'light');
  const compactHeader = width < 430;
  const devicesQuery = useDevices();
  const deviceIds = useMemo(
    () => devicesQuery.data?.devices.map((d) => d.id) ?? [],
    [devicesQuery.data?.devices]
  );
  useTelemetrySubscription(deviceIds);
  const connectionStatus = useTelemetryConnectionStatus();
  const lastUpdatedAt = useTelemetryLastUpdatedAt();
  const updatedAt = Math.max(
    lastUpdatedAt || 0,
    devicesQuery.dataUpdatedAt || 0
  );
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
            Updated {formatAgo(updatedAt || null)}
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
