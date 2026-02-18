import { useMemo } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Image, Platform, ScrollView, useWindowDimensions } from 'react-native';
import { Button, Text, XStack, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { Card } from '@/shared/ui/Card';
import { PowerFlowGlyph } from '@/shared/ui/PowerFlowGlyph';
import { Pill } from '@/shared/ui/Pill';
import { Stat } from '@/shared/ui/Stat';
import { AppMenu } from '@/shared/ui/AppMenu';
import { SocBar } from '@/shared/ui/SocBar';
import { SparklineTrend } from '@/shared/ui/SparklineTrend';
import { useDevice } from '@/features/devices/hooks';
import { getCapacityKWh } from '@/features/devices/capacity';
import { useTelemetrySnapshot } from '@/features/telemetry/hooks';
import { formatAgo, formatEtaMinutes, formatW } from '@/features/telemetry/format';
import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';
import { getEcoFlowAsset, getEcoFlowDefaultSize } from '@/shared/assets/ecoflowAssets';
import { getStatusGlyph } from '@/shared/ui/statusGlyph';

const DETAIL_TREND_POINTS = 60;

function isNearZero(value: number | undefined | null): boolean {
  if (value === null || value === undefined || Number.isNaN(value)) return false;
  return value >= -0.5 && value <= 0.5;
}

function toneForState(state: string | undefined): 'neutral' | 'success' | 'warning' | 'danger' | 'info' {
  const value = (state ?? '').toLowerCase();
  if (value.includes('charging') || value.includes('online') || value.includes('on')) return 'success';
  if (value.includes('locked') || value.includes('idle')) return 'warning';
  if (value.includes('error') || value.includes('fault') || value.includes('off')) return 'danger';
  return 'neutral';
}

function formatSolarState(state: string | undefined): string {
  if (!state) return 'unknown';
  return state.replaceAll('_', ' ');
}

export default function DeviceDetailScreen() {
  const { width } = useWindowDimensions();
  const isTablet = width >= 768;
  const isDesktop = width >= 1200;
  const router = useRouter();
  const { deviceId } = useLocalSearchParams<{ deviceId: string }>();
  const deviceQuery = useDevice(deviceId);
  const telemetry = useTelemetrySnapshot(deviceId ? [deviceId] : []);
  const snapshot = deviceId ? telemetry.byId[deviceId] : undefined;

  const sparklineLoad = useMemo(
    () => snapshot?.sparkline.loadW.map((p) => p.value) ?? [],
    [snapshot]
  );
  const sparklinePV = useMemo(
    () => snapshot?.sparkline.pvW.map((p) => p.value) ?? [],
    [snapshot]
  );
  const acInW = deviceQuery.data?.acInW;
  const dcW = deviceQuery.data?.dcW;
  const tempC = snapshot?.metrics?.tempC;
  const isColdTemp = typeof tempC === 'number' && tempC <= 2;
  const netW =
    snapshot?.metrics
      ? snapshot.metrics.pvW - snapshot.metrics.loadW
      : deviceQuery.data?.netW;
  const capacityKWh = deviceQuery.data ? getCapacityKWh(deviceQuery.data) : null;
  const fallbackState =
    deviceQuery.data?.state === 'charging' ||
    deviceQuery.data?.state === 'discharging' ||
    deviceQuery.data?.state === 'idle'
      ? deviceQuery.data.state
      : 'idle';
  const detailState =
    snapshot && !snapshot.stale && snapshot.status !== 'stale'
      ? snapshot.status
      : fallbackState;
  const connectionGlyph =
    snapshot?.stale
      ? getStatusGlyph('stale')
      : telemetry.connectionStatus === 'connected'
        ? getStatusGlyph('online')
        : telemetry.connectionStatus === 'connecting' ||
            telemetry.connectionStatus === 'reconnecting'
          ? getStatusGlyph('processing')
          : getStatusGlyph('waiting');
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
  const modelLower = (deviceQuery.data?.model ?? '').toLowerCase();
  const details = deviceQuery.data?.details;
  const packRows = details?.packs ?? [];
  const solarRows = details?.solarPorts ?? [];
  const estimateLabel = details?.estimateMode
    ? `${details.estimateMode}${details.estimateSource ? ` · ${details.estimateSource}` : ''}`
    : details?.estimateSource ?? 'n/a';
  const signalRows: Array<{ label: string; on?: boolean }> = [
    { label: 'AC On', on: details?.acOn },
    { label: 'DC On', on: details?.dcOn },
    { label: 'USB On', on: details?.usbOn },
    { label: '12V On', on: details?.dc12vOn },
    { label: 'EV Charging', on: details?.evChargingOn },
    { label: 'Fan', on: details?.fanOn },
    { label: 'Solar Charging', on: details?.solarChargingOn }
  ];
  const isDelta2Max = modelLower.includes('delta 2 max');
  const isDeltaProUltra = modelLower.includes('delta pro ultra');
  const desktopScale = isDelta2Max ? 1.46 : isDeltaProUltra ? 1.5 : 1.48;
  const desktopOffsetY = isDelta2Max ? 8 : 0;
  const mobileImageSize = Math.min(width - 64, 360);
  const mediaColumnWidth = isDesktop ? 320 : 280;
  const mediaBoxHeight = isDesktop
    ? Math.round(mediaColumnWidth * 0.86)
    : isTablet
      ? Math.round(mediaColumnWidth * 0.92)
      : mobileImageSize;

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
          <YStack alignItems="flex-end">
            <AppMenu />
          </YStack>
        }
      />

      <YStack flex={1} minHeight={0}>
        {Platform.OS === 'web' ? (
          <div
            style={{
              flex: 1,
              minHeight: 0,
              overflowY: 'auto',
              overflowX: 'hidden',
              paddingBottom: 16
            }}
          >
            <YStack gap="$3">
              <Card gap="$3">
        <XStack
          gap="$4"
          alignItems={isTablet ? 'stretch' : 'flex-start'}
          flexDirection={isTablet ? 'row' : 'column'}
        >
          {deviceAsset ? (
            <YStack
              width={isTablet ? mediaColumnWidth : '100%'}
              height={mediaBoxHeight}
              flexShrink={0}
              borderRadius="$4"
              overflow="hidden"
              backgroundColor="rgba(120,120,128,0.12)"
              alignItems="center"
              justifyContent="center"
            >
              {Platform.OS === 'web' ? (
                <Image
                  source={{ uri: deviceAsset.uri }}
                  style={{
                    width: (isTablet ? mediaColumnWidth : mobileImageSize) * (isTablet ? desktopScale : 1.35),
                    height: mediaBoxHeight * (isTablet ? desktopScale : 1.35),
                    transform: isTablet ? [{ translateY: desktopOffsetY }] : undefined
                  }}
                  resizeMode="cover"
                />
              ) : (
                <Text fontSize="$9">{deviceAsset.emoji}</Text>
              )}
            </YStack>
          ) : null}

          <YStack gap="$3" flex={1} minWidth={0}>
            <XStack alignItems="flex-end" gap="$3">
              <XStack flex={1} minWidth={0}>
                <SocBar value={snapshot?.metrics?.soc ?? deviceQuery.data?.batteryPct} fullWidth />
              </XStack>
              <Text fontSize="$3" opacity={0.75} marginBottom="$1" flexShrink={0}>
                {capacityKWh !== null ? `🔋 ${capacityKWh.toFixed(1)}kWh` : '🔋 n/a'}
              </Text>
            </XStack>
            <XStack gap="$3" flexWrap="wrap" paddingRight="$2">
              <Stat label="∿ AC" value={formatW(acInW)} tone={isNearZero(acInW) ? 'muted' : 'default'} />
              <Stat label="⎓ DC" value={formatW(dcW)} tone={isNearZero(dcW) ? 'muted' : 'default'} />
              <Stat label="☼ PV" value={formatW(snapshot?.metrics?.pvW)} tone={isNearZero(snapshot?.metrics?.pvW) ? 'muted' : 'default'} />
              <Stat label="⌂ Load" value={formatW(snapshot?.metrics?.loadW)} tone={isNearZero(snapshot?.metrics?.loadW) ? 'muted' : 'default'} />
              <Stat label="⚖ Net" value={formatW(netW)} />
              <Stat label="🔋 Battery" value={formatW(snapshot?.metrics?.batteryW)} />
              <Stat
                label={isColdTemp ? '❄ Temp' : '🌡 Temp'}
                value={snapshot?.metrics ? `${snapshot.metrics.tempC.toFixed(1)}°C` : '—'}
                tone={isColdTemp ? 'cold' : 'default'}
              />
              <Stat
                label="◉ State"
                value={deviceQuery.data ? detailState : '—'}
              />
              <Stat label="⏱ ETA" value={formatEtaMinutes(deviceQuery.data?.etaMinutes)} />
            </XStack>
            <XStack justifyContent="flex-end" alignItems="center" gap="$2">
              {deviceQuery.data ? (
                <PowerFlowGlyph
                  status={detailState}
                  pvW={snapshot?.metrics?.pvW ?? deviceQuery.data?.pvW}
                  loadW={snapshot?.metrics?.loadW ?? deviceQuery.data?.loadW}
                  fontSize="$6"
                  lineHeight={24}
                />
              ) : null}
              <Text fontSize="$3" opacity={0.9}>
                {connectionGlyph}
              </Text>
            </XStack>
          </YStack>
        </XStack>
      </Card>

      <XStack gap="$3" flexWrap="wrap">
        <Card gap="$3" flex={1} minWidth={isDesktop ? 320 : 280}>
          <XStack justifyContent="space-between" alignItems="center">
            <Text fontSize="$4" fontWeight="700">
              🔋 Battery Packs
            </Text>
            <Pill
              label={`${details?.bpCount ?? packRows.length ?? 0} packs`}
              tone="info"
            />
          </XStack>
          {packRows.length ? (
            <YStack gap="$2">
              {packRows.map((pack) => (
                <YStack
                  key={pack.id}
                  gap="$1"
                  padding="$2"
                  borderRadius="$3"
                  borderWidth={1}
                  borderColor="rgba(120,120,128,0.24)"
                >
                  <XStack alignItems="center" justifyContent="space-between">
                    <Text fontWeight="700">{pack.id.toUpperCase()}</Text>
                    <Text opacity={0.8}>
                      {Number.isFinite(pack.powerW as number) ? formatW(pack.powerW) : '—'}
                      {' · '}
                      {Number.isFinite(pack.tempC as number) ? `${pack.tempC?.toFixed(1)}°C` : '—'}
                    </Text>
                  </XStack>
                  <SocBar value={pack.socPct} fullWidth />
                </YStack>
              ))}
            </YStack>
          ) : (
            <Text opacity={0.7}>No per-pack telemetry yet.</Text>
          )}
        </Card>

        <Card gap="$3" flex={1} minWidth={isDesktop ? 320 : 280}>
          <Text fontSize="$4" fontWeight="700">
            ☀ Solar Inputs
          </Text>
          <YStack gap="$2">
            {solarRows.map((port) => (
              <YStack
                key={port.id}
                gap="$2"
                padding="$2"
                borderRadius="$3"
                borderWidth={1}
                borderColor="rgba(120,120,128,0.24)"
              >
                <XStack justifyContent="space-between" alignItems="center">
                  <Text fontWeight="700">{port.name}</Text>
                  <Pill
                    label={formatSolarState(port.state)}
                    tone={toneForState(port.state)}
                  />
                </XStack>
                <XStack gap="$3" flexWrap="wrap">
                  <Stat label="⚡ W" value={formatW(port.watts)} tone={isNearZero(port.watts) ? 'muted' : 'default'} />
                  <Stat label="V" value={Number.isFinite(port.volts as number) ? `${port.volts?.toFixed(1)}V` : '—'} />
                  <Stat label="A" value={Number.isFinite(port.amps as number) ? `${port.amps?.toFixed(2)}A` : '—'} />
                  <Stat
                    label="Cap"
                    value={
                      port.maxWatts
                        ? `${port.maxWatts}W · ${port.maxVolts ?? '—'}V · ${port.maxAmps ?? '—'}A`
                        : '—'
                    }
                  />
                </XStack>
              </YStack>
            ))}
          </YStack>
        </Card>
      </XStack>

      <XStack gap="$3" flexWrap="wrap">
        <Card gap="$3" flex={1} minWidth={isDesktop ? 360 : 280}>
          <Text fontSize="$4" fontWeight="700">
            🧭 Estimate & Queue
          </Text>
          <XStack gap="$2" flexWrap="wrap">
            <Pill label={`mode: ${estimateLabel}`} tone="neutral" />
            <Pill
              label={`eta: ${formatEtaMinutes(details?.estimateEtaMin ?? deviceQuery.data?.etaMinutes)}`}
              tone="info"
            />
            <Pill
              label={`queue: ${details?.mqttQueueDepth ?? 0}`}
              tone={(details?.mqttQueueDepth ?? 0) > 48 ? 'warning' : 'success'}
            />
            <Pill
              label={`dropped: ${details?.mqttQueueDroppedOldest ?? 0}`}
              tone={(details?.mqttQueueDroppedOldest ?? 0) > 0 ? 'danger' : 'neutral'}
            />
          </XStack>
        </Card>
        <Card gap="$3" flex={1} minWidth={isDesktop ? 360 : 280}>
          <Text fontSize="$4" fontWeight="700">
            ✅ System Signals
          </Text>
          <XStack gap="$2" flexWrap="wrap">
            {signalRows.map((signal) => (
              <Pill
                key={signal.label}
                label={`${signal.on ? '●' : '○'} ${signal.label}`}
                tone={signal.on === true ? 'success' : signal.on === false ? 'neutral' : 'warning'}
              />
            ))}
          </XStack>
        </Card>
      </XStack>

      {isDesktop ? (
        <XStack gap="$3" alignItems="stretch">
          <Card gap="$2" flex={2}>
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
              <SparklineTrend values={sparklineLoad} points={DETAIL_TREND_POINTS} />
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
              <SparklineTrend values={sparklinePV} points={DETAIL_TREND_POINTS} />
            </YStack>
          </Card>
          <Card gap="$2" flex={1}>
            <Text fontSize="$4" fontWeight="700">
              Connection
            </Text>
            <Text opacity={0.8}>Engine: {telemetry.connectionStatus}</Text>
            <Text opacity={0.8}>Staleness: {snapshot?.stale ? 'STALE (>5s)' : 'fresh'}</Text>
            <Text opacity={0.8}>Serial: {deviceQuery.data?.serialNumber ?? '—'}</Text>
          </Card>
        </XStack>
      ) : (
        <>
          <Card gap="$2">
            {isTablet ? (
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
                  <SparklineTrend values={sparklineLoad} points={DETAIL_TREND_POINTS} />
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
                  <SparklineTrend values={sparklinePV} points={DETAIL_TREND_POINTS} />
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
                  <SparklineTrend values={sparklineLoad} points={DETAIL_TREND_POINTS} />
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
                  <SparklineTrend values={sparklinePV} points={DETAIL_TREND_POINTS} />
                </YStack>
              </YStack>
            )}
          </Card>

          <Card gap="$2">
            <Text fontSize="$4" fontWeight="700">
              Connection
            </Text>
            <Text opacity={0.8}>Engine: {telemetry.connectionStatus}</Text>
            <Text opacity={0.8}>Staleness: {snapshot?.stale ? 'STALE (>5s)' : 'fresh'}</Text>
            <Text opacity={0.8}>Serial: {deviceQuery.data?.serialNumber ?? '—'}</Text>
          </Card>
        </>
      )}
            </YStack>
          </div>
        ) : (
          <ScrollView
            style={{ flex: 1 }}
            contentContainerStyle={{ paddingBottom: 16 }}
            showsVerticalScrollIndicator
          >
            <YStack gap="$3">
              <Card gap="$3">
                <XStack
                  gap="$4"
                  alignItems={isTablet ? 'stretch' : 'flex-start'}
                  flexDirection={isTablet ? 'row' : 'column'}
                >
                  {deviceAsset ? (
                    <YStack
                      width={isTablet ? mediaColumnWidth : '100%'}
                      height={mediaBoxHeight}
                      flexShrink={0}
                      borderRadius="$4"
                      overflow="hidden"
                      backgroundColor="rgba(120,120,128,0.12)"
                      alignItems="center"
                      justifyContent="center"
                    >
                      <Image
                        source={{ uri: deviceAsset.uri }}
                        style={{
                          width: (isTablet ? mediaColumnWidth : mobileImageSize) * (isTablet ? desktopScale : 1.35),
                          height: mediaBoxHeight * (isTablet ? desktopScale : 1.35),
                          transform: isTablet ? [{ translateY: desktopOffsetY }] : undefined
                        }}
                        resizeMode="cover"
                      />
                    </YStack>
                  ) : null}

                  <YStack gap="$3" flex={1} minWidth={0}>
                    <XStack alignItems="flex-end" gap="$3">
                      <XStack flex={1} minWidth={0}>
                        <SocBar value={snapshot?.metrics?.soc ?? deviceQuery.data?.batteryPct} fullWidth />
                      </XStack>
                      <Text fontSize="$3" opacity={0.75} marginBottom="$1" flexShrink={0}>
                        {capacityKWh !== null ? `🔋 ${capacityKWh.toFixed(1)}kWh` : '🔋 n/a'}
                      </Text>
                    </XStack>
                    <XStack gap="$3" flexWrap="wrap" paddingRight="$2">
                      <Stat label="∿ AC" value={formatW(acInW)} tone={isNearZero(acInW) ? 'muted' : 'default'} />
                      <Stat label="⎓ DC" value={formatW(dcW)} tone={isNearZero(dcW) ? 'muted' : 'default'} />
                      <Stat label="☼ PV" value={formatW(snapshot?.metrics?.pvW)} tone={isNearZero(snapshot?.metrics?.pvW) ? 'muted' : 'default'} />
                      <Stat label="⌂ Load" value={formatW(snapshot?.metrics?.loadW)} tone={isNearZero(snapshot?.metrics?.loadW) ? 'muted' : 'default'} />
                      <Stat label="⚖ Net" value={formatW(netW)} />
                      <Stat label="🔋 Battery" value={formatW(snapshot?.metrics?.batteryW)} />
                      <Stat
                        label={isColdTemp ? '❄ Temp' : '🌡 Temp'}
                        value={snapshot?.metrics ? `${snapshot.metrics.tempC.toFixed(1)}°C` : '—'}
                        tone={isColdTemp ? 'cold' : 'default'}
                      />
                      <Stat
                        label="◉ State"
                        value={deviceQuery.data ? detailState : '—'}
                      />
                      <Stat label="⏱ ETA" value={formatEtaMinutes(deviceQuery.data?.etaMinutes)} />
                    </XStack>
                    <XStack justifyContent="flex-end" alignItems="center" gap="$2">
                      {deviceQuery.data ? (
                        <PowerFlowGlyph
                          status={detailState}
                          pvW={snapshot?.metrics?.pvW ?? deviceQuery.data?.pvW}
                          loadW={snapshot?.metrics?.loadW ?? deviceQuery.data?.loadW}
                          fontSize="$6"
                          lineHeight={24}
                        />
                      ) : null}
                      <Text fontSize="$3" opacity={0.9}>
                        {connectionGlyph}
                      </Text>
                    </XStack>
                  </YStack>
                </XStack>
              </Card>

              <XStack gap="$3" flexWrap="wrap">
                <Card gap="$3" flex={1} minWidth={isDesktop ? 320 : 280}>
                  <XStack justifyContent="space-between" alignItems="center">
                    <Text fontSize="$4" fontWeight="700">
                      🔋 Battery Packs
                    </Text>
                    <Pill
                      label={`${details?.bpCount ?? packRows.length ?? 0} packs`}
                      tone="info"
                    />
                  </XStack>
                  {packRows.length ? (
                    <YStack gap="$2">
                      {packRows.map((pack) => (
                        <YStack
                          key={pack.id}
                          gap="$1"
                          padding="$2"
                          borderRadius="$3"
                          borderWidth={1}
                          borderColor="rgba(120,120,128,0.24)"
                        >
                          <XStack alignItems="center" justifyContent="space-between">
                            <Text fontWeight="700">{pack.id.toUpperCase()}</Text>
                            <Text opacity={0.8}>
                              {Number.isFinite(pack.powerW as number) ? formatW(pack.powerW) : '—'}
                              {' · '}
                              {Number.isFinite(pack.tempC as number) ? `${pack.tempC?.toFixed(1)}°C` : '—'}
                            </Text>
                          </XStack>
                          <SocBar value={pack.socPct} fullWidth />
                        </YStack>
                      ))}
                    </YStack>
                  ) : (
                    <Text opacity={0.7}>No per-pack telemetry yet.</Text>
                  )}
                </Card>

                <Card gap="$3" flex={1} minWidth={isDesktop ? 320 : 280}>
                  <Text fontSize="$4" fontWeight="700">
                    ☀ Solar Inputs
                  </Text>
                  <YStack gap="$2">
                    {solarRows.map((port) => (
                      <YStack
                        key={port.id}
                        gap="$2"
                        padding="$2"
                        borderRadius="$3"
                        borderWidth={1}
                        borderColor="rgba(120,120,128,0.24)"
                      >
                        <XStack justifyContent="space-between" alignItems="center">
                          <Text fontWeight="700">{port.name}</Text>
                          <Pill
                            label={formatSolarState(port.state)}
                            tone={toneForState(port.state)}
                          />
                        </XStack>
                        <XStack gap="$3" flexWrap="wrap">
                          <Stat label="⚡ W" value={formatW(port.watts)} tone={isNearZero(port.watts) ? 'muted' : 'default'} />
                          <Stat label="V" value={Number.isFinite(port.volts as number) ? `${port.volts?.toFixed(1)}V` : '—'} />
                          <Stat label="A" value={Number.isFinite(port.amps as number) ? `${port.amps?.toFixed(2)}A` : '—'} />
                          <Stat
                            label="Cap"
                            value={
                              port.maxWatts
                                ? `${port.maxWatts}W · ${port.maxVolts ?? '—'}V · ${port.maxAmps ?? '—'}A`
                                : '—'
                            }
                          />
                        </XStack>
                      </YStack>
                    ))}
                  </YStack>
                </Card>
              </XStack>

              <XStack gap="$3" flexWrap="wrap">
                <Card gap="$3" flex={1} minWidth={isDesktop ? 360 : 280}>
                  <Text fontSize="$4" fontWeight="700">
                    🧭 Estimate & Queue
                  </Text>
                  <XStack gap="$2" flexWrap="wrap">
                    <Pill label={`mode: ${estimateLabel}`} tone="neutral" />
                    <Pill
                      label={`eta: ${formatEtaMinutes(details?.estimateEtaMin ?? deviceQuery.data?.etaMinutes)}`}
                      tone="info"
                    />
                    <Pill
                      label={`queue: ${details?.mqttQueueDepth ?? 0}`}
                      tone={(details?.mqttQueueDepth ?? 0) > 48 ? 'warning' : 'success'}
                    />
                    <Pill
                      label={`dropped: ${details?.mqttQueueDroppedOldest ?? 0}`}
                      tone={(details?.mqttQueueDroppedOldest ?? 0) > 0 ? 'danger' : 'neutral'}
                    />
                  </XStack>
                </Card>
                <Card gap="$3" flex={1} minWidth={isDesktop ? 360 : 280}>
                  <Text fontSize="$4" fontWeight="700">
                    ✅ System Signals
                  </Text>
                  <XStack gap="$2" flexWrap="wrap">
                    {signalRows.map((signal) => (
                      <Pill
                        key={signal.label}
                        label={`${signal.on ? '●' : '○'} ${signal.label}`}
                        tone={signal.on === true ? 'success' : signal.on === false ? 'neutral' : 'warning'}
                      />
                    ))}
                  </XStack>
                </Card>
              </XStack>

              {isDesktop ? (
                <XStack gap="$3" alignItems="stretch">
                  <Card gap="$2" flex={2}>
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
                      <SparklineTrend values={sparklineLoad} points={DETAIL_TREND_POINTS} />
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
                      <SparklineTrend values={sparklinePV} points={DETAIL_TREND_POINTS} />
                    </YStack>
                  </Card>
                  <Card gap="$2" flex={1}>
                    <Text fontSize="$4" fontWeight="700">
                      Connection
                    </Text>
                    <Text opacity={0.8}>Engine: {telemetry.connectionStatus}</Text>
                    <Text opacity={0.8}>Staleness: {snapshot?.stale ? 'STALE (>5s)' : 'fresh'}</Text>
                    <Text opacity={0.8}>Serial: {deviceQuery.data?.serialNumber ?? '—'}</Text>
                  </Card>
                </XStack>
              ) : (
                <>
                  <Card gap="$2">
                    {isTablet ? (
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
                          <SparklineTrend values={sparklineLoad} points={DETAIL_TREND_POINTS} />
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
                          <SparklineTrend values={sparklinePV} points={DETAIL_TREND_POINTS} />
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
                          <SparklineTrend values={sparklineLoad} points={DETAIL_TREND_POINTS} />
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
                          <SparklineTrend values={sparklinePV} points={DETAIL_TREND_POINTS} />
                        </YStack>
                      </YStack>
                    )}
                  </Card>

                  <Card gap="$2">
                    <Text fontSize="$4" fontWeight="700">
                      Connection
                    </Text>
                    <Text opacity={0.8}>Engine: {telemetry.connectionStatus}</Text>
                    <Text opacity={0.8}>Staleness: {snapshot?.stale ? 'STALE (>5s)' : 'fresh'}</Text>
                    <Text opacity={0.8}>Serial: {deviceQuery.data?.serialNumber ?? '—'}</Text>
                  </Card>
                </>
              )}
            </YStack>
          </ScrollView>
        )}
      </YStack>
    </YStack>
  );
}
