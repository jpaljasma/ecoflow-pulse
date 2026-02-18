import { router } from 'expo-router';
import { Image, Platform } from 'react-native';
import { useMemo } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot, TelemetryEngineStatus } from '@/features/telemetry/engine/types';
import { Card } from '@/shared/ui/Card';
import { PowerFlowGlyph } from '@/shared/ui/PowerFlowGlyph';
import { Stat } from '@/shared/ui/Stat';
import { formatEtaMinutes, formatW } from '@/features/telemetry/format';
import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';
import { getCapacityKWh } from '@/features/devices/capacity';
import { SocBar } from '@/shared/ui/SocBar';
import { getEcoFlowAsset, getEcoFlowDefaultSize } from '@/shared/assets/ecoflowAssets';
import { getStatusGlyph } from '@/shared/ui/statusGlyph';

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
  const metrics = snapshot?.metrics;
  const pvW = metrics?.pvW ?? device.pvW;
  const acInW = device.acInW;
  const dcW = device.dcW;
  const loadW = metrics?.loadW ?? device.loadW;
  const netW = metrics ? metrics.pvW - metrics.loadW : (device.netW ?? (pvW !== undefined && loadW !== undefined ? pvW - loadW : undefined));
  const match = getDeviceAssetMatch(device.model);
  const cardImage = match.slug ? getEcoFlowAsset(match.slug, getEcoFlowDefaultSize(imageContext)) : null;
  const imageSource = useMemo(() => (cardImage ? { uri: cardImage } : undefined), [cardImage]);
  const iconSize = 84;

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

  return (
    <Card
      pressStyle={{ scale: 0.995, opacity: 0.95 }}
      onPress={() => router.push(`/device/${device.id}`)}
      role="button"
      cursor="pointer"
    >
      <XStack gap="$3" alignItems="stretch">
        <YStack width={iconSize} flexShrink={0} alignItems="center" gap="$2">
          <YStack
            width={iconSize}
            height={iconSize}
            alignItems="center"
            justifyContent="center"
            borderRadius="$4"
            backgroundColor="rgba(120,120,128,0.12)"
            overflow="hidden"
          >
            {imageSource && Platform.OS === 'web' ? (
              <Image
                source={imageSource}
                style={{ width: iconSize * 1.45, height: iconSize * 1.05 }}
                resizeMode="cover"
              />
            ) : (
              <Text fontSize="$10">{match.glyph.emoji}</Text>
            )}
          </YStack>
          <Text fontSize="$2" opacity={0.75} textAlign="center" numberOfLines={1}>
            {capacityKWh !== null ? `🔋 ${capacityKWh.toFixed(1)}kWh` : '🔋 n/a'}
          </Text>
          <PowerFlowGlyph
            status={snapshotState}
            pvW={snapshot?.metrics?.pvW ?? device.pvW}
            loadW={snapshot?.metrics?.loadW ?? device.loadW}
            fontSize="$7"
            lineHeight={30}
          />
        </YStack>

        <YStack gap="$3" flex={1} justifyContent="space-between">
          <XStack alignItems="flex-start" gap="$2">
            <Text fontFamily="$heading" fontSize="$7" fontWeight="700" numberOfLines={1} flex={1}>
              {device.name}
            </Text>
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

          <XStack justifyContent="flex-end">
            <Text fontSize="$3" opacity={0.9}>
              {connGlyph}
            </Text>
          </XStack>
        </YStack>
      </XStack>
    </Card>
  );
}
