import { router } from 'expo-router';
import { Animated, Platform, useWindowDimensions } from 'react-native';
import { useEffect, useMemo, useRef } from 'react';
import { Button, Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot, TelemetryEngineStatus } from '@/features/telemetry/engine/types';
import { Card } from '@/shared/ui/Card';
import { DeviceHeroPanel } from '@/shared/ui/DeviceHeroPanel';
import { PowerFlowGlyph } from '@/shared/ui/PowerFlowGlyph';
import { Stat } from '@/shared/ui/Stat';
import { formatAgo, formatEtaMinutes, formatKWh, formatW, maskSerialNumber } from '@/features/telemetry/format';
import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';
import { getCapacityKWh } from '@/features/devices/capacity';
import { SocBar } from '@/shared/ui/SocBar';
import { getEcoFlowAsset, getEcoFlowDefaultSize } from '@/shared/assets/ecoflowAssets';
import { getStatusGlyph } from '@/shared/ui/statusGlyph';
import { getBundledDeviceFallback } from '@/shared/assets/deviceFallbacks';
import { env } from '@/shared/config/env';
import { SolarTodayBadge } from '@/shared/ui/SolarTodayBadge';
import { MetricsGrid, type MetricsGridItem } from '@/shared/ui/MetricsGrid';
import { isMutedMetric } from '@/shared/ui/uiMappings';
import { useTelemetryDeviceSnapshot } from '@/features/telemetry/hooks';
import { useAuthSession } from '@/features/auth/hooks';
import { useDeviceSolarHistory } from '@/features/history/hooks';

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

function getMaxSolarWatts(device: DeviceSummary): number | undefined {
  const total = device.details?.solarPorts?.reduce((sum, port) => sum + (port.maxWatts ?? 0), 0) ?? 0;
  return total > 0 ? total : undefined;
}

