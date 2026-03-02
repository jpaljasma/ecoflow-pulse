import { useEffect, useMemo } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Animated, Platform, ScrollView, useWindowDimensions } from 'react-native';
import { Text, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { TopBar } from '@/shared/ui/TopBar';
import { AppMenu } from '@/shared/ui/AppMenu';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { useDevice, useDevices } from '@/features/devices/hooks';
import type { DeviceSummary } from '@/features/devices/api';
import {
  useTelemetryConnectionStatus,
  useTelemetryDeviceSnapshot,
  useTelemetrySubscription
} from '@/features/telemetry/hooks';
import { useDeviceDetailViewModel } from '@/features/device-detail/view-model';
import { DeviceDetailBody } from '@/features/device-detail/components/DeviceDetailBody';
import { env } from '@/shared/config/env';
import { ApiError } from '@/shared/api/restClient';
import { Card } from '@/shared/ui/Card';
import { useDeviceSolarHistory } from '@/features/history/hooks';

const DETAIL_TREND_POINTS = 60;
const SOLAR_GENERATED_POINTS = 72;
const UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

function isUuid(value: string | undefined): boolean {
  return Boolean(value && UUID_PATTERN.test(value));
}

function describeQueryError(error: unknown): string {
  if (error instanceof ApiError) return error.message;
  if (error instanceof Error) return error.message;
  return 'Unknown device detail error';
}

function resolveRouteDevice(
  routeDeviceId: string | undefined,
  devices: DeviceSummary[] | undefined
): DeviceSummary | undefined {
  if (!routeDeviceId || !devices?.length) return undefined;
  return devices.find(
    (device) => device.id === routeDeviceId || device.serialNumber === routeDeviceId
  );
}

export default function DeviceDetailScreen() {
  const { width } = useWindowDimensions();
  const isTablet = width >= 768;
  const isDesktop = width >= 1200;
  const useRemoteImage = Boolean(env.assetBaseUrl);
  const { deviceId: routeDeviceParam } = useLocalSearchParams<{ deviceId: string | string[] }>();
  const routeDeviceId = Array.isArray(routeDeviceParam) ? routeDeviceParam[0] : routeDeviceParam;
  const router = useRouter();
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);
  const { authConfigured, authReady, authKey, sessionValid, token } = useAuthSession();
  const queryEnabled = authReady && (!authConfigured || sessionValid);
  const devicesQuery = useDevices({ token, authKey, enabled: queryEnabled });
  const routeDevice = useMemo(
    () => resolveRouteDevice(routeDeviceId, devicesQuery.data?.devices),
    [routeDeviceId, devicesQuery.data?.devices]
  );
  const resolvedDeviceId = routeDevice?.id ?? (isUuid(routeDeviceId) ? routeDeviceId : undefined);

  useEffect(() => {
    if (!routeDeviceId || !routeDevice || routeDevice.id === routeDeviceId) {
      return;
    }
    router.replace(`/device/${routeDevice.id}`);
  }, [routeDevice, routeDeviceId, router]);

  const deviceQuery = useDevice(resolvedDeviceId, {
    token,
    authKey,
    enabled: queryEnabled && Boolean(resolvedDeviceId)
  });
  const solarHistory = useDeviceSolarHistory(resolvedDeviceId, {
    token,
    authKey,
    enabled: queryEnabled && Boolean(resolvedDeviceId)
  });
  const device = deviceQuery.data ?? routeDevice;
  useTelemetrySubscription(resolvedDeviceId ? [resolvedDeviceId] : []);
  const telemetryConnectionStatus = useTelemetryConnectionStatus();
  const snapshot = useTelemetryDeviceSnapshot(resolvedDeviceId);
  const routeError = deviceQuery.error ?? devicesQuery.error ?? solarHistory.error;
  const deviceNotFound =
    queryEnabled &&
    !devicesQuery.isLoading &&
    !devicesQuery.isFetching &&
    !resolvedDeviceId &&
    !routeError;

  const solarGeneratedTrend =
    solarHistory.data?.seriesWh ?? Array.from({ length: SOLAR_GENERATED_POINTS }, () => 0);

  const detailTrend = useMemo(
    () => ({
      load:
        snapshot?.sparkline.loadW.map((point) => point.value) ??
        Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
      pv:
        snapshot?.sparkline.pvW.map((point) => point.value) ??
        Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
      ac:
        snapshot?.sparkline.acW.map((point) => point.value) ??
        Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
      dc:
        snapshot?.sparkline.dcW.map((point) => point.value) ??
        Array.from({ length: DETAIL_TREND_POINTS }, () => 0)
    }),
    [snapshot]
  );

  const vm = useDeviceDetailViewModel({
    device,
    snapshot,
    connectionStatus: telemetryConnectionStatus,
    useRemoteImage,
    todayWh: solarHistory.data?.todayWh
  });

  const mobileImageSize = Math.min(width - 64, 360);
  const mediaColumnWidth = isDesktop ? 320 : 280;
  const mediaBoxHeight = isDesktop
    ? Math.round(mediaColumnWidth * 0.86)
    : isTablet
      ? Math.round(mediaColumnWidth * 0.92)
      : mobileImageSize;

  const detailContent = (
    <DeviceDetailBody
      device={device}
      snapshot={snapshot}
      vm={vm}
      isTablet={isTablet}
      isDesktop={isDesktop}
      mediaColumnWidth={mediaColumnWidth}
      mediaBoxHeight={mediaBoxHeight}
      mobileImageSize={mobileImageSize}
      sparklineLoad={detailTrend.load}
      sparklinePV={detailTrend.pv}
      sparklineAC={detailTrend.ac}
      sparklineDC={detailTrend.dc}
      solarGeneratedTrend={solarGeneratedTrend}
    />
  );

  return (
    <Animated.View style={containerStyle}>
      <YStack flex={1} backgroundColor="$background" paddingHorizontal="$4" paddingVertical="$4" gap="$4">
        <TopBar
          left={<CloseToHomeButton onClose={closeToHome} />}
          title={device?.name ?? 'Device'}
          subtitle={device ? device.model : 'Loading…'}
          right={(
            <YStack alignItems="flex-end">
              <AppMenu />
            </YStack>
          )}
        />

        <YStack flex={1} minHeight={0}>
          {routeError ? (
            <Card gap="$2">
              <Text fontSize="$5" fontWeight="700">
                Device detail failed to load
              </Text>
              <Text opacity={0.8}>{describeQueryError(routeError)}</Text>
              <Text opacity={0.65}>
                Check the local cluster public endpoint and API configuration. In k3d, the app, `/api`, and
                `/ws` should all be served from the same host.
              </Text>
            </Card>
          ) : deviceNotFound ? (
            <Card gap="$2">
              <Text fontSize="$5" fontWeight="700">
                Device not found
              </Text>
              <Text opacity={0.8}>
                Route parameter `{routeDeviceId ?? 'unknown'}` did not match any loaded device id or serial number.
              </Text>
            </Card>
          ) : Platform.OS === 'web' ? (
            <div
              style={{
                flex: 1,
                minHeight: 0,
                overflowY: 'auto',
                overflowX: 'hidden',
                paddingBottom: 16
              }}
            >
              {detailContent}
            </div>
          ) : (
            <ScrollView style={{ flex: 1 }} contentContainerStyle={{ paddingBottom: 16 }} showsVerticalScrollIndicator>
              {detailContent}
            </ScrollView>
          )}
        </YStack>
      </YStack>
    </Animated.View>
  );
}
