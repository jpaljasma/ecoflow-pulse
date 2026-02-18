import { useMemo } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Image, Platform } from 'react-native';
import { Button, Text, XStack, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { Card } from '@/shared/ui/Card';
import { Pill } from '@/shared/ui/Pill';
import { Stat } from '@/shared/ui/Stat';
import { AppMenu } from '@/shared/ui/AppMenu';
import { SocBar } from '@/shared/ui/SocBar';
import { useDevice } from '@/features/devices/hooks';
import { useTelemetrySnapshot } from '@/features/telemetry/hooks';
import { formatAgo, formatEtaMinutes, formatW } from '@/features/telemetry/format';
import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';
import { getEcoFlowAsset, getEcoFlowDefaultSize } from '@/shared/assets/ecoflowAssets';
import { getStatusGlyph } from '@/shared/ui/statusGlyph';

function SparklinePlaceholder({ values }: { values: number[] }) {
  if (!values.length) {
    return <Text opacity={0.6}>Waiting for chart data…</Text>;
  }

  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const normalized = values.map((v) => ((v - min) / range) * 36);

  return (
    <Text fontSize="$3" opacity={0.85}>
      {normalized.map((v) => (v > 28 ? '█' : v > 18 ? '▓' : v > 10 ? '▒' : '░')).join('')}
    </Text>
  );
}

export default function DeviceDetailScreen() {
  const router = useRouter();
  const { deviceId } = useLocalSearchParams<{ deviceId: string }>();
  const deviceQuery = useDevice(deviceId);
  const telemetry = useTelemetrySnapshot(deviceId ? [deviceId] : []);
  const snapshot = deviceId ? telemetry.byId[deviceId] : undefined;

  const sparklineLoad = useMemo(
    () => snapshot?.sparkline.loadW.map((p) => p.value) ?? [],
    [snapshot]
  );
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
          <YStack alignItems="flex-end" gap="$2">
            <Pill
              label={
                snapshot?.stale
                  ? getStatusGlyph('stale')
                  : telemetry.connectionStatus === 'connected'
                    ? getStatusGlyph('online')
                    : telemetry.connectionStatus === 'connecting' ||
                        telemetry.connectionStatus === 'reconnecting'
                      ? getStatusGlyph('processing')
                      : getStatusGlyph('waiting')
              }
              tone={snapshot?.stale ? 'warning' : telemetry.connectionStatus === 'connected' ? 'success' : 'neutral'}
              glyph
            />
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
        <SocBar value={snapshot?.metrics?.soc ?? deviceQuery.data?.batteryPct} />
        <XStack gap="$3" flexWrap="wrap">
          <Stat label="PV" value={formatW(snapshot?.metrics?.pvW)} />
          <Stat label="Load" value={formatW(snapshot?.metrics?.loadW)} />
          <Stat label="Battery" value={formatW(snapshot?.metrics?.batteryW)} />
          <Stat label="Temp" value={snapshot?.metrics ? `${snapshot.metrics.tempC.toFixed(1)}°C` : '—'} />
          <Stat label="State" value={deviceQuery.data?.state ?? '—'} />
          <Stat label="ETA" value={formatEtaMinutes(deviceQuery.data?.etaMinutes)} />
        </XStack>
      </Card>

      <Card gap="$2">
        <Text fontSize="$4" fontWeight="700">
          Load Trend (placeholder)
        </Text>
        <SparklinePlaceholder values={sparklineLoad} />
      </Card>

      <Card gap="$2">
        <Text fontSize="$4" fontWeight="700">
          Connection
        </Text>
        <Text opacity={0.8}>Engine: {telemetry.connectionStatus}</Text>
        <Text opacity={0.8}>Staleness: {snapshot?.stale ? 'STALE (>3s)' : 'fresh'}</Text>
        <Text opacity={0.8}>Serial: {deviceQuery.data?.serialNumber ?? '—'}</Text>
      </Card>
    </YStack>
  );
}
