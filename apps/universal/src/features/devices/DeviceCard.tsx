import { router } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Animated, Platform } from 'react-native';
import { useEffect, useMemo, useRef } from 'react';
import type { ComponentProps } from 'react';
import { Button, Text, XStack, YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import { buildStormGuardLabel } from '@/features/devices/stormGuard';
import type { DeviceSnapshot, TelemetryEngineStatus } from '@/features/telemetry/engine/types';
import { resolveNetPowerW } from '@/features/devices/net';
import { Card } from '@/shared/ui/Card';
import { DeviceHeroPanel } from '@/shared/ui/DeviceHeroPanel';
import { PowerFlowGlyph } from '@/shared/ui/PowerFlowGlyph';
import { Stat } from '@/shared/ui/Stat';
import { formatAgo, formatEtaMinutes, formatKWh, formatW, maskSerialNumber } from '@/features/telemetry/format';
import { getCapacityKWh } from '@/features/devices/capacity';
import { SocBar } from '@/shared/ui/SocBar';
import { getStatusIconName } from '@/shared/ui/statusGlyph';
import { env } from '@/shared/config/env';
import { SolarTodayBadge } from '@/shared/ui/SolarTodayBadge';
import { MetricsGrid, type MetricsGridItem } from '@/shared/ui/MetricsGrid';
import { buildInventoryMetricLayout, buildStatusDotHoverLabel } from '@/features/devices/deviceCardLayout';
import { isMutedMetric } from '@/shared/ui/uiMappings';
import { StormGuardChip } from '@/shared/ui/StormGuardChip';
import { useTelemetryDeviceSnapshot } from '@/features/telemetry/hooks';
import { useAuthSession } from '@/features/auth/hooks';
import { useDeviceSolarHistory } from '@/features/history/hooks';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { IconLabel } from '@/shared/ui/IconLabel';
import { buildPulseActionButtonStyles } from '@/shared/ui/buttonInteractions';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';
import { resolveDeviceVisualAssets } from '@/features/devices/deviceVisuals';
import { resolveDeviceSocPct } from '@/features/devices/soc';
import { usePrefersReducedMotion } from '@/shared/ui/usePrefersReducedMotion';

const INACTIVE_CARD_OPACITY = 0.82;
const INVENTORY_METRIC_LAYOUT = buildInventoryMetricLayout();

function connectivityGlyph(
  snapshot: DeviceSnapshot | undefined,
  connectionStatus: TelemetryEngineStatus
): ComponentProps<typeof MaterialCommunityIcons>['name'] {
  if (snapshot?.stale) return getStatusIconName('stale');
  if (snapshot?.inactive || snapshot?.online === false) return getStatusIconName('offline');
  if (connectionStatus === 'connecting' || connectionStatus === 'reconnecting') {
    return getStatusIconName('processing');
  }
  if (connectionStatus === 'connected' && snapshot?.online) return getStatusIconName('online');
  return getStatusIconName('waiting');
}

function getMaxSolarWatts(device: DeviceSummary): number | undefined {
  const total = device.details?.solarPorts?.reduce((sum, port) => sum + (port.maxWatts ?? 0), 0) ?? 0;
  return total > 0 ? total : undefined;
}

export function DeviceCard({
  device,
  imageContext = 'card',
  connectionStatus,
  highlighted = false,
  loadSolarHistory = true,
  inventoryCompact = false,
  fillHeight = false
}: {
  device: DeviceSummary;
  imageContext?: 'list' | 'card' | 'detail';
  connectionStatus: TelemetryEngineStatus;
  highlighted?: boolean;
  loadSolarHistory?: boolean;
  inventoryCompact?: boolean;
  fillHeight?: boolean;
}) {
  const semantics = useThemeSemantics();
  const actionButtonStyles = buildPulseActionButtonStyles(semantics, { web: Platform.OS === 'web' });
  const { contentWidth: width } = useNavigationShellMetrics();
  const snapshot = useTelemetryDeviceSnapshot(device.id);
  const { authConfigured, authReady, authKey, sessionValid, token } = useAuthSession();
  const historyEnabled = authReady && (!authConfigured || sessionValid);
  const maxSolarWatts = getMaxSolarWatts(device);
  const solarHistory = useDeviceSolarHistory(device.id, {
    token,
    authKey,
    enabled: historyEnabled && loadSolarHistory,
    maxSolarWatts
  });
  const isPhoneCompact = width < 460;
  const isTabletUp = width >= 768;
  const isDesktopWide = width >= 1200;
  const metrics = snapshot?.metrics;
  const socPct = resolveDeviceSocPct({ device, snapshot });
  const pvW = metrics?.pvW ?? device.pvW;
  const acInW = metrics?.acW ?? device.acInW;
  const dcW = metrics?.dcW ?? device.dcW;
  const loadW = metrics?.loadW ?? device.loadW;
  const netW = resolveNetPowerW({
    acInW,
    pvW,
    dcW,
    loadW,
    fallbackNetW: device.netW
  });
  const useRemoteImage = Boolean(env.assetBaseUrl);
  const { match, imageUri, fallbackSource } = resolveDeviceVisualAssets(device, {
    useRemoteImage,
    imageContext
  });
  const imageBoxSize = inventoryCompact ? 82 : isDesktopWide ? 106 : isTabletUp ? 98 : 82;
  const railWidth = inventoryCompact ? 112 : isDesktopWide ? 124 : isTabletUp ? 112 : 94;

  const fallbackStatus =
    device.state === 'charging' || device.state === 'discharging' || device.state === 'idle'
      ? device.state
      : 'idle';
  const snapshotState =
    snapshot && !snapshot.stale && snapshot.status !== 'stale'
      ? snapshot.status
      : fallbackStatus;
  const stormGuardLabel = buildStormGuardLabel(device.details);
  const capacityKWh = getCapacityKWh(device);
  const lastSeenAtCandidates = [snapshot?.lastSeenAt ?? 0, device.telemetryTsMs ?? 0];
  const freshestLastSeenAt = Math.max(...lastSeenAtCandidates);
  const lastSeenAt = freshestLastSeenAt > 0 ? freshestLastSeenAt : null;
  const isInactive =
    snapshot?.inactive ??
    (lastSeenAt !== null ? Date.now() - lastSeenAt > 60_000 : false);
  const isOffline = snapshot ? snapshot.inactive || !snapshot.online : device.online === false || isInactive;
  const cardStatusLabel = isOffline ? 'Offline' : isInactive ? 'Inactive' : null;
  const connGlyph = isOffline ? getStatusIconName('offline') : connectivityGlyph(snapshot, connectionStatus);
  const lastSeenLabel = `Last seen ${formatAgo(lastSeenAt)}`;
  const statusDotHoverLabel = buildStatusDotHoverLabel(lastSeenLabel, cardStatusLabel);
  const fadeOpacity = useRef(new Animated.Value(isInactive ? INACTIVE_CARD_OPACITY : 1)).current;
  const prefersReducedMotion = usePrefersReducedMotion();

  useEffect(() => {
    if (prefersReducedMotion) {
      fadeOpacity.stopAnimation();
      fadeOpacity.setValue(isInactive ? INACTIVE_CARD_OPACITY : 1);
      return;
    }
    Animated.timing(fadeOpacity, {
      toValue: isInactive ? INACTIVE_CARD_OPACITY : 1,
      duration: 220,
      useNativeDriver: Platform.OS !== 'web'
    }).start();
  }, [fadeOpacity, isInactive, prefersReducedMotion]);

  const metricItems = useMemo<MetricsGridItem[]>(() => {
    const contentByKey = {
      ac: (
        <Stat
          label={<IconLabel icon="power-plug-outline" label="AC" />}
          value={formatW(acInW)}
          tone={isMutedMetric(acInW) ? 'muted' : 'default'}
          compact
          minWidth={0}
        />
      ),
      dc: (
        <Stat
          label={<IconLabel icon="current-dc" label="DC" />}
          value={formatW(dcW)}
          tone={isMutedMetric(dcW) ? 'muted' : 'default'}
          compact
          minWidth={0}
        />
      ),
      pv: (
        <Stat
          label={<IconLabel icon="white-balance-sunny" label="PV" />}
          value={formatW(pvW)}
          tone={isMutedMetric(pvW) ? 'muted' : 'default'}
          compact
          minWidth={0}
        />
      ),
      load: (
        <Stat
          label={<IconLabel icon="home-outline" label="Load" />}
          value={formatW(loadW)}
          tone={isMutedMetric(loadW) ? 'muted' : 'default'}
          compact
          minWidth={0}
        />
      ),
      net: (
        <Stat
          label={<IconLabel icon="scale-balance" label="Net" />}
          value={formatW(netW)}
          compact
          minWidth={0}
        />
      )
    };
    return INVENTORY_METRIC_LAYOUT.items.map(({ key, span }) => ({
      key,
      span,
      content: contentByKey[key]
    }));
  }, [
    acInW,
    dcW,
    loadW,
    netW,
    pvW
  ]);

  return (
    <Animated.View style={{ opacity: fadeOpacity, flex: fillHeight ? 1 : undefined }}>
      <Card
        testID={`device-card-${device.id}`}
        flex={fillHeight ? 1 : undefined}
        padding={inventoryCompact ? '$3' : undefined}
        borderColor={highlighted ? '$accentColor' : undefined}
        borderWidth={highlighted ? 2 : undefined}
        shadowOpacity={highlighted ? 0.18 : undefined}
        hoverStyle={
          isInactive
            ? undefined
            : {
                transform: [{ translateY: -2 }],
                shadowOpacity: 0.14,
                borderColor: '$accentColor'
              }
        }
        pressStyle={{ scale: 0.995, opacity: 0.95 }}
        onPress={() => router.push(`/device/${device.id}`)}
        role="button"
        cursor="pointer"
        style={
          highlighted
            ? { backgroundColor: semantics.actionBackground }
            : isInactive
              ? { backgroundColor: semantics.mutedPanelBackground }
              : undefined
        }
      >
        <DeviceHeroPanel
          leftWidth={railWidth}
          imageWidth={imageBoxSize}
          imageHeight={imageBoxSize}
          imageScale={1.08}
          imageUri={imageUri}
          fallbackSource={fallbackSource}
          iconFallback={match.glyph.icon}
          leftMeta={(
            <XStack
              alignItems="center"
              justifyContent="center"
              gap="$1"
              marginTop="$1"
            >
              <MaterialCommunityIcons name="battery-high" size={14} color={semantics.subtleStrongText} />
              <Text
                fontSize={isPhoneCompact ? 11 : 13}
                numberOfLines={1}
                style={{ color: semantics.subtleStrongText }}
                opacity={0.8}
              >
                {capacityKWh !== null ? formatKWh(capacityKWh) : 'n/a'}
              </Text>
            </XStack>
          )}
          leftFooter={(
            <YStack marginTop={isPhoneCompact ? '$3' : '$2'} gap="$2" width="100%" alignItems="center">
              <PowerFlowGlyph
                status={snapshotState}
                pvW={snapshot?.metrics?.pvW ?? device.pvW}
                loadW={snapshot?.metrics?.loadW ?? device.loadW}
                fontSize={isPhoneCompact ? '$7' : '$8'}
                lineHeight={isPhoneCompact ? 26 : 30}
              />
              <SolarTodayBadge
                valueWh={solarHistory.data?.todayWh}
                previousWh={solarHistory.data?.yesterdayWh}
                deltaPct={solarHistory.data?.deltaPct}
                compact
                railCompact
              />
            </YStack>
          )}
          right={(
            <YStack gap={inventoryCompact ? '$2' : '$3'} flex={1} justifyContent="space-between">
              <XStack alignItems="flex-start" gap="$2">
                <Text
                  fontFamily="$heading"
                  fontSize={inventoryCompact || isPhoneCompact ? '$6' : '$7'}
                  fontWeight="700"
                  numberOfLines={inventoryCompact ? 2 : 1}
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
                  style={actionButtonStyles.style}
                  hoverStyle={actionButtonStyles.hoverStyle}
                  pressStyle={actionButtonStyles.pressStyle}
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
                  <XStack alignItems="center" gap="$1">
                    <MaterialCommunityIcons name="lightning-bolt-outline" size={16} color={semantics.actionText} />
                    <Text style={{ color: semantics.actionText }} fontWeight="700">Energy</Text>
                  </XStack>
                </Button>
                {cardStatusLabel ? (
                  <Text fontSize="$2" marginTop="$1" flexShrink={0} style={{ color: semantics.subtleText }}>
                    ({cardStatusLabel.toLowerCase()})
                  </Text>
                ) : null}
              </XStack>

              <Text
                fontFamily="$body"
                fontSize={isPhoneCompact ? '$2' : '$3'}
                numberOfLines={1}
                style={{ color: semantics.subtleStrongText }}
                opacity={0.84}
              >
                {device.model} · SN {maskSerialNumber(device.serialNumber)}
              </Text>

              {stormGuardLabel ? <StormGuardChip label={stormGuardLabel} /> : null}

              <YStack gap={inventoryCompact ? '$2' : '$3'}>
                <XStack alignItems="flex-end" gap="$2">
                  <YStack flex={1} minWidth={0}>
                    <SocBar value={socPct} sweepMode={snapshotState} fullWidth={inventoryCompact} />
                  </YStack>
                  <YStack
                    borderRadius="$4"
                    paddingHorizontal="$2"
                    paddingVertical="$1"
                    minWidth={72}
                    style={{
                      backgroundColor: semantics.mutedPanelBackground,
                      borderColor: semantics.mutedPanelBorder
                    }}
                    borderWidth={1}
                  >
                    <Text fontSize="$1" fontWeight="700" style={{ color: semantics.subtleStrongText }}>
                      ETA
                    </Text>
                    <Text fontSize="$1" fontWeight="800" numberOfLines={1}>
                      {formatEtaMinutes(device.etaMinutes)}
                    </Text>
                  </YStack>
                </XStack>
                <MetricsGrid items={metricItems} columns={INVENTORY_METRIC_LAYOUT.columns} padX={3} padY={3} />
              </YStack>

              <XStack justifyContent="flex-end" alignItems="center">
                <XStack
                  testID={`device-card-status-${device.id}`}
                  alignItems="center"
                  justifyContent="center"
                  minWidth={28}
                  minHeight={28}
                  borderRadius="$5"
                  opacity={0.9}
                  accessibilityLabel={statusDotHoverLabel}
                  {...(Platform.OS === 'web' ? { title: statusDotHoverLabel } : undefined)}
                >
                  <MaterialCommunityIcons name={connGlyph} size={16} color={semantics.subtleStrongText} />
                </XStack>
              </XStack>
            </YStack>
          )}
        />
      </Card>
    </Animated.View>
  );
}
