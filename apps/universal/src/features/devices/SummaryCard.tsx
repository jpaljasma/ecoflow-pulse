import { useMemo } from 'react';
import { useWindowDimensions } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';
import { Card } from '@/shared/ui/Card';
import { SparklineTrend, normalizeTrend } from '@/shared/ui/SparklineTrend';
import { Stat } from '@/shared/ui/Stat';
import { formatW } from '@/features/telemetry/format';
import { getCapacityKWh } from '@/features/devices/capacity';

const SUMMARY_TREND_POINTS = 60;

function formatPct(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return '—';
  return `${value.toFixed(1)}%`;
}

function isNearZero(value: number | undefined | null): boolean {
  if (value === null || value === undefined || Number.isNaN(value)) return false;
  return value >= -0.5 && value <= 0.5;
}

export function SummaryCard({
  devices,
  byId
}: {
  devices: DeviceSummary[];
  byId: Record<string, DeviceSnapshot>;
}) {
  const { width } = useWindowDimensions();
  const isDesktop = width >= 900;

  const summary = useMemo(() => {
    if (!devices.length) {
      return {
        totalCapacityKWh: null,
        avgSocPct: null,
        acInW: undefined,
        dcW: undefined,
        pvW: undefined,
        loadW: undefined,
        netW: undefined,
        pvTrend: [] as number[],
        loadTrend: [] as number[]
      };
    }

    let totalCapacity = 0;
    let capacityCount = 0;
    let socSum = 0;
    let acInW = 0;
    let dcW = 0;
    let pvW = 0;
    let loadW = 0;
    let netW = 0;
    const pvTrend: number[] = [];
    const loadTrend: number[] = [];

    for (const device of devices) {
      const cap = getCapacityKWh(device);
      if (cap !== null) {
        totalCapacity += cap;
        capacityCount += 1;
      }

      const snapshot = byId[device.id];
      const soc = snapshot?.metrics?.soc ?? device.batteryPct;
      socSum += soc;

      acInW += device.acInW ?? 0;
      dcW += device.dcW ?? 0;
      pvW += snapshot?.metrics?.pvW ?? device.pvW ?? 0;
      loadW += snapshot?.metrics?.loadW ?? device.loadW ?? 0;
      netW += snapshot?.metrics ? snapshot.metrics.pvW - snapshot.metrics.loadW : device.netW ?? 0;

      const snapshotPV = snapshot?.sparkline?.pvW ?? [];
      for (let i = 0; i < snapshotPV.length; i += 1) {
        const point = snapshotPV[i];
        if (!point) continue;
        pvTrend[i] = (pvTrend[i] ?? 0) + point.value;
      }
      const snapshotLoad = snapshot?.sparkline?.loadW ?? [];
      for (let i = 0; i < snapshotLoad.length; i += 1) {
        const point = snapshotLoad[i];
        if (!point) continue;
        loadTrend[i] = (loadTrend[i] ?? 0) + point.value;
      }
    }

    return {
      totalCapacityKWh: capacityCount ? totalCapacity : null,
      avgSocPct: socSum / devices.length,
      acInW,
      dcW,
      pvW,
      loadW,
      netW,
      pvTrend: normalizeTrend(pvTrend, SUMMARY_TREND_POINTS),
      loadTrend: normalizeTrend(loadTrend, SUMMARY_TREND_POINTS)
    };
  }, [devices, byId]);

  return (
    <Card>
      <YStack gap="$3">
        <Text fontSize="$5" fontWeight="700">
          Fleet Summary
        </Text>
        <XStack gap="$3" flexWrap="wrap">
          <Stat
            label="🔋 Battery"
            value={
              summary.totalCapacityKWh !== null
                ? `${summary.totalCapacityKWh.toFixed(1)}kWh`
                : '—'
            }
          />
          <Stat label="⏲️ SOC" value={formatPct(summary.avgSocPct)} />
          <Stat label="∿ AC" value={formatW(summary.acInW)} tone={isNearZero(summary.acInW) ? 'muted' : 'default'} />
          <Stat label="⎓ DC" value={formatW(summary.dcW)} tone={isNearZero(summary.dcW) ? 'muted' : 'default'} />
          <Stat label="☼ PV" value={formatW(summary.pvW)} tone={isNearZero(summary.pvW) ? 'muted' : 'default'} />
          <Stat label="⌂ Load" value={formatW(summary.loadW)} tone={isNearZero(summary.loadW) ? 'muted' : 'default'} />
          <Stat label="⚖️ Net" value={formatW(summary.netW)} />
        </XStack>
        <YStack height={1} backgroundColor="rgba(120,120,128,0.24)" />
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
              <SparklineTrend values={summary.loadTrend} points={SUMMARY_TREND_POINTS} />
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
              <SparklineTrend values={summary.pvTrend} points={SUMMARY_TREND_POINTS} />
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
              <SparklineTrend values={summary.loadTrend} points={SUMMARY_TREND_POINTS} />
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
              <SparklineTrend values={summary.pvTrend} points={SUMMARY_TREND_POINTS} />
            </YStack>
          </YStack>
        )}
      </YStack>
    </Card>
  );
}
