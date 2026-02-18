import { useMemo } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
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
              label={snapshot?.stale ? 'STALE' : telemetry.connectionStatus.toUpperCase()}
              tone={snapshot?.stale ? 'warning' : telemetry.connectionStatus === 'connected' ? 'success' : 'neutral'}
            />
            <AppMenu />
          </YStack>
        }
      />

      <Card gap="$3">
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
