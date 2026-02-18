import { router } from 'expo-router';
import { Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';
import { Card } from '@/shared/ui/Card';
import { Pill } from '@/shared/ui/Pill';
import { Stat } from '@/shared/ui/Stat';
import { formatSoc, formatW } from '@/features/telemetry/format';

function statusTone(snapshot: DeviceSnapshot | undefined): 'neutral' | 'success' | 'warning' | 'danger' {
  if (!snapshot) return 'neutral';
  if (snapshot.stale) return 'warning';
  if (snapshot.status === 'charging') return 'success';
  if (snapshot.status === 'discharging') return 'danger';
  return 'neutral';
}

function statusLabel(snapshot: DeviceSnapshot | undefined): string {
  if (!snapshot) return 'Waiting';
  if (snapshot.stale) return 'STALE';
  return snapshot.status.toUpperCase();
}

export function DeviceCard({
  device,
  snapshot
}: {
  device: DeviceSummary;
  snapshot?: DeviceSnapshot;
}) {
  const metrics = snapshot?.metrics;
  const netW = metrics ? metrics.pvW - metrics.loadW : undefined;

  return (
    <Card
      pressStyle={{ scale: 0.995, opacity: 0.95 }}
      onPress={() => router.push(`/device/${device.id}`)}
      role="button"
      cursor="pointer"
      gap="$3"
    >
      <XStack justifyContent="space-between" alignItems="center">
        <YStack gap="$1" flex={1}>
          <Text fontSize="$5" fontWeight="700" numberOfLines={1}>
            {device.name}
          </Text>
          <Text fontSize="$2" opacity={0.72}>
            {device.model}
          </Text>
        </YStack>
        <Pill label={statusLabel(snapshot)} tone={statusTone(snapshot)} />
      </XStack>

      <XStack gap="$3" flexWrap="wrap">
        <Stat label="SOC" value={formatSoc(metrics?.soc)} />
        <Stat label="Net" value={formatW(netW)} />
        <Stat label="PV" value={formatW(metrics?.pvW)} />
        <Stat label="Load" value={formatW(metrics?.loadW)} />
      </XStack>
    </Card>
  );
}
