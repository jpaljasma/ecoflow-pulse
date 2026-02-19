import { router } from 'expo-router';
import { Animated, Platform, useWindowDimensions } from 'react-native';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Image as ExpoImage } from 'expo-image';
import { Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot, TelemetryEngineStatus } from '@/features/telemetry/engine/types';
import { Card } from '@/shared/ui/Card';
import { PowerFlowGlyph } from '@/shared/ui/PowerFlowGlyph';
import { Stat } from '@/shared/ui/Stat';
import { formatAgo, formatEtaMinutes, formatW } from '@/features/telemetry/format';
import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';
import { getCapacityKWh } from '@/features/devices/capacity';
import { SocBar } from '@/shared/ui/SocBar';
import { getEcoFlowAsset, getEcoFlowDefaultSize } from '@/shared/assets/ecoflowAssets';
import { getStatusGlyph } from '@/shared/ui/statusGlyph';
import { CachedImage } from '@/shared/ui/CachedImage';
import { getBundledDeviceFallback } from '@/shared/assets/deviceFallbacks';
import { env } from '@/shared/config/env';

function isNearZero(value: number | undefined | null): boolean {
  if (value === null || value === undefined || Number.isNaN(value)) return false;
  return value >= -0.5 && value <= 0.5;
}

function connectivityGlyph(
  snapshot: DeviceSnapshot | undefined,
  connectionStatus: TelemetryEngineStatus
): string {
  if (snapshot?.stale) return getStatusGlyph('stale');
  if (connectionStatus === 'connecting' || connectionStatus === 'reconnecting') {
    return getStatusGlyph('processing');
  }
  if (connectionStatus === 'connected' && snapshot?.online) return getStatusGlyph('online');
  return getStatusGlyph('waiting');
}

