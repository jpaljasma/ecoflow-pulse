import { ActivityIndicator } from 'react-native';
import { useMemo } from 'react';
import { useRouter } from 'expo-router';
import { Text, XStack, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { AppMenu } from '@/shared/ui/AppMenu';
import { useDevices } from '@/features/devices/hooks';
import { useTelemetrySnapshot } from '@/features/telemetry/hooks';
import { DeviceList } from '@/features/devices/DeviceList';
import { formatAgo } from '@/features/telemetry/format';

function statusColor(status: string): '$success' | '$warning' | '$danger' {
  if (status === 'connected') return '$success';
  if (status === 'reconnecting' || status === 'connecting') return '$warning';
  return '$danger';
}

export default function DevicesScreen() {
  const router = useRouter();
  const devicesQuery = useDevices();
  const deviceIds = useMemo(
    () => devicesQuery.data?.devices.map((d) => d.id) ?? [],
    [devicesQuery.data?.devices]
  );
  const telemetry = useTelemetrySnapshot(deviceIds);

  return (
    <YStack flex={1} backgroundColor="$background">
      <TopBar
        title={<BrandLogo onPress={() => router.push('/devices')} />}
        subtitle={`Updated ${formatAgo(telemetry.lastUpdatedAt || null)}`}
        titleFlex={3}
        rightFlex={1}
        right={
          <XStack alignItems="flex-start" gap="$2" paddingBottom="$2">
            <Text fontSize="$7" color={statusColor(telemetry.connectionStatus)}>
              ●
            </Text>
            <AppMenu />
          </XStack>
        }
      />

      {devicesQuery.isLoading ? (
        <YStack flex={1} alignItems="center" justifyContent="center" gap="$3">
          <ActivityIndicator />
          <Text opacity={0.75}>Loading devices…</Text>
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
        <DeviceList devices={devicesQuery.data.devices} byId={telemetry.byId} />
      ) : null}
    </YStack>
  );
}
