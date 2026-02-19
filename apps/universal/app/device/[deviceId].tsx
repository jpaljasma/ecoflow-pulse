import { useEffect, useMemo, useRef, useState } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Animated, Platform, ScrollView, useWindowDimensions } from 'react-native';
import { Image as ExpoImage } from 'expo-image';
import { Text, XStack, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { Card } from '@/shared/ui/Card';
import { PowerFlowGlyph } from '@/shared/ui/PowerFlowGlyph';
import { Pill } from '@/shared/ui/Pill';
import { Stat } from '@/shared/ui/Stat';
import { AppMenu } from '@/shared/ui/AppMenu';
import { SocBar } from '@/shared/ui/SocBar';
import { PowerTrendChart } from '@/shared/ui/PowerTrendChart';
import { SolarGeneratedChart } from '@/shared/ui/SolarGeneratedChart';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { useChartPrefs } from '@/shared/ui/chartPrefs';
import { SolarTodayBadge } from '@/shared/ui/SolarTodayBadge';
import { useDevice, useDevices } from '@/features/devices/hooks';
import { getCapacityKWh } from '@/features/devices/capacity';
import { useTelemetrySnapshot } from '@/features/telemetry/hooks';
import { formatAgo, formatEtaMinutes, formatKWh, formatW } from '@/features/telemetry/format';
import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';
import { getEcoFlowAsset, getEcoFlowDefaultSize } from '@/shared/assets/ecoflowAssets';
import { getStatusGlyph } from '@/shared/ui/statusGlyph';
import { CachedImage } from '@/shared/ui/CachedImage';
import { getBundledDeviceFallback } from '@/shared/assets/deviceFallbacks';
import { env } from '@/shared/config/env';

const DETAIL_TREND_POINTS = 60;
const DETAIL_TREND_BUCKET_MS = 5_000;
const SOLAR_GENERATED_POINTS = 72;

function isNearZero(value: number | undefined | null): boolean {
  if (value === null || value === undefined || Number.isNaN(value)) return false;
  return value >= -0.5 && value <= 0.5;
}

function toneForState(state: string | undefined): 'neutral' | 'success' | 'warning' | 'danger' | 'info' {
  const value = (state ?? '').toLowerCase();
  if (value.includes('charging') || value.includes('online') || value.includes('on')) return 'success';
  if (value.includes('locked') || value.includes('idle')) return 'warning';
  if (value.includes('error') || value.includes('fault') || value.includes('off')) return 'danger';
  return 'neutral';
}

function formatSolarState(state: string | undefined): string {
  if (!state) return 'unknown';
  return state.replaceAll('_', ' ');
}

function isInactivePvPort(volts: number | undefined): boolean {
  return Number.isFinite(volts as number) && Math.abs((volts as number) ?? 0) <= 0.5;
}

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, value));
}

function toPctOfMax(watts: number | undefined, maxWatts: number | undefined): number | null {
  if (!Number.isFinite(watts as number) || !Number.isFinite(maxWatts as number) || (maxWatts as number) <= 0) {
    return null;
  }
  return ((watts as number) / (maxWatts as number)) * 100;
}

function pvLoadColor(percent: number): string {
  const pct = clampPercent(percent) / 100;
  // Match SOC orange tone: #ff9f0a, with a lighter ramp at low utilization.
  const start = { r: 255, g: 232, b: 199 };
  const end = { r: 255, g: 159, b: 10 };
  const r = Math.round(start.r + (end.r - start.r) * pct);
  const g = Math.round(start.g + (end.g - start.g) * pct);
  const b = Math.round(start.b + (end.b - start.b) * pct);
  return `rgb(${r},${g},${b})`;
}

function getSolarPortUiState(port: {
  state?: string;
  volts?: number;
  amps?: number;
  watts?: number;
}): { label: string; tone: 'neutral' | 'success' | 'warning' | 'danger' | 'info' } {
  const stateLabel = formatSolarState(port.state).toLowerCase();
  const hasFlow =
    (Number.isFinite(port.watts as number) && (port.watts as number) > 1) ||
    (Number.isFinite(port.amps as number) && (port.amps as number) > 0.03);
  const isInactive = isInactivePvPort(port.volts);

  if (isInactive) {
    return { label: 'inactive', tone: 'neutral' };
  }
  if (stateLabel === 'unknown' && hasFlow) {
    return { label: 'active', tone: 'success' };
  }
  return {
    label: stateLabel,
    tone: toneForState(port.state)
  };
}

