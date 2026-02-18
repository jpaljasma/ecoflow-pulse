import { ActivityIndicator, Platform, ScrollView } from 'react-native';
import { useMemo } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { AppMenu } from '@/shared/ui/AppMenu';
import { useDevices } from '@/features/devices/hooks';
import { useTelemetrySnapshot } from '@/features/telemetry/hooks';
import { SummaryCard } from '@/features/devices/SummaryCard';
import { DeviceList } from '@/features/devices/DeviceList';
import { formatAgo } from '@/features/telemetry/format';

function statusColor(status: string): '$success' | '$warning' | '$danger' {
  if (status === 'connected') return '$success';
  if (status === 'reconnecting' || status === 'connecting') return '$warning';
  return '$danger';
}

export default function DevicesScreen() {
  const devicesQuery = useDevices();
  const deviceIds = useMemo(
    () => devicesQuery.data?.devices.map((d) => d.id) ?? [],
    [devicesQuery.data?.devices]
  );
  const telemetry = useTelemetrySnapshot(deviceIds);
  const updatedAt = Math.max(
    telemetry.lastUpdatedAt || 0,
    devicesQuery.dataUpdatedAt || 0
  );

  return (
    <YStack flex={1} backgroundColor="$background">
      <TopBar
        title={
          <BrandLogo
            onPress={() => {
              void devicesQuery.refetch();
            }}
          />
        }
        subtitle={`Updated ${formatAgo(updatedAt || null)}`}
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
        <YStack flex={1} minHeight={0}>
          {Platform.OS === 'web' ? (
            <div
              style={{
                flex: 1,
                minHeight: 0,
                overflowY: 'auto',
                overflowX: 'hidden',
                paddingBottom: 16
              }}
            >
              <YStack gap="$3">
                <YStack paddingHorizontal="$4">
                  <SummaryCard devices={devicesQuery.data.devices} byId={telemetry.byId} />
                </YStack>
                <DeviceList
                  devices={devicesQuery.data.devices}
                  byId={telemetry.byId}
                  connectionStatus={telemetry.connectionStatus}
                />
              </YStack>
            </div>
          ) : (
            <ScrollView
              style={{ flex: 1 }}
              contentContainerStyle={{ paddingBottom: 16 }}
              showsVerticalScrollIndicator
            >
              <YStack gap="$3">
                <YStack paddingHorizontal="$4">
                  <SummaryCard devices={devicesQuery.data.devices} byId={telemetry.byId} />
                </YStack>
                <DeviceList
                  devices={devicesQuery.data.devices}
                  byId={telemetry.byId}
                  connectionStatus={telemetry.connectionStatus}
                />
              </YStack>
            </ScrollView>
          )}
        </YStack>
      ) : null}
    </YStack>
  );
}