export function DeviceCard({
  device,
  imageContext = 'card',
  connectionStatus
}: {
  device: DeviceSummary;
  imageContext?: 'list' | 'card' | 'detail';
  connectionStatus: TelemetryEngineStatus;
}) {
  const { width } = useWindowDimensions();
  const snapshot = useTelemetryDeviceSnapshot(device.id);
  const { authConfigured, authReady, authKey, sessionValid, token } = useAuthSession();
  const historyEnabled = authReady && (!authConfigured || sessionValid);
  const maxSolarWatts = getMaxSolarWatts(device);
  const solarHistory = useDeviceSolarHistory(device.id, {
    token,
    authKey,
    enabled: historyEnabled,
    maxSolarWatts
  });
  const isPhoneCompact = width < 460;
  const isTabletUp = width >= 768;
  const isDesktopWide = width >= 1200;
  const metrics = snapshot?.metrics;
  const pvW = metrics?.pvW ?? device.pvW;
  const acInW = metrics?.acW ?? device.acInW;
  const dcW = metrics?.dcW ?? device.dcW;
  const loadW = metrics?.loadW ?? device.loadW;
  const netW =
    metrics
      ? metrics.pvW - metrics.loadW
      : device.netW ?? (pvW !== undefined && loadW !== undefined ? pvW - loadW : undefined);
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
  const imageBoxSize = isDesktopWide ? 106 : isTabletUp ? 98 : 82;
  const railWidth = isDesktopWide ? 124 : isTabletUp ? 112 : 94;

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
  const isInactive =
    snapshot?.inactive ??
    (lastSeenAt !== null ? Date.now() - lastSeenAt > 60_000 : false);
  const fadeOpacity = useRef(new Animated.Value(isInactive ? 0.46 : 1)).current;

  useEffect(() => {
    Animated.timing(fadeOpacity, {
      toValue: isInactive ? 0.46 : 1,
      duration: isInactive ? 900 : 220,
      useNativeDriver: Platform.OS !== 'web'
    }).start();
  }, [fadeOpacity, isInactive]);

  const metricItems = useMemo<MetricsGridItem[]>(() => {
    return [
      {
        key: 'ac',
        content: (
          <Stat
            label="AC"
            value={formatW(acInW)}
            tone={isMutedMetric(acInW) ? 'muted' : 'default'}
            compact
          />
        )
      },
      {
        key: 'dc',
        content: (
          <Stat
            label="DC"
            value={formatW(dcW)}
            tone={isMutedMetric(dcW) ? 'muted' : 'default'}
            compact
          />
        )
      },
      {
        key: 'pv',
        content: (
          <Stat
            label="PV"
            value={formatW(pvW)}
            tone={isMutedMetric(pvW) ? 'muted' : 'default'}
            compact
          />
        )
      },
      {
        key: 'today',
        content: (
          <SolarTodayBadge
            valueWh={solarHistory.data?.todayWh}
            deltaPct={solarHistory.data?.deltaPct}
            compact
            fitCell
          />
        )
      },
      {
        key: 'load',
        content: (
          <Stat
            label="Load"
            value={formatW(loadW)}
            tone={isMutedMetric(loadW) ? 'muted' : 'default'}
            compact
          />
        )
      },
      {
        key: 'net',
        content: <Stat label="Net" value={formatW(netW)} compact />
      },
      {
        key: 'eta',
        content: <Stat label="⏱ ETA" value={formatEtaMinutes(device.etaMinutes)} compact />
      }
    ];
  }, [
    acInW,
    dcW,
    device.etaMinutes,
    loadW,
    netW,
    pvW,
    solarHistory.data?.deltaPct,
    solarHistory.data?.todayWh
  ]);

  return (
    <Animated.View style={{ opacity: fadeOpacity }}>
      <Card
        testID={`device-card-${device.id}`}
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
        <DeviceHeroPanel
          leftWidth={railWidth}
          imageWidth={imageBoxSize}
          imageHeight={imageBoxSize}
          imageScale={1.08}
          imageUri={imageUri}
          fallbackSource={fallbackSource}
          emojiFallback={match.glyph.emoji}
          leftMeta={(
            <Text
              fontSize={isPhoneCompact ? 11 : 13}
              opacity={0.75}
              textAlign="center"
              numberOfLines={1}
              marginTop="$1"
            >
              {capacityKWh !== null ? `🔋 ${formatKWh(capacityKWh)}` : '🔋 n/a'}
            </Text>
          )}
          leftFooter={(
            <YStack marginTop={isPhoneCompact ? '$3' : '$2'}>
              <PowerFlowGlyph
                status={snapshotState}
                pvW={snapshot?.metrics?.pvW ?? device.pvW}
                loadW={snapshot?.metrics?.loadW ?? device.loadW}
                fontSize={isPhoneCompact ? '$7' : '$8'}
                lineHeight={isPhoneCompact ? 26 : 30}
              />
            </YStack>
          )}
          right={(
            <YStack gap="$3" flex={1} justifyContent="space-between">
              <XStack alignItems="flex-start" gap="$2">
                <Text
                  fontFamily="$heading"
                  fontSize={isPhoneCompact ? '$6' : '$7'}
                  fontWeight="700"
                  numberOfLines={1}
                  flex={1}
                >
                  {device.name}
                </Text>
                <Button
                  size="$2"
                  borderRadius={999}
                  borderWidth={1}
                  paddingHorizontal="$3"
                  minHeight={32}
                  backgroundColor="rgba(10,132,255,0.08)"
                  borderColor="rgba(10,132,255,0.18)"
                  onPress={(event: any) => {
                    event?.stopPropagation?.();
                    router.push({
                      pathname: '/(tabs)/energy',
                      params: {
                        device: device.id,
                        preset: 'today',
                        compare: '1'
                      }
                    });
                  }}
                >
                  ⚡ Energy
                </Button>
                {isInactive ? (
                  <Text fontSize="$2" color="rgba(60,60,67,0.65)" marginTop="$1" flexShrink={0}>
                    (inactive)
                  </Text>
                ) : null}
              </XStack>

              <Text fontFamily="$body" fontSize={isPhoneCompact ? '$2' : '$3'} opacity={0.8} numberOfLines={1}>
                {device.model} · SN {maskSerialNumber(device.serialNumber)}
              </Text>

              <YStack gap="$2">
                <SocBar value={metrics?.soc ?? device.batteryPct} />
                <MetricsGrid items={metricItems} columns={3} />
              </YStack>

              <XStack justifyContent="space-between" alignItems="center">
                <Text fontSize={10} opacity={0.48} numberOfLines={1}>
                  Last seen {formatAgo(lastSeenAt)}
                </Text>
                <Text fontSize="$3" opacity={0.9}>
                  {isInactive ? `Inactive · ${connGlyph}` : connGlyph}
                </Text>
              </XStack>
            </YStack>
          )}
        />
      </Card>
    </Animated.View>
  );
}