export function DeviceCard({
  device,
  snapshot,
  imageContext = 'card',
  connectionStatus
}: {
  device: DeviceSummary;
  snapshot?: DeviceSnapshot;
  imageContext?: 'list' | 'card' | 'detail';
  connectionStatus: TelemetryEngineStatus;
}) {
  const { width } = useWindowDimensions();
  const [nowMs, setNowMs] = useState(() => Date.now());
  const isTabletUp = width >= 768;
  const isDesktopWide = width >= 1200;
  const metrics = snapshot?.metrics;
  const pvW = metrics?.pvW ?? device.pvW;
  const acInW = device.acInW;
  const dcW = device.dcW;
  const loadW = metrics?.loadW ?? device.loadW;
  const netW = metrics ? metrics.pvW - metrics.loadW : (device.netW ?? (pvW !== undefined && loadW !== undefined ? pvW - loadW : undefined));
  const batteryCount =
    device.details?.bpCount ??
    ((device.capabilities as { batteryPacks?: number } | undefined)?.batteryPacks ?? 1);
  const match = getDeviceAssetMatch(device.model, { batteryCount });
  const useRemoteImage = Boolean(env.assetBaseUrl);
  const imageUri = useMemo(
    () =>
      useRemoteImage && match.slug
        ? getEcoFlowAsset(match.slug, getEcoFlowDefaultSize(imageContext))
        : undefined,
    [useRemoteImage, match.slug, imageContext]
  );
  const fallbackSource = useMemo(
    () => (match.slug ? getBundledDeviceFallback(match.slug, '256') : undefined),
    [match.slug]
  );
  const [imageFailed, setImageFailed] = useState(false);
  const imageBoxSize = isDesktopWide ? 106 : isTabletUp ? 99 : 92;
  const railWidth = isDesktopWide ? 124 : isTabletUp ? 117 : 106;

  const fallbackStatus =
    device.state === 'charging' || device.state === 'discharging' || device.state === 'idle'
      ? device.state
      : 'idle';
  const snapshotState =
    snapshot && !snapshot.stale && snapshot.status !== 'stale'
      ? snapshot.status
      : fallbackStatus;
  const connGlyph = connectivityGlyph(snapshot, connectionStatus);
  const capacityKWh = getCapacityKWh(device);
  const lastSeenAtCandidates = [snapshot?.lastSeenAt ?? 0, device.telemetryTsMs ?? 0];
  const freshestLastSeenAt = Math.max(...lastSeenAtCandidates);
  const lastSeenAt = freshestLastSeenAt > 0 ? freshestLastSeenAt : null;
  const isInactive = lastSeenAt !== null && nowMs - lastSeenAt > 60_000;
  const fadeOpacity = useRef(new Animated.Value(isInactive ? 0.46 : 1)).current;

  useEffect(() => {
    const timer = setInterval(() => setNowMs(Date.now()), 1_000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    Animated.timing(fadeOpacity, {
      toValue: isInactive ? 0.46 : 1,
      duration: isInactive ? 900 : 220,
      useNativeDriver: Platform.OS !== 'web'
    }).start();
  }, [fadeOpacity, isInactive]);

  useEffect(() => {
    setImageFailed(false);
  }, [imageUri]);

  return (
    <Animated.View style={{ opacity: fadeOpacity }}>
      <Card
        hoverStyle={
          isInactive
            ? undefined
            : {
                transform: [{ translateY: -2 }],
                shadowOpacity: 0.14,
                borderColor: 'rgba(10,132,255,0.28)'
              }
        }
        pressStyle={{ scale: 0.995, opacity: 0.95 }}
        onPress={() => router.push(`/device/${device.id}`)}
        role="button"
        cursor="pointer"
        backgroundColor={isInactive ? 'rgba(120,120,128,0.08)' : '$background'}
      >
        <XStack gap="$3" alignItems="stretch">
          <YStack width={railWidth} flexShrink={0} alignItems="center" gap="$2">
            <YStack
              width={imageBoxSize}
              height={imageBoxSize}
              marginTop={2}
              alignItems="center"
              justifyContent="center"
              borderRadius="$4"
              backgroundColor="rgba(120,120,128,0.12)"
              overflow="hidden"
            >
              {imageUri && !imageFailed ? (
                <CachedImage
                  uri={imageUri}
                  style={{ width: imageBoxSize * 1.08, height: imageBoxSize * 1.08 }}
                  contentFit="cover"
                  onError={() => setImageFailed(true)}
                />
              ) : fallbackSource ? (
                <ExpoImage
                  source={fallbackSource}
                  style={{ width: imageBoxSize * 1.08, height: imageBoxSize * 1.08 }}
                  contentFit="cover"
                />
              ) : (
                <Text fontSize="$10">{match.glyph.emoji}</Text>
              )}
            </YStack>
            <Text fontSize="$3" opacity={0.75} textAlign="center" numberOfLines={1}>
              {capacityKWh !== null ? `🔋 ${capacityKWh.toFixed(1)}kWh` : '🔋 n/a'}
            </Text>
            <PowerFlowGlyph
              status={snapshotState}
              pvW={snapshot?.metrics?.pvW ?? device.pvW}
              loadW={snapshot?.metrics?.loadW ?? device.loadW}
              fontSize="$8"
              lineHeight={32}
            />
          </YStack>

          <YStack gap="$3" flex={1} justifyContent="space-between">
            <XStack alignItems="flex-start" gap="$2">
              <Text fontFamily="$heading" fontSize="$7" fontWeight="700" numberOfLines={1} flex={1}>
                {device.name}
              </Text>
              {isInactive ? (
                <Text fontSize="$2" color="rgba(60,60,67,0.65)" marginTop="$1" flexShrink={0}>
                  (inactive)
                </Text>
              ) : null}
            </XStack>

            <Text fontFamily="$body" fontSize="$3" opacity={0.8} numberOfLines={1}>
              {device.model} · SN {device.serialNumber}
            </Text>

            <XStack gap="$3" flexWrap="wrap" alignItems="flex-end">
              <SocBar value={metrics?.soc ?? device.batteryPct} />
              <Stat label="AC" value={formatW(acInW)} tone={isNearZero(acInW) ? 'muted' : 'default'} />
              <Stat label="DC" value={formatW(dcW)} tone={isNearZero(dcW) ? 'muted' : 'default'} />
              <Stat label="PV" value={formatW(pvW)} tone={isNearZero(pvW) ? 'muted' : 'default'} />
              <Stat label="Load" value={formatW(loadW)} tone={isNearZero(loadW) ? 'muted' : 'default'} />
              <Stat label="Net" value={formatW(netW)} />
              <Stat label="⏱ ETA" value={formatEtaMinutes(device.etaMinutes)} />
            </XStack>

          <XStack justifyContent="space-between" alignItems="center">
            <Text fontSize={10} opacity={0.48} numberOfLines={1}>
              Last seen {formatAgo(lastSeenAt)}
            </Text>
            <Text fontSize="$3" opacity={0.9}>
              {isInactive ? `Inactive · ${connGlyph}` : connGlyph}
            </Text>
          </XStack>
          </YStack>
        </XStack>
      </Card>
    </Animated.View>
  );
}
