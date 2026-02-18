import { router } from 'expo-router';
import { Image, Platform } from 'react-native';
import { useMemo } from 'react';
import { Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';
import { Card } from '@/shared/ui/Card';
import { Pill } from '@/shared/ui/Pill';
import { Stat } from '@/shared/ui/Stat';
import { formatEtaMinutes, formatW } from '@/features/telemetry/format';
import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';
import { SocBar } from '@/shared/ui/SocBar';
import { getEcoFlowAsset, getEcoFlowDefaultSize } from '@/shared/assets/ecoflowAssets';
import { getPowerFlowGlyph, getStatusGlyph } from '@/shared/ui/statusGlyph';

function statusTone(snapshot: DeviceSnapshot | undefined): 'neutral' | 'success' | 'warning' | 'danger' {
  if (!snapshot) return 'neutral';
  if (snapshot.stale) return 'warning';
  if (snapshot.status === 'charging') return 'success';
  if (snapshot.status === 'discharging') return 'danger';
  return 'neutral';
}

function statusLabel(snapshot: DeviceSnapshot | undefined): string {
  if (!snapshot) return getStatusGlyph('waiting');
  return getPowerFlowGlyph({
    stale: snapshot.stale,
    status: snapshot.status,
    pvW: snapshot.metrics?.pvW,
    loadW: snapshot.metrics?.loadW
  });
}

export function DeviceCard({
  device,
  snapshot,
  imageContext = 'card'
}: {
  device: DeviceSummary;
  snapshot?: DeviceSnapshot;
  imageContext?: 'list' | 'card' | 'detail';
}) {
  const metrics = snapshot?.metrics;
  const netW = metrics ? metrics.pvW - metrics.loadW : undefined;
  const match = getDeviceAssetMatch(device.model);
  const cardImage = match.slug ? getEcoFlowAsset(match.slug, getEcoFlowDefaultSize(imageContext)) : null;
  const imageSource = useMemo(() => (cardImage ? { uri: cardImage } : undefined), [cardImage]);
  const iconSize = 84;

  return (
    <Card
      pressStyle={{ scale: 0.995, opacity: 0.95 }}
      onPress={() => router.push(`/device/${device.id}`)}
      role="button"
      cursor="pointer"
    >
      <XStack gap="$3" alignItems="stretch">
        <YStack
          width={iconSize}
          height={iconSize}
          flexShrink={0}
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

        <YStack gap="$3" flex={1}>
          <XStack justifyContent="space-between" alignItems="center" gap="$2">
            <Text fontFamily="$heading" fontSize="$7" fontWeight="700" numberOfLines={1} flex={1}>
              {device.name}
            </Text>
            <Pill label={statusLabel(snapshot)} tone={statusTone(snapshot)} glyph />
          </XStack>

          <Text fontFamily="$body" fontSize="$3" opacity={0.8} numberOfLines={1}>
            {device.model} · SN {device.serialNumber}
          </Text>

          <XStack gap="$3" flexWrap="wrap" alignItems="flex-end">
            <SocBar value={metrics?.soc ?? device.batteryPct} />
            <Stat label="Net" value={formatW(netW)} />
            <Stat label="PV" value={formatW(metrics?.pvW)} />
            <Stat label="Load" value={formatW(metrics?.loadW)} />
            <Stat label="State" value={device.state} />
            <Stat label="ETA" value={formatEtaMinutes(device.etaMinutes)} />
          </XStack>
        </YStack>
      </XStack>
    </Card>
  );
}
