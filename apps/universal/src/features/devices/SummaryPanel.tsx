import { useEffect, useMemo, useRef, useState } from 'react';
import { useWindowDimensions } from 'react-native';
import { Image as ExpoImage } from 'expo-image';
import { Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';
import { Card } from '@/shared/ui/Card';
import { PowerTrendChart } from '@/shared/ui/PowerTrendChart';
import { SolarGeneratedChart } from '@/shared/ui/SolarGeneratedChart';
import { Stat } from '@/shared/ui/Stat';
import { formatKWh, formatW } from '@/features/telemetry/format';
import { SolarTodayBadge } from '@/shared/ui/SolarTodayBadge';
import { getCapacityKWh } from '@/features/devices/capacity';
import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';
import { getEcoFlowAsset, getEcoFlowDefaultSize } from '@/shared/assets/ecoflowAssets';
import { useChartPrefs } from '@/shared/ui/chartPrefs';
import { CachedImage } from '@/shared/ui/CachedImage';
import { getBundledDeviceFallback } from '@/shared/assets/deviceFallbacks';
import { env } from '@/shared/config/env';

const SUMMARY_TREND_POINTS = 60;
const SUMMARY_TREND_BUCKET_MS = 5_000;
const SOLAR_GENERATED_POINTS = 72;

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
  const isTabletUp = width >= 768;
  const isCompact = width < 720;
  const metricCellMinWidth = isCompact ? 0 : 96;
  const useRemoteImage = Boolean(env.assetBaseUrl);
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
  const [solarGeneratedTrend, setSolarGeneratedTrend] = useState<number[]>(
    Array.from({ length: SOLAR_GENERATED_POINTS }, () => 0)
  );
  const solarGeneratedMinuteRef = useRef<number | null>(null);
  const solarGeneratedInitializedRef = useRef(false);
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

  useEffect(() => {
    const minuteBucket = Math.floor(nowMs / 60_000);
    if (
      solarGeneratedInitializedRef.current &&
      solarGeneratedMinuteRef.current === minuteBucket
    ) {
      return;
    }
    const byIndex = Array.from({ length: SOLAR_GENERATED_POINTS }, () => 0);
    for (const device of devices) {
      const series = device.solarGeneratedSeriesWh ?? [];
      const padded =
        series.length >= SOLAR_GENERATED_POINTS
          ? series.slice(-SOLAR_GENERATED_POINTS)
          : [
              ...Array.from({ length: SOLAR_GENERATED_POINTS - series.length }, () => 0),
              ...series
            ];
      for (let i = 0; i < SOLAR_GENERATED_POINTS; i += 1) {
        byIndex[i] = (byIndex[i] ?? 0) + Math.max(0, padded[i] ?? 0);
      }
    }
    setSolarGeneratedTrend(byIndex);
    solarGeneratedInitializedRef.current = true;
    solarGeneratedMinuteRef.current = minuteBucket;
  }, [devices, nowMs]);

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
        solarTodayWh: undefined
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
    let solarTodayWh = 0;

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
      solarTodayWh += Math.max(0, device.solarTodayWh ?? 0);
    }

    return {
      totalCapacityKWh: capacityCount ? totalCapacity : null,
      avgSocPct: socSum / devices.length,
      acInW,
      dcW,
      pvW,
      loadW,
      netW,
      solarTodayWh
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
    const out: Array<{
      key: string;
      label: string;
      uri?: string;
      fallback?: ReturnType<typeof getBundledDeviceFallback>;
      emoji: string;
      active: boolean;
    }> = [];
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
        uri:
          useRemoteImage && match.slug
            ? getEcoFlowAsset(match.slug, getEcoFlowDefaultSize('list'))
            : undefined,
        fallback: match.slug ? getBundledDeviceFallback(match.slug, '256') : undefined,
        emoji: match.glyph.emoji,
        active: hasActive
      });
    }
    return out;
  }, [devices, byId, nowMs, useRemoteImage]);

  return (
    <Card>
      <YStack gap="$3">
        <Text fontSize="$5" fontWeight="700">
          Fleet Summary
        </Text>
        <XStack gap="$2" alignItems="center" flexWrap="wrap">
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
              {item.uri ? (
                <CachedImage
                  uri={item.uri}
                  style={{ width: 34, height: 34 }}
                  contentFit="cover"
                />
              ) : item.fallback ? (
                <ExpoImage
                  source={item.fallback}
                  style={{ width: 34, height: 34 }}
                  contentFit="cover"
                />
              ) : (
                <Text>{item.emoji}</Text>
              )}
            </YStack>
          ))}
        </XStack>
        {isCompact ? (
          <XStack flexWrap="wrap" marginHorizontal={-4} alignItems="flex-start">
            <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
              <Stat
                label="🔋 Battery"
                value={
                  summary.totalCapacityKWh !== null
                    ? formatKWh(summary.totalCapacityKWh)
                    : '—'
                }
                compact
              />
            </YStack>
            <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
              <Stat label="⏲️ SOC" value={formatPct(summary.avgSocPct)} compact />
            </YStack>
            <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
              <Stat label="⚖️ Net" value={formatW(summary.netW)} compact />
            </YStack>
            <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
              <Stat
                label="∿ AC"
                value={formatW(summary.acInW)}
                tone={isNearZero(summary.acInW) ? 'muted' : 'default'}
                compact
              />
            </YStack>
            <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
              <Stat
                label="⎓ DC"
                value={formatW(summary.dcW)}
                tone={isNearZero(summary.dcW) ? 'muted' : 'default'}
                compact
              />
            </YStack>
            <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
              <Stat
                label="☼ PV"
                value={formatW(summary.pvW)}
                tone={isNearZero(summary.pvW) ? 'muted' : 'default'}
                compact
              />
            </YStack>
            <YStack width="100%" paddingHorizontal={4} paddingVertical={2}>
              <SolarTodayBadge valueWh={summary.solarTodayWh} compact />
            </YStack>
            <YStack width="33.333%" paddingHorizontal={4} paddingVertical={2}>
              <Stat
                label="⌂ Load"
                value={formatW(summary.loadW)}
                tone={isNearZero(summary.loadW) ? 'muted' : 'default'}
                compact
              />
            </YStack>
          </XStack>
        ) : (
          <XStack gap="$2" flexWrap="nowrap" alignItems="flex-start">
            <YStack minWidth={metricCellMinWidth} flex={1}>
              <Stat
                label="🔋 Battery"
                value={
                  summary.totalCapacityKWh !== null
                    ? formatKWh(summary.totalCapacityKWh)
                    : '—'
                }
              />
            </YStack>
            <YStack minWidth={metricCellMinWidth} flex={1}>
              <Stat label="⏲️ SOC" value={formatPct(summary.avgSocPct)} />
            </YStack>
            <YStack minWidth={metricCellMinWidth} flex={1}>
              <Stat label="⚖️ Net" value={formatW(summary.netW)} />
            </YStack>
            <YStack minWidth={metricCellMinWidth} flex={1}>
              <Stat
                label="∿ AC"
                value={formatW(summary.acInW)}
                tone={isNearZero(summary.acInW) ? 'muted' : 'default'}
              />
            </YStack>
            <YStack minWidth={metricCellMinWidth} flex={1}>
              <Stat
                label="⎓ DC"
                value={formatW(summary.dcW)}
                tone={isNearZero(summary.dcW) ? 'muted' : 'default'}
              />
            </YStack>
            <YStack minWidth={metricCellMinWidth} flex={1}>
              <Stat
                label="☼ PV"
                value={formatW(summary.pvW)}
                tone={isNearZero(summary.pvW) ? 'muted' : 'default'}
              />
            </YStack>
            <YStack minWidth={Math.max(180, metricCellMinWidth * 2)} flex={1.4}>
              <SolarTodayBadge valueWh={summary.solarTodayWh} />
            </YStack>
            <YStack minWidth={metricCellMinWidth} flex={1}>
              <Stat
                label="⌂ Load"
                value={formatW(summary.loadW)}
                tone={isNearZero(summary.loadW) ? 'muted' : 'default'}
              />
            </YStack>
          </XStack>
        )}
        <YStack height={1} backgroundColor="rgba(120,120,128,0.24)" />
        {isTabletUp ? (
          <XStack gap="$3" alignItems="stretch" flexWrap="nowrap">
            <YStack
              flexBasis="50%"
              minWidth="50%"
              maxWidth="50%"
              gap="$2"
              padding="$3"
              borderRadius="$3"
              borderWidth={1}
              borderColor="rgba(120,120,128,0.24)"
            >
              <XStack alignItems="center" justifyContent="space-between">
                <Text fontSize="$4" fontWeight="700">
                  ☼ Solar Generated (6am-6pm, 10m buckets)
                </Text>
                <Text fontSize="$2" opacity={0.72}>
                  1m refresh
                </Text>
              </XStack>
              <SolarGeneratedChart valuesWh={solarGeneratedTrend} points={SOLAR_GENERATED_POINTS} />
            </YStack>
            <YStack
              flexBasis="50%"
              minWidth="50%"
              maxWidth="50%"
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
                  ☼ Solar Generated (6am-6pm, 10m buckets)
                </Text>
                <Text fontSize="$2" opacity={0.72}>
                  1m refresh
                </Text>
              </XStack>
              <SolarGeneratedChart valuesWh={solarGeneratedTrend} points={SOLAR_GENERATED_POINTS} />
            </YStack>
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
