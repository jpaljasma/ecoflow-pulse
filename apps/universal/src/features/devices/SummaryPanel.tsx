import { useEffect, useMemo, useRef, useState } from 'react';
import { Image, Platform, useWindowDimensions } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';
import { Card } from '@/shared/ui/Card';
import { PowerTrendChart } from '@/shared/ui/PowerTrendChart';
import { Stat } from '@/shared/ui/Stat';
import { formatW } from '@/features/telemetry/format';
import { getCapacityKWh } from '@/features/devices/capacity';
import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';
import { getEcoFlowAsset, getEcoFlowDefaultSize } from '@/shared/assets/ecoflowAssets';
import { useChartPrefs } from '@/shared/ui/chartPrefs';

const SUMMARY_TREND_POINTS = 60;
const SUMMARY_TREND_BUCKET_MS = 5_000;

function formatPct(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return '—';
  return `${value.toFixed(1)}%`;
}

function isNearZero(value: number | undefined | null): boolean {
  if (value === null || value === undefined || Number.isNaN(value)) return false;
  return value >= -0.5 && value <= 0.5;
}

function typeKey(model: string): string {
  const m = model.toLowerCase();
  if (m.includes('delta 2 max')) return 'delta_2_max';
  if (m.includes('delta pro ultra x')) return 'delta_pro_ultra_x';
  if (m.includes('delta pro ultra')) return 'delta_pro_ultra';
  if (m.includes('delta pro 3')) return 'delta_pro_3';
  if (m.includes('delta pro')) return 'delta_pro';
  if (m.includes('delta 3 ultra')) return 'delta_3_ultra';
  if (m.includes('delta 3 max')) return 'delta_3_max';
  if (m.includes('delta 3 plus')) return 'delta_3_plus';
  if (m.includes('delta 3')) return 'delta_3';
  return m;
}

