import { useMemo } from 'react';
import { Text, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { useRequireAuth } from '@/features/auth/useRequireAuth';
import { BreadcrumbTrail } from '@/shared/ui/BreadcrumbTrail';
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
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';

export default function DevicesScreen() {
  const { contentWidth } = useNavigationShellMetrics();
  const compactHeader = contentWidth < 430;
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

  if (waiting || !allowed) {
    return <BrandedLoadingState minHeight={260} message="Checking session…" />;
  }

  return (
    <YStack flex={1} backgroundColor="$background" testID="screen-devices">
      <TopBar
        eyebrow={(
          <BreadcrumbTrail
            items={[
              { label: 'Home', href: '/devices', icon: 'home-outline', hideLabel: true },
              { label: 'Devices', current: true }
            ]}
          />
        )}
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
        right={<AppMenu />}
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
