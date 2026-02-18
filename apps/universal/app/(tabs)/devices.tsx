import { ActivityIndicator } from 'react-native';
import { useMemo } from 'react';
import { Text, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { Pill } from '@/shared/ui/Pill';
import { useDevices } from '@/features/devices/hooks';
import { useTelemetrySnapshot } from '@/features/telemetry/hooks';
import { DeviceList } from '@/features/devices/DeviceList';
import { formatAgo } from '@/features/telemetry/format';

export default function DevicesScreen() {
  const devicesQuery = useDevices();
  const deviceIds = useMemo(
    () => devicesQuery.data?.devices.map((d) => d.id) ?? [],
    [devicesQuery.data?.devices]
  );
  const telemetry = useTelemetrySnapshot(deviceIds);

  return (
    <YStack flex={1} backgroundColor="$background">
      <TopBar
        title="EcoFlow Pulse"
        subtitle={`Updated ${formatAgo(telemetry.lastUpdatedAt || null)}`}
        right={
          <Pill
            label={telemetry.connectionStatus.toUpperCase()}
            tone={telemetry.connectionStatus === 'connected' ? 'success' : 'warning'}
          />
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