export default function DeviceDetailScreen() {
  const { width } = useWindowDimensions();
  const isTablet = width >= 768;
  const isDesktop = width >= 1200;
  const useRemoteImage = Boolean(env.assetBaseUrl);
  const { deviceId } = useLocalSearchParams<{ deviceId: string }>();
  const router = useRouter();
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);
  const trendChartStyle = useChartPrefs((s) => s.trendChartStyle);
  const toggleTrendChartStyle = useChartPrefs((s) => s.toggleTrendChartStyle);
  const deviceQuery = useDevice(deviceId);
  const devicesQuery = useDevices();
  const telemetry = useTelemetrySnapshot(deviceId ? [deviceId] : []);
  const snapshot = deviceId ? telemetry.byId[deviceId] : undefined;
  const [nowMs, setNowMs] = useState(() => Date.now());
  const [detailTrend, setDetailTrend] = useState<{
    load: number[];
    pv: number[];
    ac: number[];
    dc: number[];
  }>(() => ({
    load: Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
    pv: Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
    ac: Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
    dc: Array.from({ length: DETAIL_TREND_POINTS }, () => 0)
  }));
  const [solarGeneratedTrend, setSolarGeneratedTrend] = useState<number[]>(
    Array.from({ length: SOLAR_GENERATED_POINTS }, () => 0)
  );
  const solarGeneratedMinuteRef = useRef<number | null>(null);
  const solarGeneratedSigRef = useRef<string>('');
  const solarGeneratedInitializedRef = useRef(false);
  const pendingBucketRef = useRef<{
    bucket: number;
    loadSum: number;
    pvSum: number;
    acSum: number;
    dcSum: number;
    count: number;
  } | null>(null);

  useEffect(() => {
    const timer = setInterval(() => setNowMs(Date.now()), 1_000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    pendingBucketRef.current = null;
    solarGeneratedInitializedRef.current = false;
    solarGeneratedMinuteRef.current = null;
    solarGeneratedSigRef.current = '';
    setDetailTrend({
      load: Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
      pv: Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
      ac: Array.from({ length: DETAIL_TREND_POINTS }, () => 0),
      dc: Array.from({ length: DETAIL_TREND_POINTS }, () => 0)
    });
    setSolarGeneratedTrend(Array.from({ length: SOLAR_GENERATED_POINTS }, () => 0));
  }, [deviceId]);

  const listDeviceSolarSeries = useMemo(() => {
    if (!deviceId || !devicesQuery.data?.devices?.length) return [];
    return devicesQuery.data.devices.find((d) => d.id === deviceId)?.solarGeneratedSeriesWh ?? [];
  }, [deviceId, devicesQuery.data?.devices]);

  const detailSolarSeries = deviceQuery.data?.solarGeneratedSeriesWh ?? [];
  const perDeviceSolarSeries =
    detailSolarSeries.length >= listDeviceSolarSeries.length
      ? detailSolarSeries
      : listDeviceSolarSeries;

  useEffect(() => {
    const raw = perDeviceSolarSeries;
    if (!raw.length) {
      return;
    }
    const minuteBucket = Math.floor(nowMs / 60_000);
    const rawSig = `${raw.length}:${raw[0] ?? 0}:${raw[raw.length - 1] ?? 0}:${raw
      .slice(Math.max(0, raw.length - 6))
      .map((v) => Number(v).toFixed(3))
      .join(',')}`;
    if (
      solarGeneratedInitializedRef.current &&
      solarGeneratedMinuteRef.current === minuteBucket &&
      solarGeneratedSigRef.current === rawSig
    ) {
      return;
    }
    const padded =
      raw.length >= SOLAR_GENERATED_POINTS
        ? raw.slice(-SOLAR_GENERATED_POINTS)
        : [...Array.from({ length: SOLAR_GENERATED_POINTS - raw.length }, () => 0), ...raw];
    setSolarGeneratedTrend(padded.map((v) => Math.max(0, v)));
    solarGeneratedInitializedRef.current = true;
    solarGeneratedMinuteRef.current = minuteBucket;
    solarGeneratedSigRef.current = rawSig;
  }, [perDeviceSolarSeries, nowMs]);

  useEffect(() => {
    const currentLoad = snapshot?.metrics?.loadW ?? deviceQuery.data?.loadW ?? 0;
    const currentPV = snapshot?.metrics?.pvW ?? deviceQuery.data?.pvW ?? 0;
    const currentAC = deviceQuery.data?.acInW ?? 0;
    const currentDC = deviceQuery.data?.dcW ?? 0;
    const currentBucket = Math.floor(nowMs / DETAIL_TREND_BUCKET_MS);
    const pending = pendingBucketRef.current;

    if (!pending) {
      pendingBucketRef.current = {
        bucket: currentBucket,
        loadSum: currentLoad,
        pvSum: currentPV,
        acSum: currentAC,
        dcSum: currentDC,
        count: 1
      };
      return;
    }

    if (pending.bucket === currentBucket) {
      pending.loadSum += currentLoad;
      pending.pvSum += currentPV;
      pending.acSum += currentAC;
      pending.dcSum += currentDC;
      pending.count += 1;
      return;
    }

    const bucketLoadAvg = pending.count > 0 ? pending.loadSum / pending.count : 0;
    const bucketPVAvg = pending.count > 0 ? pending.pvSum / pending.count : 0;
    const bucketACAvg = pending.count > 0 ? pending.acSum / pending.count : 0;
    const bucketDCAvg = pending.count > 0 ? pending.dcSum / pending.count : 0;
    const bucketDelta = Math.max(1, currentBucket - pending.bucket);

    setDetailTrend((prev) => {
      const loadNext = [...prev.load];
      const pvNext = [...prev.pv];
      const acNext = [...prev.ac];
      const dcNext = [...prev.dc];
      for (let i = 0; i < bucketDelta; i += 1) {
        loadNext.push(bucketLoadAvg);
        pvNext.push(bucketPVAvg);
        acNext.push(bucketACAvg);
        dcNext.push(bucketDCAvg);
      }
      return {
        load: loadNext.slice(-DETAIL_TREND_POINTS),
        pv: pvNext.slice(-DETAIL_TREND_POINTS),
        ac: acNext.slice(-DETAIL_TREND_POINTS),
        dc: dcNext.slice(-DETAIL_TREND_POINTS)
      };
    });

    pendingBucketRef.current = {
      bucket: currentBucket,
      loadSum: currentLoad,
      pvSum: currentPV,
      acSum: currentAC,
      dcSum: currentDC,
      count: 1
    };
  }, [nowMs, snapshot?.metrics?.loadW, snapshot?.metrics?.pvW, deviceQuery.data?.loadW, deviceQuery.data?.pvW, deviceQuery.data?.acInW, deviceQuery.data?.dcW]);

  const sparklineLoad = useMemo(() => detailTrend.load, [detailTrend.load]);
  const sparklinePV = useMemo(() => detailTrend.pv, [detailTrend.pv]);
  const sparklineAC = useMemo(() => detailTrend.ac, [detailTrend.ac]);
  const sparklineDC = useMemo(() => detailTrend.dc, [detailTrend.dc]);
  const acInW = deviceQuery.data?.acInW;
  const dcW = deviceQuery.data?.dcW;
  const tempC = snapshot?.metrics?.tempC;
  const isColdTemp = typeof tempC === 'number' && tempC <= 2;
  const netW =
    snapshot?.metrics
      ? snapshot.metrics.pvW - snapshot.metrics.loadW
      : deviceQuery.data?.netW;
  const capacityKWh = deviceQuery.data ? getCapacityKWh(deviceQuery.data) : null;
  const fallbackState =
    deviceQuery.data?.state === 'charging' ||
    deviceQuery.data?.state === 'discharging' ||
    deviceQuery.data?.state === 'idle'
      ? deviceQuery.data.state
      : 'idle';
  const detailState =
    snapshot && !snapshot.stale && snapshot.status !== 'stale'
      ? snapshot.status
      : fallbackState;
  const connectionGlyph =
    snapshot?.stale
      ? getStatusGlyph('stale')
      : telemetry.connectionStatus === 'connected'
        ? getStatusGlyph('online')
        : telemetry.connectionStatus === 'connecting' ||
            telemetry.connectionStatus === 'reconnecting'
          ? getStatusGlyph('processing')
          : getStatusGlyph('waiting');
  const deviceAsset = useMemo(() => {
    const model = deviceQuery.data?.model;
    if (!model) return null;
    const batteryCount =
      deviceQuery.data?.details?.bpCount ??
      ((deviceQuery.data?.capabilities as { batteryPacks?: number } | undefined)
        ?.batteryPacks ?? 1);
    const match = getDeviceAssetMatch(model, { batteryCount });
    if (!match.slug) return null;
    return {
      slug: match.slug,
      uri: useRemoteImage
        ? getEcoFlowAsset(match.slug, getEcoFlowDefaultSize('detail'))
        : undefined,
      emoji: match.glyph.emoji
    };
  }, [deviceQuery.data?.model, deviceQuery.data?.details?.bpCount, deviceQuery.data?.capabilities, useRemoteImage]);
  const detailFallback = useMemo(
    () => (deviceAsset?.slug ? getBundledDeviceFallback(deviceAsset.slug, '512') : undefined),
    [deviceAsset?.slug]
  );
  const [detailImageFailed, setDetailImageFailed] = useState(false);
  const modelLower = (deviceQuery.data?.model ?? '').toLowerCase();
  const details = deviceQuery.data?.details;
  const packRows = details?.packs ?? [];
  const solarRows = details?.solarPorts ?? [];
  const estimateLabel = details?.estimateMode
    ? `${details.estimateMode}${details.estimateSource ? ` · ${details.estimateSource}` : ''}`
    : details?.estimateSource ?? 'n/a';
  const capabilities = deviceQuery.data?.capabilities as Record<string, unknown> | undefined;
  const supportsEvCharging =
    modelLower.includes('delta pro ultra') ||
    capabilities?.evCharging === true ||
    capabilities?.evCharger === true ||
    capabilities?.evOutput === true;
  const supportsBatteryHeating =
    modelLower.includes('delta pro ultra') ||
    capabilities?.batteryHeating === true ||
    capabilities?.preconditioning === true;
  const preconditioningOn =
    details?.batteryHeatingOn === true ||
    (details?.packs ?? []).some((pack) => pack.heatingOn === true);
  const dcSignalOn = details?.dcOn === true || details?.usbOn === true || details?.dc12vOn === true;
  const signalRows: Array<{ label: string; on?: boolean }> = [
    { label: 'AC On', on: details?.acOn },
    { label: 'DC On', on: dcSignalOn },
    { label: 'USB On', on: details?.usbOn },
    { label: '12V On', on: details?.dc12vOn },
    ...(supportsEvCharging ? [{ label: 'EV Charging', on: details?.evChargingOn }] : []),
    ...(supportsBatteryHeating ? [{ label: 'Preconditioning', on: preconditioningOn }] : []),
    { label: 'Fan', on: details?.fanOn },
    { label: 'Solar Charging', on: details?.solarChargingOn }
  ];
  const isDelta2Max = modelLower.includes('delta 2 max');
  const isDeltaProUltra = modelLower.includes('delta pro ultra');
  const desktopScale = isDelta2Max ? 1.46 : isDeltaProUltra ? 1.5 : 1.48;
  const desktopOffsetY = isDelta2Max ? 8 : 0;
  const mobileImageSize = Math.min(width - 64, 360);
  const mediaColumnWidth = isDesktop ? 320 : 280;
  const mediaBoxHeight = isDesktop
    ? Math.round(mediaColumnWidth * 0.86)
    : isTablet
      ? Math.round(mediaColumnWidth * 0.92)
      : mobileImageSize;

  useEffect(() => {
    setDetailImageFailed(false);
  }, [deviceAsset?.uri, deviceAsset?.slug]);

  return (
    <Animated.View style={containerStyle}>
      <YStack
        flex={1}
        backgroundColor="$background"
        paddingHorizontal="$4"
        paddingVertical="$4"
        gap="$4"
      >
      <TopBar
        left={<CloseToHomeButton onClose={closeToHome} />}
        title={deviceQuery.data?.name ?? 'Device'}
        subtitle={deviceQuery.data ? `${deviceQuery.data.model} · ${formatAgo(snapshot?.lastSeenAt ?? null)}` : 'Loading…'}
        right={
          <YStack alignItems="flex-end">
            <AppMenu />
          </YStack>
        }
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
            <YStack gap="$3">
              <Card gap="$3">
        <XStack
          gap="$4"
          alignItems={isTablet ? 'stretch' : 'flex-start'}
          flexDirection={isTablet ? 'row' : 'column'}
        >
          {deviceAsset ? (
            <YStack
              width={isTablet ? mediaColumnWidth : '100%'}
              height={mediaBoxHeight}
              flexShrink={0}
              borderRadius="$4"
              overflow="hidden"
              backgroundColor="rgba(120,120,128,0.12)"
              alignItems="center"
              justifyContent="center"
            >
              {deviceAsset.uri && !detailImageFailed ? (
                <CachedImage
                  uri={deviceAsset.uri}
                  style={{
                    width: (isTablet ? mediaColumnWidth : mobileImageSize) * (isTablet ? desktopScale : 1.35),
                    height: mediaBoxHeight * (isTablet ? desktopScale : 1.35),
                    transform: isTablet ? [{ translateY: desktopOffsetY }] : undefined
                  }}
                  contentFit="cover"
                  onError={() => setDetailImageFailed(true)}
                />
              ) : detailFallback ? (
                <ExpoImage
                  source={detailFallback}
                  style={{
                    width: (isTablet ? mediaColumnWidth : mobileImageSize) * (isTablet ? desktopScale : 1.35),
                    height: mediaBoxHeight * (isTablet ? desktopScale : 1.35),
                    transform: isTablet ? [{ translateY: desktopOffsetY }] : undefined
                  }}
                  contentFit="cover"
                />
              ) : null}
            </YStack>
          ) : null}

          <YStack gap="$3" flex={1} minWidth={0}>
            <XStack alignItems="flex-end" gap="$3">
              <XStack flex={1} minWidth={0}>
                <SocBar value={snapshot?.metrics?.soc ?? deviceQuery.data?.batteryPct} fullWidth />
              </XStack>
              <Text fontSize="$3" opacity={0.75} marginBottom="$1" flexShrink={0}>
                {capacityKWh !== null ? `🔋 ${formatKWh(capacityKWh)}` : '🔋 n/a'}
              </Text>
            </XStack>
            <XStack flexWrap="wrap" marginHorizontal={-4}>
              <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                <Stat label="∿ AC" value={formatW(acInW)} tone={isNearZero(acInW) ? 'muted' : 'default'} />
              </YStack>
              <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                <Stat label="⎓ DC" value={formatW(dcW)} tone={isNearZero(dcW) ? 'muted' : 'default'} />
              </YStack>
              <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                <Stat label="☼ PV" value={formatW(snapshot?.metrics?.pvW)} tone={isNearZero(snapshot?.metrics?.pvW) ? 'muted' : 'default'} />
              </YStack>
              <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                <SolarTodayBadge valueWh={deviceQuery.data?.solarTodayWh} compact fitCell />
              </YStack>
              <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                <Stat label="⌂ Load" value={formatW(snapshot?.metrics?.loadW)} tone={isNearZero(snapshot?.metrics?.loadW) ? 'muted' : 'default'} />
              </YStack>
              <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                <Stat label="⚖ Net" value={formatW(netW)} />
              </YStack>
              <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                <Stat label="🔋 Battery" value={formatW(snapshot?.metrics?.batteryW)} />
              </YStack>
              <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                <Stat
                  label={isColdTemp ? '❄ Temp' : '🌡 Temp'}
                  value={snapshot?.metrics ? `${snapshot.metrics.tempC.toFixed(1)}°C` : '—'}
                  tone={isColdTemp ? 'cold' : 'default'}
                />
              </YStack>
              <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                <Stat
                  label="◉ State"
                  value={deviceQuery.data ? detailState : '—'}
                />
              </YStack>
              <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                <Stat label="⏱ ETA" value={formatEtaMinutes(deviceQuery.data?.etaMinutes)} />
              </YStack>
            </XStack>
            <XStack justifyContent="flex-end" alignItems="center" gap="$2">
              {deviceQuery.data ? (
                <PowerFlowGlyph
                  status={detailState}
                  pvW={snapshot?.metrics?.pvW ?? deviceQuery.data?.pvW}
                  loadW={snapshot?.metrics?.loadW ?? deviceQuery.data?.loadW}
                  fontSize="$6"
                  lineHeight={24}
                />
              ) : null}
              <Text fontSize="$3" opacity={0.9}>
                {connectionGlyph}
              </Text>
            </XStack>
          </YStack>
      </XStack>
    </Card>

      {isTablet ? (
        <XStack gap="$3" alignItems="stretch" flexWrap="nowrap">
          <YStack
            flexBasis="50%"
            minWidth="50%"
            maxWidth="50%"
            gap="$2"
            padding="$3"
            borderRadius="$3"
            borderWidth={1}
            borderColor="rgba(120,120,128,0.24)"
          >
            <XStack alignItems="center" justifyContent="space-between">
              <Text fontSize="$4" fontWeight="700">
                ☼ Solar Generated (6am-6pm, 10m buckets)
              </Text>
              <Text fontSize="$2" opacity={0.72}>
                1m refresh
              </Text>
            </XStack>
            <SolarGeneratedChart valuesWh={solarGeneratedTrend} points={SOLAR_GENERATED_POINTS} />
          </YStack>
          <YStack
            flexBasis="50%"
            minWidth="50%"
            maxWidth="50%"
            gap="$2"
            padding="$3"
            borderRadius="$3"
            borderWidth={1}
            borderColor="rgba(120,120,128,0.24)"
          >
            <XStack alignItems="center" justifyContent="space-between">
              <Text fontSize="$4" fontWeight="700">
                Power Trends
              </Text>
              <Text
                fontSize="$2"
                opacity={0.88}
                paddingHorizontal="$2"
                paddingVertical="$1"
                borderRadius="$3"
                borderWidth={1}
                borderColor="rgba(120,120,128,0.3)"
                onPress={toggleTrendChartStyle}
                cursor="pointer"
              >
                {trendChartStyle === 'premium' ? 'Premium' : 'ASCII'}
              </Text>
            </XStack>
            <PowerTrendChart
              solar={sparklinePV}
              ac={sparklineAC}
              dc={sparklineDC}
              load={sparklineLoad}
              points={DETAIL_TREND_POINTS}
              style={trendChartStyle}
            />
          </YStack>
        </XStack>
      ) : (
        <YStack gap="$3">
          <Card gap="$2">
            <XStack alignItems="center" justifyContent="space-between">
              <Text fontSize="$4" fontWeight="700">
                ☼ Solar Generated (6am-6pm, 10m buckets)
              </Text>
              <Text fontSize="$2" opacity={0.72}>
                1m refresh
              </Text>
            </XStack>
            <SolarGeneratedChart valuesWh={solarGeneratedTrend} points={SOLAR_GENERATED_POINTS} />
          </Card>
          <Card gap="$2">
            <XStack alignItems="center" justifyContent="space-between">
              <Text fontSize="$4" fontWeight="700">
                Power Trends
              </Text>
              <Text
                fontSize="$2"
                opacity={0.88}
                paddingHorizontal="$2"
                paddingVertical="$1"
                borderRadius="$3"
                borderWidth={1}
                borderColor="rgba(120,120,128,0.3)"
                onPress={toggleTrendChartStyle}
                cursor="pointer"
              >
                {trendChartStyle === 'premium' ? 'Premium' : 'ASCII'}
              </Text>
            </XStack>
            <PowerTrendChart
              solar={sparklinePV}
              ac={sparklineAC}
              dc={sparklineDC}
              load={sparklineLoad}
              points={DETAIL_TREND_POINTS}
              style={trendChartStyle}
            />
          </Card>
        </YStack>
      )}

      <XStack gap="$3" flexWrap="wrap">
        <Card gap="$3" flex={1} minWidth={isDesktop ? 320 : 280}>
          <XStack justifyContent="space-between" alignItems="center">
            <Text fontSize="$4" fontWeight="700">
              🔋 Battery Packs
            </Text>
            <Pill
              label={`${details?.bpCount ?? packRows.length ?? 0} packs`}
              tone="info"
            />
          </XStack>
          {packRows.length ? (
            <YStack gap="$2">
              {packRows.map((pack) => (
                <YStack
                  key={pack.id}
                  gap="$1"
                  padding="$2"
                  borderRadius="$3"
                  borderWidth={1}
                  borderColor={
                    pack.heatingOn
                      ? 'rgba(255,159,10,0.55)'
                      : 'rgba(120,120,128,0.24)'
                  }
                  backgroundColor={
                    pack.heatingOn
                      ? 'rgba(255,159,10,0.10)'
                      : undefined
                  }
                >
                  <XStack alignItems="center" justifyContent="space-between">
                    <Text fontWeight="700">{pack.id.toUpperCase()}</Text>
                    <XStack gap="$2" alignItems="center">
                      {pack.heatingOn ? (
                        <Text color="rgba(255,159,10,0.95)" fontWeight="700">
                          ♨ Preconditioning
                        </Text>
                      ) : null}
                      <Text opacity={0.8}>
                        {Number.isFinite(pack.powerW as number) ? formatW(pack.powerW) : '—'}
                        {' · '}
                        {Number.isFinite(pack.tempC as number) ? `${pack.tempC?.toFixed(1)}°C` : '—'}
                      </Text>
                    </XStack>
                  </XStack>
                  <SocBar value={pack.socPct} fullWidth />
                </YStack>
              ))}
            </YStack>
          ) : (
            <Text opacity={0.7}>No per-pack telemetry yet.</Text>
          )}
        </Card>

        <Card gap="$3" flex={1} minWidth={isDesktop ? 320 : 280}>
          <Text fontSize="$4" fontWeight="700">
            ☀ Solar Inputs
          </Text>
          <YStack gap="$2">
            {solarRows.map((port) => (
              (() => {
                const portInactive = isInactivePvPort(port.volts);
                const uiState = getSolarPortUiState(port);
                const pvLoadPct = toPctOfMax(port.watts, port.maxWatts);
                const barPct = clampPercent(pvLoadPct ?? 0);
                return (
              <YStack
                key={port.id}
                gap="$2"
                padding="$2"
                borderRadius="$3"
                borderWidth={1}
                borderColor="rgba(120,120,128,0.24)"
                opacity={portInactive ? 0.72 : 1}
              >
                <XStack justifyContent="space-between" alignItems="center">
                  <Text fontWeight="700">{port.name}</Text>
                  <Pill
                    label={uiState.label}
                    tone={uiState.tone}
                  />
                </XStack>
                <XStack gap="$3" flexWrap="wrap">
                  <Stat
                    label="⚡ W"
                    value={formatW(port.watts)}
                    tone={portInactive || isNearZero(port.watts) ? 'muted' : 'default'}
                  />
                  <Stat
                    label="V"
                    value={Number.isFinite(port.volts as number) ? `${port.volts?.toFixed(1)}V` : '—'}
                    tone={portInactive ? 'muted' : 'default'}
                  />
                  <Stat
                    label="A"
                    value={Number.isFinite(port.amps as number) ? `${port.amps?.toFixed(2)}A` : '—'}
                    tone={portInactive ? 'muted' : 'default'}
                  />
                  <Stat
                    label="Cap"
                    value={
                      port.maxWatts
                        ? `${port.maxWatts}W · ${port.maxVolts ?? '—'}V · ${port.maxAmps ?? '—'}A`
                        : '—'
                    }
                    tone={portInactive ? 'muted' : 'default'}
                  />
                </XStack>
                <YStack gap="$1">
                  <XStack alignItems="center" justifyContent="space-between">
                    <Text opacity={portInactive ? 0.6 : 0.85} fontWeight="600">
                      PV Load
                    </Text>
                    <Text opacity={portInactive ? 0.6 : 0.9} fontWeight="700">
                      {pvLoadPct === null ? '—' : `${barPct.toFixed(1)}%`}
                    </Text>
                  </XStack>
                  <XStack
                    height={10}
                    borderRadius="$5"
                    overflow="hidden"
                    backgroundColor="rgba(255,159,10,0.14)"
                  >
                    <XStack
                      height="100%"
                      width={`${barPct}%` as `${number}%`}
                      opacity={portInactive ? 0.5 : 1}
                      style={{ backgroundColor: pvLoadColor(barPct) }}
                    />
                  </XStack>
                </YStack>
              </YStack>
                );
              })()
            ))}
          </YStack>
        </Card>
      </XStack>

      <XStack gap="$3" flexWrap="wrap">
        <Card gap="$3" flex={1} minWidth={isDesktop ? 360 : 280}>
          <Text fontSize="$4" fontWeight="700">
            🧭 Estimate & Queue
          </Text>
          <XStack gap="$2" flexWrap="wrap">
            <Pill label={`mode: ${estimateLabel}`} tone="neutral" />
            <Pill
              label={`eta: ${formatEtaMinutes(details?.estimateEtaMin ?? deviceQuery.data?.etaMinutes)}`}
              tone="info"
            />
            <Pill
              label={`queue: ${details?.mqttQueueDepth ?? 0}`}
              tone={(details?.mqttQueueDepth ?? 0) > 48 ? 'warning' : 'success'}
            />
            <Pill
              label={`dropped: ${details?.mqttQueueDroppedOldest ?? 0}`}
              tone={(details?.mqttQueueDroppedOldest ?? 0) > 0 ? 'danger' : 'neutral'}
            />
          </XStack>
        </Card>
        <Card gap="$3" flex={1} minWidth={isDesktop ? 360 : 280}>
          <Text fontSize="$4" fontWeight="700">
            ✅ System Signals
          </Text>
          <XStack gap="$2" flexWrap="wrap">
            {signalRows.map((signal) => (
              <Pill
                key={signal.label}
                label={`${signal.on ? '●' : '○'} ${signal.label}`}
                tone={signal.on === true ? 'success' : signal.on === false ? 'neutral' : 'warning'}
              />
            ))}
          </XStack>
        </Card>
      </XStack>

      <Card gap="$2">
        <Text fontSize="$4" fontWeight="700">
          Connection
        </Text>
        <Text opacity={0.8}>Engine: {telemetry.connectionStatus}</Text>
        <Text opacity={0.8}>Staleness: {snapshot?.stale ? 'STALE (>5s)' : 'fresh'}</Text>
        <Text opacity={0.8}>Serial: {deviceQuery.data?.serialNumber ?? '—'}</Text>
      </Card>
            </YStack>
          </div>
        ) : (
          <ScrollView
            style={{ flex: 1 }}
            contentContainerStyle={{ paddingBottom: 16 }}
            showsVerticalScrollIndicator
          >
            <YStack gap="$3">
              <Card gap="$3">
                <XStack
                  gap="$4"
                  alignItems={isTablet ? 'stretch' : 'flex-start'}
                  flexDirection={isTablet ? 'row' : 'column'}
                >
                  {deviceAsset ? (
                    <YStack
                      width={isTablet ? mediaColumnWidth : '100%'}
                      height={mediaBoxHeight}
                      flexShrink={0}
                      borderRadius="$4"
                      overflow="hidden"
                      backgroundColor="rgba(120,120,128,0.12)"
                      alignItems="center"
                      justifyContent="center"
                    >
                      {deviceAsset.uri && !detailImageFailed ? (
                        <CachedImage
                          uri={deviceAsset.uri}
                          style={{
                            width: (isTablet ? mediaColumnWidth : mobileImageSize) * (isTablet ? desktopScale : 1.35),
                            height: mediaBoxHeight * (isTablet ? desktopScale : 1.35),
                            transform: isTablet ? [{ translateY: desktopOffsetY }] : undefined
                          }}
                          contentFit="cover"
                          onError={() => setDetailImageFailed(true)}
                        />
                      ) : detailFallback ? (
                        <ExpoImage
                          source={detailFallback}
                          style={{
                            width: (isTablet ? mediaColumnWidth : mobileImageSize) * (isTablet ? desktopScale : 1.35),
                            height: mediaBoxHeight * (isTablet ? desktopScale : 1.35),
                            transform: isTablet ? [{ translateY: desktopOffsetY }] : undefined
                          }}
                          contentFit="cover"
                        />
                      ) : null}
                    </YStack>
                  ) : null}

                  <YStack gap="$3" flex={1} minWidth={0}>
                    <XStack alignItems="flex-end" gap="$3">
                      <XStack flex={1} minWidth={0}>
                        <SocBar value={snapshot?.metrics?.soc ?? deviceQuery.data?.batteryPct} fullWidth />
                      </XStack>
                      <Text fontSize="$3" opacity={0.75} marginBottom="$1" flexShrink={0}>
                        {capacityKWh !== null ? `🔋 ${formatKWh(capacityKWh)}` : '🔋 n/a'}
                      </Text>
                    </XStack>
                    <XStack flexWrap="wrap" marginHorizontal={-4}>
                      <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                        <Stat label="∿ AC" value={formatW(acInW)} tone={isNearZero(acInW) ? 'muted' : 'default'} />
                      </YStack>
                      <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                        <Stat label="⎓ DC" value={formatW(dcW)} tone={isNearZero(dcW) ? 'muted' : 'default'} />
                      </YStack>
                      <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                        <Stat label="☼ PV" value={formatW(snapshot?.metrics?.pvW)} tone={isNearZero(snapshot?.metrics?.pvW) ? 'muted' : 'default'} />
                      </YStack>
                      <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                        <SolarTodayBadge valueWh={deviceQuery.data?.solarTodayWh} compact fitCell />
                      </YStack>
                      <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                        <Stat label="⌂ Load" value={formatW(snapshot?.metrics?.loadW)} tone={isNearZero(snapshot?.metrics?.loadW) ? 'muted' : 'default'} />
                      </YStack>
                      <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                        <Stat label="⚖ Net" value={formatW(netW)} />
                      </YStack>
                      <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                        <Stat label="🔋 Battery" value={formatW(snapshot?.metrics?.batteryW)} />
                      </YStack>
                      <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                        <Stat
                          label={isColdTemp ? '❄ Temp' : '🌡 Temp'}
                          value={snapshot?.metrics ? `${snapshot.metrics.tempC.toFixed(1)}°C` : '—'}
                          tone={isColdTemp ? 'cold' : 'default'}
                        />
                      </YStack>
                      <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                        <Stat
                          label="◉ State"
                          value={deviceQuery.data ? detailState : '—'}
                        />
                      </YStack>
                      <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
                        <Stat label="⏱ ETA" value={formatEtaMinutes(deviceQuery.data?.etaMinutes)} />
                      </YStack>
                    </XStack>
                    <XStack justifyContent="flex-end" alignItems="center" gap="$2">
                      {deviceQuery.data ? (
                        <PowerFlowGlyph
                          status={detailState}
                          pvW={snapshot?.metrics?.pvW ?? deviceQuery.data?.pvW}
                          loadW={snapshot?.metrics?.loadW ?? deviceQuery.data?.loadW}
                          fontSize="$6"
                          lineHeight={24}
                        />
                      ) : null}
                      <Text fontSize="$3" opacity={0.9}>
                        {connectionGlyph}
                      </Text>
                    </XStack>
                  </YStack>
                </XStack>
              </Card>

              {isTablet ? (
                <XStack gap="$3" alignItems="stretch" flexWrap="nowrap">
                  <YStack
                    flexBasis="50%"
                    minWidth="50%"
                    maxWidth="50%"
                    gap="$2"
                    padding="$3"
                    borderRadius="$3"
                    borderWidth={1}
                    borderColor="rgba(120,120,128,0.24)"
                  >
                    <XStack alignItems="center" justifyContent="space-between">
                      <Text fontSize="$4" fontWeight="700">
                        ☼ Solar Generated (6am-6pm, 10m buckets)
                      </Text>
                      <Text fontSize="$2" opacity={0.72}>
                        1m refresh
                      </Text>
                    </XStack>
                    <SolarGeneratedChart valuesWh={solarGeneratedTrend} points={SOLAR_GENERATED_POINTS} />
                  </YStack>
                  <YStack
                    flexBasis="50%"
                    minWidth="50%"
                    maxWidth="50%"
                    gap="$2"
                    padding="$3"
                    borderRadius="$3"
                    borderWidth={1}
                    borderColor="rgba(120,120,128,0.24)"
                  >
                    <XStack alignItems="center" justifyContent="space-between">
                      <Text fontSize="$4" fontWeight="700">
                        Power Trends
                      </Text>
                      <Text
                        fontSize="$2"
                        opacity={0.88}
                        paddingHorizontal="$2"
                        paddingVertical="$1"
                        borderRadius="$3"
                        borderWidth={1}
                        borderColor="rgba(120,120,128,0.3)"
                        onPress={toggleTrendChartStyle}
                        cursor="pointer"
                      >
                        {trendChartStyle === 'premium' ? 'Premium' : 'ASCII'}
                      </Text>
                    </XStack>
                    <PowerTrendChart
                      solar={sparklinePV}
                      ac={sparklineAC}
                      dc={sparklineDC}
                      load={sparklineLoad}
                      points={DETAIL_TREND_POINTS}
                      style={trendChartStyle}
                    />
                  </YStack>
                </XStack>
              ) : (
                <YStack gap="$3">
                  <Card gap="$2">
                    <XStack alignItems="center" justifyContent="space-between">
                      <Text fontSize="$4" fontWeight="700">
                        ☼ Solar Generated (6am-6pm, 10m buckets)
                      </Text>
                      <Text fontSize="$2" opacity={0.72}>
                        1m refresh
                      </Text>
                    </XStack>
                    <SolarGeneratedChart valuesWh={solarGeneratedTrend} points={SOLAR_GENERATED_POINTS} />
                  </Card>
                  <Card gap="$2">
                    <XStack alignItems="center" justifyContent="space-between">
                      <Text fontSize="$4" fontWeight="700">
                        Power Trends
                      </Text>
                      <Text
                        fontSize="$2"
                        opacity={0.88}
                        paddingHorizontal="$2"
                        paddingVertical="$1"
                        borderRadius="$3"
                        borderWidth={1}
                        borderColor="rgba(120,120,128,0.3)"
                        onPress={toggleTrendChartStyle}
                        cursor="pointer"
                      >
                        {trendChartStyle === 'premium' ? 'Premium' : 'ASCII'}
                      </Text>
                    </XStack>
                    <PowerTrendChart
                      solar={sparklinePV}
                      ac={sparklineAC}
                      dc={sparklineDC}
                      load={sparklineLoad}
                      points={DETAIL_TREND_POINTS}
                      style={trendChartStyle}
                    />
                  </Card>
                </YStack>
              )}

              <XStack gap="$3" flexWrap="wrap">
                <Card gap="$3" flex={1} minWidth={isDesktop ? 320 : 280}>
                  <XStack justifyContent="space-between" alignItems="center">
                    <Text fontSize="$4" fontWeight="700">
                      🔋 Battery Packs
                    </Text>
                    <Pill
                      label={`${details?.bpCount ?? packRows.length ?? 0} packs`}
                      tone="info"
                    />
                  </XStack>
                  {packRows.length ? (
                    <YStack gap="$2">
                      {packRows.map((pack) => (
                        <YStack
                          key={pack.id}
                          gap="$1"
                          padding="$2"
                          borderRadius="$3"
                          borderWidth={1}
                          borderColor="rgba(120,120,128,0.24)"
                        >
                          <XStack alignItems="center" justifyContent="space-between">
                            <Text fontWeight="700">{pack.id.toUpperCase()}</Text>
                            <Text opacity={0.8}>
                              {Number.isFinite(pack.powerW as number) ? formatW(pack.powerW) : '—'}
                              {' · '}
                              {Number.isFinite(pack.tempC as number) ? `${pack.tempC?.toFixed(1)}°C` : '—'}
                            </Text>
                          </XStack>
                          <SocBar value={pack.socPct} fullWidth />
                        </YStack>
                      ))}
                    </YStack>
                  ) : (
                    <Text opacity={0.7}>No per-pack telemetry yet.</Text>
                  )}
                </Card>

                <Card gap="$3" flex={1} minWidth={isDesktop ? 320 : 280}>
                  <Text fontSize="$4" fontWeight="700">
                    ☀ Solar Inputs
                  </Text>
                  <YStack gap="$2">
                    {solarRows.map((port) => (
                      (() => {
                        const portInactive = isInactivePvPort(port.volts);
                        const uiState = getSolarPortUiState(port);
                        const pvLoadPct = toPctOfMax(port.watts, port.maxWatts);
                        const barPct = clampPercent(pvLoadPct ?? 0);
                        return (
                      <YStack
                        key={port.id}
                        gap="$2"
                        padding="$2"
                        borderRadius="$3"
                        borderWidth={1}
                        borderColor="rgba(120,120,128,0.24)"
                        opacity={portInactive ? 0.72 : 1}
                      >
                        <XStack justifyContent="space-between" alignItems="center">
                          <Text fontWeight="700">{port.name}</Text>
                          <Pill
                            label={uiState.label}
                            tone={uiState.tone}
                          />
                        </XStack>
                        <XStack gap="$3" flexWrap="wrap">
                          <Stat
                            label="⚡ W"
                            value={formatW(port.watts)}
                            tone={portInactive || isNearZero(port.watts) ? 'muted' : 'default'}
                          />
                          <Stat
                            label="V"
                            value={Number.isFinite(port.volts as number) ? `${port.volts?.toFixed(1)}V` : '—'}
                            tone={portInactive ? 'muted' : 'default'}
                          />
                          <Stat
                            label="A"
                            value={Number.isFinite(port.amps as number) ? `${port.amps?.toFixed(2)}A` : '—'}
                            tone={portInactive ? 'muted' : 'default'}
                          />
                          <Stat
                            label="Cap"
                            value={
                              port.maxWatts
                                ? `${port.maxWatts}W · ${port.maxVolts ?? '—'}V · ${port.maxAmps ?? '—'}A`
                                : '—'
                            }
                            tone={portInactive ? 'muted' : 'default'}
                          />
                        </XStack>
                        <YStack gap="$1">
                          <XStack alignItems="center" justifyContent="space-between">
                            <Text opacity={portInactive ? 0.6 : 0.85} fontWeight="600">
                              PV Load
                            </Text>
                            <Text opacity={portInactive ? 0.6 : 0.9} fontWeight="700">
                              {pvLoadPct === null ? '—' : `${barPct.toFixed(1)}%`}
                            </Text>
                          </XStack>
                          <XStack
                            height={10}
                            borderRadius="$5"
                            overflow="hidden"
                            backgroundColor="rgba(255,159,10,0.14)"
                          >
                            <XStack
                              height="100%"
                              width={`${barPct}%` as `${number}%`}
                              opacity={portInactive ? 0.5 : 1}
                              style={{ backgroundColor: pvLoadColor(barPct) }}
                            />
                          </XStack>
                        </YStack>
                      </YStack>
                        );
                      })()
                    ))}
                  </YStack>
                </Card>
              </XStack>

              <XStack gap="$3" flexWrap="wrap">
                <Card gap="$3" flex={1} minWidth={isDesktop ? 360 : 280}>
                  <Text fontSize="$4" fontWeight="700">
                    🧭 Estimate & Queue
                  </Text>
                  <XStack gap="$2" flexWrap="wrap">
                    <Pill label={`mode: ${estimateLabel}`} tone="neutral" />
                    <Pill
                      label={`eta: ${formatEtaMinutes(details?.estimateEtaMin ?? deviceQuery.data?.etaMinutes)}`}
                      tone="info"
                    />
                    <Pill
                      label={`queue: ${details?.mqttQueueDepth ?? 0}`}
                      tone={(details?.mqttQueueDepth ?? 0) > 48 ? 'warning' : 'success'}
                    />
                    <Pill
                      label={`dropped: ${details?.mqttQueueDroppedOldest ?? 0}`}
                      tone={(details?.mqttQueueDroppedOldest ?? 0) > 0 ? 'danger' : 'neutral'}
                    />
                  </XStack>
                </Card>
                <Card gap="$3" flex={1} minWidth={isDesktop ? 360 : 280}>
                  <Text fontSize="$4" fontWeight="700">
                    ✅ System Signals
                  </Text>
                  <XStack gap="$2" flexWrap="wrap">
                    {signalRows.map((signal) => (
                      <Pill
                        key={signal.label}
                        label={`${signal.on ? '●' : '○'} ${signal.label}`}
                        tone={signal.on === true ? 'success' : signal.on === false ? 'neutral' : 'warning'}
                      />
                    ))}
                  </XStack>
                </Card>
              </XStack>

              <Card gap="$2">
                <Text fontSize="$4" fontWeight="700">
                  Connection
                </Text>
                <Text opacity={0.8}>Engine: {telemetry.connectionStatus}</Text>
                <Text opacity={0.8}>Staleness: {snapshot?.stale ? 'STALE (>5s)' : 'fresh'}</Text>
                <Text opacity={0.8}>Serial: {deviceQuery.data?.serialNumber ?? '—'}</Text>
              </Card>
            </YStack>
          </ScrollView>
        )}
      </YStack>
      </YStack>
    </Animated.View>
  );
}
