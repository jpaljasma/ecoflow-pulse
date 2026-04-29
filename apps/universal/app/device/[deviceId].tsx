import { useEffect, useMemo } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Animated, Platform, ScrollView } from 'react-native';
import { Text, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { useRequireAuth } from '@/features/auth/useRequireAuth';
import { TopBar } from '@/shared/ui/TopBar';
import { AppMenu } from '@/shared/ui/AppMenu';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { useDevice, useDevices } from '@/features/devices/hooks';
import type { DeviceSummary } from '@/features/devices/api';
import { useCurrentUser } from '@/features/profile/hooks';
import { resolveProfileWeatherState } from '@/features/weather/model';
import { useSolarOutlook, useWeatherForecast } from '@/features/weather/hooks';
import { buildStormGuardBanner } from '@/features/devices/stormGuard';
import { useDeviceInsights } from '@/features/inference/hooks';
import {
  useTelemetryConnectionStatus,
  useTelemetryDeviceSnapshot,
  useTelemetrySubscription
} from '@/features/telemetry/hooks';
import { useDeviceDetailViewModel } from '@/features/device-detail/view-model';
import { DeviceDetailBody } from '@/features/device-detail/components/DeviceDetailBody';
import { env } from '@/shared/config/env';
import { ApiError } from '@/shared/api/restClient';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';
import { Card } from '@/shared/ui/Card';
import { BreadcrumbTrail } from '@/shared/ui/BreadcrumbTrail';
import { StormGuardBanner } from '@/shared/ui/StormGuardBanner';
import { SecondaryPageShell } from '@/shared/ui/SecondaryPageShell';
import { useDevicePowerTrendHistory, useDeviceSolarHistory } from '@/features/history/hooks';
import {
  mergeTrendPrefillWithLivePoints
} from '@/features/history/powerTrend';
import { resolveSolarHistoryWindow } from '@/features/history/solar';
import { maskSerialNumber } from '@/features/telemetry/format';
import { usePageLayoutMetrics } from '@/shared/ui/navigationShell';

const DETAIL_TREND_POINTS = 60;
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

function describeRouteParam(value: string | undefined): string {
  if (!value) return 'unknown';
  return isUuid(value) ? value : maskSerialNumber(value);
}

function resolveRouteDevice(
  routeDeviceId: string | undefined,
  devices: DeviceSummary[] | undefined
): DeviceSummary | undefined {
  if (!routeDeviceId || !devices?.length) return undefined;
  return devices.find((device) => device.id === routeDeviceId);
}

function getMaxSolarWatts(device: DeviceSummary | undefined): number | undefined {
  const total = device?.details?.solarPorts?.reduce((sum, port) => sum + (port.maxWatts ?? 0), 0) ?? 0;
  return total > 0 ? total : undefined;
}

function mergeDeviceSources(
  preferred: DeviceSummary | undefined,
  fallback: DeviceSummary | undefined
): DeviceSummary | undefined {
  if (!preferred) return fallback;
  if (!fallback) return preferred;

  return {
    ...fallback,
    ...preferred,
    pvW: preferred.pvW ?? fallback.pvW,
    acInW: preferred.acInW ?? fallback.acInW,
    dcW: preferred.dcW ?? fallback.dcW,
    loadW: preferred.loadW ?? fallback.loadW,
    netW: preferred.netW ?? fallback.netW,
    tempC: preferred.tempC ?? fallback.tempC,
    telemetryTsMs: preferred.telemetryTsMs ?? fallback.telemetryTsMs,
    etaMinutes: preferred.etaMinutes ?? fallback.etaMinutes,
    capabilities: preferred.capabilities ?? fallback.capabilities,
    details: preferred.details ?? fallback.details
  };
}

export default function DeviceDetailScreen() {
  const { contentWidth: width, horizontalPadding, isSidebarMode } = usePageLayoutMetrics();
  const isTablet = width >= 768;
  const isDesktop = width >= 1200;
  const useRemoteImage = Boolean(env.assetBaseUrl);
  const { deviceId: routeDeviceParam } = useLocalSearchParams<{ deviceId: string | string[] }>();
  const routeDeviceId = Array.isArray(routeDeviceParam) ? routeDeviceParam[0] : routeDeviceParam;
  const router = useRouter();
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);
  const { authReady, authKey, token } = useAuthSession();
  const { allowed, waiting } = useRequireAuth();
  const queryEnabled = authReady && allowed;
  const currentUserQuery = useCurrentUser({
    token,
    authKey,
    enabled: queryEnabled
  });
  const devicesQuery = useDevices({ token, authKey, enabled: queryEnabled });
  const routeDevice = useMemo(
    () => resolveRouteDevice(routeDeviceId, devicesQuery.data?.devices),
    [routeDeviceId, devicesQuery.data?.devices]
  );
  const resolvedDeviceId = routeDevice?.id ?? (isUuid(routeDeviceId) ? routeDeviceId : undefined);
  const stormGuardBanner = useMemo(
    () => buildStormGuardBanner(devicesQuery.data?.devices),
    [devicesQuery.data?.devices]
  );

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
  const maxSolarWatts = getMaxSolarWatts(deviceQuery.data ?? routeDevice);
  const resolvedWeatherState = resolveProfileWeatherState(currentUserQuery.data?.user);
  const weatherForecastQuery = useWeatherForecast({
    token,
    authKey,
    locationKey: resolvedWeatherState.locationKey,
    enabled: queryEnabled && resolvedWeatherState.enabled
  });
  const solarHistoryWindow = useMemo(
    () => resolveSolarHistoryWindow(weatherForecastQuery.data?.forecast),
    [weatherForecastQuery.data?.forecast]
  );
  const solarHistory = useDeviceSolarHistory(resolvedDeviceId, {
    token,
    authKey,
    enabled: queryEnabled && Boolean(resolvedDeviceId),
    maxSolarWatts,
    window: solarHistoryWindow
  });
  const deviceSolarOutlookQuery = useSolarOutlook({
    token,
    authKey,
    locationKey: resolvedWeatherState.locationKey,
    enabled: queryEnabled && Boolean(resolvedDeviceId) && resolvedWeatherState.enabled,
    scope: 'device',
    deviceId: resolvedDeviceId
  });
  const powerTrendHistory = useDevicePowerTrendHistory(resolvedDeviceId, {
    token,
    authKey,
    enabled: queryEnabled && Boolean(resolvedDeviceId)
  });
  const batteryInsightsQuery = useDeviceInsights(resolvedDeviceId, {
    token,
    authKey,
    enabled: queryEnabled && Boolean(resolvedDeviceId)
  });
  const device = mergeDeviceSources(deviceQuery.data, routeDevice);
  useTelemetrySubscription(resolvedDeviceId ? [resolvedDeviceId] : []);
  const telemetryConnectionStatus = useTelemetryConnectionStatus();
  const snapshot = useTelemetryDeviceSnapshot(resolvedDeviceId);
  const firstError = devicesQuery.error ?? deviceQuery.error ?? solarHistory.error;
  const blockingRouteError = device ? undefined : firstError;
  const detailWarningError = device ? firstError : undefined;
  const deviceNotFound =
    queryEnabled &&
    !devicesQuery.isLoading &&
    !devicesQuery.isFetching &&
    !resolvedDeviceId &&
    !blockingRouteError;

  const solarGeneratedTrend =
    solarHistory.data?.seriesWh ?? Array.from({ length: solarHistoryWindow.points }, () => 0);
  const solarGeneratedYesterdayTrend =
    solarHistory.data?.yesterdaySeriesWh ?? Array.from({ length: solarHistoryWindow.points }, () => 0);

  const detailTrend = useMemo(
    () => ({
      load: mergeTrendPrefillWithLivePoints(
        powerTrendHistory.data?.load ?? Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
        snapshot?.sparkline.loadW
      ),
      pv: mergeTrendPrefillWithLivePoints(
        powerTrendHistory.data?.solar ?? Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
        snapshot?.sparkline.pvW
      ),
      ac: mergeTrendPrefillWithLivePoints(
        powerTrendHistory.data?.ac ?? Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
        snapshot?.sparkline.acW
      ),
      dc: mergeTrendPrefillWithLivePoints(
        powerTrendHistory.data?.dc ?? Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
        snapshot?.sparkline.dcW
      )
    }),
    [powerTrendHistory.data, snapshot]
  );

  const vm = useDeviceDetailViewModel({
    device,
    snapshot,
    connectionStatus: telemetryConnectionStatus,
    useRemoteImage,
    todayWh: solarHistory.data?.todayWh,
    yesterdayWh: solarHistory.data?.yesterdayWh,
    todayDeltaPct: solarHistory.data?.deltaPct
  });

  const mobileImageSize = Math.min(width - 64, 360);
  const mediaColumnWidth = isDesktop ? 320 : 280;
  const mediaBoxHeight = isDesktop
    ? Math.round(mediaColumnWidth * 0.86)
    : isTablet
      ? Math.round(mediaColumnWidth * 0.92)
      : mobileImageSize;

  if (waiting || !allowed) {
    return <BrandedLoadingState minHeight={260} message="Checking session…" />;
  }

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
      solarGeneratedYesterdayTrend={solarGeneratedYesterdayTrend}
      solarGeneratedTodayWh={solarHistory.data?.todayWh}
      solarGeneratedYesterdayWh={solarHistory.data?.yesterdayWh}
      solarGeneratedYesterdayRunningWh={solarHistory.data?.yesterdayRunningWh}
      solarGeneratedDeltaPct={solarHistory.data?.deltaPct}
      solarHistoryWindow={solarHistoryWindow}
      solarOutlook={deviceSolarOutlookQuery.data?.outlook}
      solarOutlookLoading={deviceSolarOutlookQuery.isLoading}
      solarOutlookErrorText={
        deviceSolarOutlookQuery.error ? describeQueryError(deviceSolarOutlookQuery.error) : undefined
      }
      batteryInsights={batteryInsightsQuery.data}
      batteryInsightsLoading={batteryInsightsQuery.isLoading}
    />
  );

  return (
    <Animated.View style={containerStyle}>
      <SecondaryPageShell activeNavKey="devices">
        <YStack flex={1} backgroundColor="$background" paddingHorizontal={horizontalPadding} paddingVertical="$4" gap="$4">
          <TopBar
            left={isSidebarMode ? undefined : <CloseToHomeButton onClose={closeToHome} />}
            eyebrow={(
              <BreadcrumbTrail
                items={[
                  {
                    label: 'Home',
                    href: '/(tabs)/devices',
                    icon: 'home-variant-outline',
                    hideLabel: true
                  },
                  {
                    label: 'Devices',
                    href: '/(tabs)/devices'
                  },
                  {
                    label: device?.name ?? 'Device',
                    current: true
                  }
                ]}
              />
            )}
            title={device?.name ?? 'Device'}
            subtitle={device ? device.model : 'Loading…'}
            right={(
              <YStack alignItems="flex-end">
                <AppMenu weatherScope="device" weatherDeviceId={resolvedDeviceId} />
              </YStack>
            )}
          />

          <YStack flex={1} minHeight={0}>
            {blockingRouteError ? (
              <Card gap="$2">
                <Text fontSize="$5" fontWeight="700">
                  Device detail failed to load
                </Text>
                <Text opacity={0.8}>{describeQueryError(blockingRouteError)}</Text>
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
                  Route parameter `{describeRouteParam(routeDeviceId)}` did not match any loaded device id or serial number.
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
                <YStack gap="$3">
                  {stormGuardBanner ? <StormGuardBanner {...stormGuardBanner} /> : null}
                  {detailWarningError ? (
                    <Card gap="$2">
                      <Text fontSize="$5" fontWeight="700">
                        Detail endpoint unavailable
                      </Text>
                      <Text opacity={0.8}>{describeQueryError(detailWarningError)}</Text>
                      <Text opacity={0.65}>
                        Showing cached summary values while live detail/history endpoints retry.
                      </Text>
                    </Card>
                  ) : null}
                  {detailContent}
                </YStack>
              </div>
            ) : (
              <ScrollView style={{ flex: 1 }} contentContainerStyle={{ paddingBottom: 16 }} showsVerticalScrollIndicator>
                <YStack gap="$3">
                  {stormGuardBanner ? <StormGuardBanner {...stormGuardBanner} /> : null}
                  {detailWarningError ? (
                    <Card gap="$2">
                      <Text fontSize="$5" fontWeight="700">
                        Detail endpoint unavailable
                      </Text>
                      <Text opacity={0.8}>{describeQueryError(detailWarningError)}</Text>
                      <Text opacity={0.65}>
                        Showing cached summary values while live detail/history endpoints retry.
                      </Text>
                    </Card>
                  ) : null}
                  {detailContent}
                </YStack>
              </ScrollView>
            )}
          </YStack>
        </YStack>
      </SecondaryPageShell>
    </Animated.View>
  );
}
