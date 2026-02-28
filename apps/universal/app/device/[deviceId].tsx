import { useMemo } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Animated, Platform, ScrollView, useWindowDimensions } from 'react-native';
import { YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { TopBar } from '@/shared/ui/TopBar';
import { AppMenu } from '@/shared/ui/AppMenu';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { useDevice, useDevices } from '@/features/devices/hooks';
import {
  useTelemetryConnectionStatus,
  useTelemetryDeviceSnapshot,
  useTelemetrySubscription
} from '@/features/telemetry/hooks';
import { useDeviceDetailViewModel } from '@/features/device-detail/view-model';
import { DeviceDetailBody } from '@/features/device-detail/components/DeviceDetailBody';
import { env } from '@/shared/config/env';

const DETAIL_TREND_POINTS = 60;
const SOLAR_GENERATED_POINTS = 72;

export default function DeviceDetailScreen() {
  const { width } = useWindowDimensions();
  const isTablet = width >= 768;
  const isDesktop = width >= 1200;
  const useRemoteImage = Boolean(env.assetBaseUrl);
  const { deviceId } = useLocalSearchParams<{ deviceId: string }>();
  const router = useRouter();
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);
  const { authConfigured, authReady, authKey, sessionValid, token } = useAuthSession();
  const queryEnabled = authReady && (!authConfigured || sessionValid);
  const deviceQuery = useDevice(deviceId, { token, authKey, enabled: queryEnabled });
  const devicesQuery = useDevices({ token, authKey, enabled: queryEnabled });
  useTelemetrySubscription(deviceId ? [deviceId] : []);
  const telemetryConnectionStatus = useTelemetryConnectionStatus();
  const snapshot = useTelemetryDeviceSnapshot(deviceId);

  const listDeviceSolarSeries = useMemo(() => {
    if (!deviceId || !devicesQuery.data?.devices?.length) return [];
    return devicesQuery.data.devices.find((d) => d.id === deviceId)?.solarGeneratedSeriesWh ?? [];
  }, [deviceId, devicesQuery.data?.devices]);

  const detailSolarSeries = deviceQuery.data?.solarGeneratedSeriesWh ?? [];
  const perDeviceSolarSeries =
    detailSolarSeries.length >= listDeviceSolarSeries.length ? detailSolarSeries : listDeviceSolarSeries;

  const solarGeneratedTrend = useMemo(() => {
    const raw = perDeviceSolarSeries;
    if (!raw.length) {
      return Array.from({ length: SOLAR_GENERATED_POINTS }, () => 0);
    }
    const padded =
      raw.length >= SOLAR_GENERATED_POINTS
        ? raw.slice(-SOLAR_GENERATED_POINTS)
        : [...Array.from({ length: SOLAR_GENERATED_POINTS - raw.length }, () => 0), ...raw];
    return padded.map((v) => Math.max(0, v));
  }, [perDeviceSolarSeries]);

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
    device: deviceQuery.data,
    snapshot,
    connectionStatus: telemetryConnectionStatus,
    useRemoteImage
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
      device={deviceQuery.data}
      snapshot={snapshot}
      telemetryConnectionStatus={telemetryConnectionStatus}
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
          title={deviceQuery.data?.name ?? 'Device'}
          subtitle={deviceQuery.data ? deviceQuery.data.model : 'Loading…'}
          right={(
            <YStack alignItems="flex-end">
              <AppMenu />
            </YStack>
          )}
        />

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
