import { useMemo } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Image, Platform, useWindowDimensions } from 'react-native';
import { Button, Text, XStack, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { Card } from '@/shared/ui/Card';
import { PowerFlowGlyph } from '@/shared/ui/PowerFlowGlyph';
import { Stat } from '@/shared/ui/Stat';
import { AppMenu } from '@/shared/ui/AppMenu';
import { SocBar } from '@/shared/ui/SocBar';
import { SparklineTrend } from '@/shared/ui/SparklineTrend';
import { useDevice } from '@/features/devices/hooks';
import { getCapacityKWh } from '@/features/devices/capacity';
import { useTelemetrySnapshot } from '@/features/telemetry/hooks';
import { formatAgo, formatEtaMinutes, formatW } from '@/features/telemetry/format';
import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';
import { getEcoFlowAsset, getEcoFlowDefaultSize } from '@/shared/assets/ecoflowAssets';
import { getStatusGlyph } from '@/shared/ui/statusGlyph';

const DETAIL_TREND_POINTS = 60;

function isNearZero(value: number | undefined | null): boolean {
  if (value === null || value === undefined || Number.isNaN(value)) return false;
  return value >= -0.5 && value <= 0.5;
}

export default function DeviceDetailScreen() {
  const { width } = useWindowDimensions();
  const isDesktop = width >= 900;
  const router = useRouter();
  const { deviceId } = useLocalSearchParams<{ deviceId: string }>();
  const deviceQuery = useDevice(deviceId);
  const telemetry = useTelemetrySnapshot(deviceId ? [deviceId] : []);
  const snapshot = deviceId ? telemetry.byId[deviceId] : undefined;

  const sparklineLoad = useMemo(
    () => snapshot?.sparkline.loadW.map((p) => p.value) ?? [],
    [snapshot]
  );
  const sparklinePV = useMemo(
    () => snapshot?.sparkline.pvW.map((p) => p.value) ?? [],
    [snapshot]
  );
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
    const match = getDeviceAssetMatch(model);
    if (!match.slug) return null;
    return {
      uri: getEcoFlowAsset(match.slug, getEcoFlowDefaultSize('detail')),
      emoji: match.glyph.emoji
    };
  }, [deviceQuery.data?.model]);

  return (
    <YStack
      flex={1}
      backgroundColor="$background"
      paddingHorizontal="$4"
      paddingVertical="$4"
      gap="$4"
    >
      <TopBar
        left={
          <Button chromeless size="$3" onPress={() => router.back()}>
            ← Back
          </Button>
        }
        title={deviceQuery.data?.name ?? 'Device'}
        subtitle={deviceQuery.data ? `${deviceQuery.data.model} · ${formatAgo(snapshot?.lastSeenAt ?? null)}` : 'Loading…'}
        right={
          <YStack alignItems="flex-end">
            <AppMenu />
          </YStack>
        }
      />

      <Card gap="$3">
        {deviceAsset ? (
          <YStack
            borderRadius="$4"
            overflow="hidden"
            backgroundColor="rgba(120,120,128,0.12)"
            alignItems="center"
            justifyContent="center"
            padding="$2"
          >
            {Platform.OS === 'web' ? (
              <Image
                source={{ uri: deviceAsset.uri }}
                style={{ width: '100%', height: 220 }}
                resizeMode="cover"
              />
            ) : (
              <Text fontSize="$9">{deviceAsset.emoji}</Text>
            )}
          </YStack>
        ) : null}
        <XStack alignItems="flex-end" gap="$3">
          <XStack flex={1} minWidth={0}>
            <SocBar value={snapshot?.metrics?.soc ?? deviceQuery.data?.batteryPct} fullWidth />
          </XStack>
          <Text fontSize="$3" opacity={0.75} marginBottom="$1" flexShrink={0}>
            {capacityKWh !== null ? `🔋 ${capacityKWh.toFixed(1)}kWh` : '🔋 n/a'}
          </Text>
        </XStack>
        <XStack gap="$3" flexWrap="wrap" paddingRight="$2">
          <Stat label="∿ AC" value={formatW(acInW)} tone={isNearZero(acInW) ? 'muted' : 'default'} />
          <Stat label="⎓ DC" value={formatW(dcW)} tone={isNearZero(dcW) ? 'muted' : 'default'} />
          <Stat label="☼ PV" value={formatW(snapshot?.metrics?.pvW)} tone={isNearZero(snapshot?.metrics?.pvW) ? 'muted' : 'default'} />
          <Stat label="⌂ Load" value={formatW(snapshot?.metrics?.loadW)} tone={isNearZero(snapshot?.metrics?.loadW) ? 'muted' : 'default'} />
          <Stat label="⚖ Net" value={formatW(netW)} />
          <Stat label="🔋 Battery" value={formatW(snapshot?.metrics?.batteryW)} />
          <Stat
            label={isColdTemp ? '❄ Temp' : '🌡 Temp'}
            value={snapshot?.metrics ? `${snapshot.metrics.tempC.toFixed(1)}°C` : '—'}
            tone={isColdTemp ? 'cold' : 'default'}
          />
          <Stat
            label="◉ State"
            value={
              deviceQuery.data ? (
                <XStack alignItems="center" gap="$1">
                  <PowerFlowGlyph
                    status={detailState}
                    pvW={snapshot?.metrics?.pvW ?? deviceQuery.data?.pvW}
                    loadW={snapshot?.metrics?.loadW ?? deviceQuery.data?.loadW}
                    fontSize="$4"
                    lineHeight={22}
                  />
                  <Text>{detailState}</Text>
                </XStack>
              ) : '—'
            }
          />
          <Stat label="⏱ ETA" value={formatEtaMinutes(deviceQuery.data?.etaMinutes)} />
        </XStack>
        <XStack justifyContent="flex-end">
          <Text fontSize="$3" opacity={0.9}>
            {connectionGlyph}
          </Text>
        </XStack>
      </Card>

      <Card gap="$2">
        {isDesktop ? (
          <XStack gap="$3">
            <YStack
              flex={1}
              gap="$2"
              padding="$3"
              borderRadius="$3"
              borderWidth={1}
              borderColor="rgba(120,120,128,0.24)"
            >
              <Text fontSize="$4" fontWeight="700">
                Load Trend
              </Text>
              <SparklineTrend values={sparklineLoad} points={DETAIL_TREND_POINTS} />
            </YStack>
            <YStack
              flex={1}
              gap="$2"
              padding="$3"
              borderRadius="$3"
              borderWidth={1}
              borderColor="rgba(120,120,128,0.24)"
            >
              <Text fontSize="$4" fontWeight="700">
                PV Trend
              </Text>
              <SparklineTrend values={sparklinePV} points={DETAIL_TREND_POINTS} />
            </YStack>
          </XStack>
        ) : (
          <YStack gap="$3">
            <YStack
              gap="$2"
              padding="$3"
              borderRadius="$3"
              borderWidth={1}
              borderColor="rgba(120,120,128,0.24)"
            >
              <Text fontSize="$4" fontWeight="700">
                Load Trend
              </Text>
              <SparklineTrend values={sparklineLoad} points={DETAIL_TREND_POINTS} />
            </YStack>
            <YStack
              gap="$2"
              padding="$3"
              borderRadius="$3"
              borderWidth={1}
              borderColor="rgba(120,120,128,0.24)"
            >
              <Text fontSize="$4" fontWeight="700">
                PV Trend
              </Text>
              <SparklineTrend values={sparklinePV} points={DETAIL_TREND_POINTS} />
            </YStack>
          </YStack>
        )}
      </Card>

      <Card gap="$2">
        <Text fontSize="$4" fontWeight="700">
          Connection
        </Text>
        <Text opacity={0.8}>Engine: {telemetry.connectionStatus}</Text>
        <Text opacity={0.8}>Staleness: {snapshot?.stale ? 'STALE (>5s)' : 'fresh'}</Text>
        <Text opacity={0.8}>Serial: {deviceQuery.data?.serialNumber ?? '—'}</Text>
      </Card>
    </YStack>
  );
}