export function SummaryPanel({
  devices,
  byId
}: {
  devices: DeviceSummary[];
  byId: Record<string, DeviceSnapshot>;
}) {
  const { width } = useWindowDimensions();
  const isDesktop = width >= 900;
  const trendChartStyle = useChartPrefs((s) => s.trendChartStyle);
  const toggleTrendChartStyle = useChartPrefs((s) => s.toggleTrendChartStyle);
  const [nowMs, setNowMs] = useState(() => Date.now());
  const [fleetTrend, setFleetTrend] = useState<{
    load: number[];
    pv: number[];
    ac: number[];
    dc: number[];
  }>(() => ({
    load: Array.from({ length: SUMMARY_TREND_POINTS }, () => 0),
    pv: Array.from({ length: SUMMARY_TREND_POINTS }, () => 0),
    ac: Array.from({ length: SUMMARY_TREND_POINTS }, () => 0),
    dc: Array.from({ length: SUMMARY_TREND_POINTS }, () => 0)
  }));
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

  const summary = useMemo(() => {
    if (!devices.length) {
      return {
        totalCapacityKWh: null,
        avgSocPct: null,
        acInW: undefined,
        dcW: undefined,
        pvW: undefined,
        loadW: undefined,
        netW: undefined
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
    }

    return {
      totalCapacityKWh: capacityCount ? totalCapacity : null,
      avgSocPct: socSum / devices.length,
      acInW,
      dcW,
      pvW,
      loadW,
      netW
    };
  }, [devices, byId]);

  useEffect(() => {
    const aggregated = devices.reduce(
      (acc, device) => {
        const snapshot = byId[device.id];
        acc.load += snapshot?.metrics?.loadW ?? device.loadW ?? 0;
        acc.pv += snapshot?.metrics?.pvW ?? device.pvW ?? 0;
        acc.ac += device.acInW ?? 0;
        acc.dc += device.dcW ?? 0;
        return acc;
      },
      { load: 0, pv: 0, ac: 0, dc: 0 }
    );
    const currentBucket = Math.floor(nowMs / SUMMARY_TREND_BUCKET_MS);
    const pending = pendingBucketRef.current;
    if (!pending) {
      pendingBucketRef.current = {
        bucket: currentBucket,
        loadSum: aggregated.load,
        pvSum: aggregated.pv,
        acSum: aggregated.ac,
        dcSum: aggregated.dc,
        count: 1
      };
      return;
    }

    if (pending.bucket === currentBucket) {
      pending.loadSum += aggregated.load;
      pending.pvSum += aggregated.pv;
      pending.acSum += aggregated.ac;
      pending.dcSum += aggregated.dc;
      pending.count += 1;
      return;
    }

    const bucketLoadAvg = pending.count > 0 ? pending.loadSum / pending.count : 0;
    const bucketPvAvg = pending.count > 0 ? pending.pvSum / pending.count : 0;
    const bucketAcAvg = pending.count > 0 ? pending.acSum / pending.count : 0;
    const bucketDcAvg = pending.count > 0 ? pending.dcSum / pending.count : 0;
    const bucketDelta = Math.max(1, currentBucket - pending.bucket);

    setFleetTrend((prev) => {
      const loadNext = [...prev.load];
      const pvNext = [...prev.pv];
      const acNext = [...prev.ac];
      const dcNext = [...prev.dc];
      for (let i = 0; i < bucketDelta; i += 1) {
        loadNext.push(bucketLoadAvg);
        pvNext.push(bucketPvAvg);
        acNext.push(bucketAcAvg);
        dcNext.push(bucketDcAvg);
      }
      return {
        load: loadNext.slice(-SUMMARY_TREND_POINTS),
        pv: pvNext.slice(-SUMMARY_TREND_POINTS),
        ac: acNext.slice(-SUMMARY_TREND_POINTS),
        dc: dcNext.slice(-SUMMARY_TREND_POINTS)
      };
    });

    pendingBucketRef.current = {
      bucket: currentBucket,
      loadSum: aggregated.load,
      pvSum: aggregated.pv,
      acSum: aggregated.ac,
      dcSum: aggregated.dc,
      count: 1
    };
  }, [nowMs, devices, byId]);

  const uniqueTypes = useMemo(() => {
    const seen = new Set<string>();
    const out: Array<{ key: string; label: string; uri?: string; emoji: string; active: boolean }> = [];
    const now = nowMs;

    for (const device of devices) {
      const batteryCount =
        device.details?.bpCount ??
        ((device.capabilities as { batteryPacks?: number } | undefined)?.batteryPacks ?? 1);
      const match = getDeviceAssetMatch(device.model, { batteryCount });
      const key = typeKey(device.model);
      if (seen.has(key)) continue;
      seen.add(key);
      const devicesOfType = devices.filter((d) => typeKey(d.model) === key);
      const hasActive = devicesOfType.some((d) => {
        const snap = byId[d.id];
        const lastSeenAtCandidates = [snap?.lastSeenAt ?? 0, d.telemetryTsMs ?? 0];
        const freshestLastSeenAt = Math.max(...lastSeenAtCandidates);
        const lastSeenAt = freshestLastSeenAt > 0 ? freshestLastSeenAt : null;
        if (lastSeenAt === null) return false;
        return now - lastSeenAt <= 60_000;
      });
      out.push({
        key,
        label: match.glyph.label,
        uri: match.slug ? getEcoFlowAsset(match.slug, getEcoFlowDefaultSize('list')) : undefined,
        emoji: match.glyph.emoji,
        active: hasActive
      });
    }
    return out;
  }, [devices, byId, nowMs]);

  return (
    <Card>
      <YStack gap="$3">
        <Text fontSize="$5" fontWeight="700">
          Fleet Summary
        </Text>
        <XStack gap="$3" alignItems="flex-start" flexWrap={isDesktop ? 'nowrap' : 'wrap'}>
          <XStack gap="$2" alignItems="center" flexShrink={0} flexWrap="wrap">
            {uniqueTypes.map((item) => (
              <YStack
                key={item.key}
                width={40}
                height={40}
                borderRadius="$2"
                overflow="hidden"
                backgroundColor="rgba(120,120,128,0.12)"
                alignItems="center"
                justifyContent="center"
                opacity={item.active ? 1 : 0.42}
              >
                {item.uri && Platform.OS === 'web' ? (
                  <Image
                    source={{ uri: item.uri }}
                    style={{ width: 34, height: 34 }}
                    resizeMode="cover"
                  />
                ) : (
                  <Text>{item.emoji}</Text>
                )}
              </YStack>
            ))}
          </XStack>
          <XStack gap="$3" flexWrap="wrap" flex={1} minWidth={0}>
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
                solar={fleetTrend.pv}
                ac={fleetTrend.ac}
                dc={fleetTrend.dc}
                load={fleetTrend.load}
                points={SUMMARY_TREND_POINTS}
                style={trendChartStyle}
              />
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
                solar={fleetTrend.pv}
                ac={fleetTrend.ac}
                dc={fleetTrend.dc}
                load={fleetTrend.load}
                points={SUMMARY_TREND_POINTS}
                style={trendChartStyle}
              />
            </YStack>
          </YStack>
        )}
      </YStack>
    </Card>
  );
}
