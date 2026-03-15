import { startTransition, useMemo, useState } from 'react';
import { Animated, ScrollView, useWindowDimensions } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Button, Text, XStack, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { useRequireAuth } from '@/features/auth/useRequireAuth';
import { useDevices } from '@/features/devices/hooks';
import { buildStormGuardBanner } from '@/features/devices/stormGuard';
import { useEnergyComparisonInsight, useEnergyDashboard, useEnergyPvPortHistory } from '@/features/energy/hooks';
import type { EnergyPVPortHistory } from '@/features/energy/api';
import {
  buildEnergyInsights,
  buildEnergyTrendSeries,
  buildPvEnvelopeSummary,
  buildEnergyRouteParams,
  buildPowerTrendSeries,
  buildWindowLabel,
  detectDevicesTimezone,
  ENERGY_PRESETS,
  energyPresetLabel,
  formatDeltaPct,
  MIN_MEANINGFUL_CURRENCY_BASELINE,
  MIN_MEANINGFUL_SOLAR_COMPARISON_BASELINE_KWH,
  resolveEnergyRouteState,
  type EnergyRouteState
} from '@/features/energy/model';
import { useEnergySettingsStore } from '@/features/energy/store';
import { EnergyImpactCard } from '@/features/energy-impact/EnergyImpactCard';
import { formatKWh, formatSoc } from '@/features/telemetry/format';
import { ApiError } from '@/shared/api/restClient';
import { AppMenu } from '@/shared/ui/AppMenu';
import { BatteryWindowSummary } from '@/shared/ui/BatteryWindowSummary';
import { Card } from '@/shared/ui/Card';
import { ChartSection } from '@/shared/ui/ChartSection';
import { CloseToHomeButton } from '@/shared/ui/CloseToHomeButton';
import { EnergyTrendChart } from '@/shared/ui/EnergyTrendChart';
import { EnergyComparisonWidget } from '@/shared/ui/EnergyComparisonWidget';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';
import { PowerTrendChart } from '@/shared/ui/PowerTrendChart';
import { SectionCard } from '@/shared/ui/SectionCard';
import { Stat } from '@/shared/ui/Stat';
import { StormGuardBanner } from '@/shared/ui/StormGuardBanner';
import { TopBar } from '@/shared/ui/TopBar';
import { useCloseToHomeTransition } from '@/shared/ui/useCloseToHomeTransition';
import { useThemeSemantics } from '@/shared/theme/semantic';

function formatPercent(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  return `${value.toFixed(1)}%`;
}

function formatCurrency(amount: number, currency: string): string {
  const resolvedCurrency = currency || 'USD';
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: resolvedCurrency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(amount);
}

function directionText(delta: number): string {
  if (delta > 0) return 'up';
  if (delta < 0) return 'down';
  return 'flat';
}

function formatDeltaSummary(
  delta: number,
  value: string,
  deltaPct: number | null,
  previousValue?: number | null,
  minBaseline?: number
): string {
  return `${directionText(delta)} ${value} · ${formatDeltaPct(deltaPct, { previousValue, minBaseline })}`;
}

function formatObservedAtLabel(unixMs: string): string {
  const parsed = Number.parseInt(unixMs, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return '—';
  }
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit'
  }).format(new Date(parsed));
}

function findPVHistoryRow(rows: EnergyPVPortHistory[], deviceId: string, portId: string): EnergyPVPortHistory | undefined {
  const expectedKey = canonicalPvPortKey(portId);
  return rows.find(
    (row) => row.deviceId === deviceId && canonicalPvPortKey(row.portId) === expectedKey
  );
}

function canonicalPvPortKey(portId: string): string {
  const normalized = portId.trim().toLowerCase();
  const numbered = normalized.match(/(?:^|[^a-z])pv[-_\s]?(\d+)$/i) ?? normalized.match(/^pv[-_\s]?(\d+)$/i);
  if (numbered?.[1]) {
    return `pv-${numbered[1]}`;
  }
  if (normalized.includes('low')) {
    return 'pv-1';
  }
  if (normalized.includes('high')) {
    return 'pv-2';
  }
  return normalized;
}

function buttonStyles(active: boolean, semantics: ReturnType<typeof useThemeSemantics>) {
  return {
    backgroundColor: active ? semantics.periodActiveBackground : semantics.periodIdleBackground,
    borderColor: active ? semantics.periodActiveBorder : semantics.periodIdleBorder,
    color: active ? semantics.periodActiveText : semantics.periodIdleText,
    paddingHorizontal: 15,
    paddingVertical: 9,
    minHeight: 36,
    minWidth: 96,
    borderRadius: 999
  };
}

function isAuthRequired(error: unknown): boolean {
  return error instanceof ApiError && error.status === 401;
}

export default function EnergyScreen() {
  const router = useRouter();
  const params = useLocalSearchParams();
  const semantics = useThemeSemantics();
  const { width } = useWindowDimensions();
  const { authReady, authKey, token } = useAuthSession();
  const { allowed, waiting } = useRequireAuth();
  const gridPricePerKwhInput = useEnergySettingsStore((state) => state.gridPricePerKwh);
  const currency = useEnergySettingsStore((state) => state.currency);
  const [controlsExpanded, setControlsExpanded] = useState(false);
  const devicesQuery = useDevices({
    token,
    authKey,
    enabled: authReady && allowed
  });
  const devices = devicesQuery.data?.devices ?? [];
  const stormGuardBanner = useMemo(
    () => buildStormGuardBanner(devicesQuery.data?.devices),
    [devicesQuery.data?.devices]
  );
  const fallbackTimezone = useMemo(
    () => detectDevicesTimezone(devices) || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
    [devices]
  );
  const routeState = useMemo(
    () => resolveEnergyRouteState(params, devices.map((device) => device.id), fallbackTimezone),
    [devices, fallbackTimezone, params]
  );
  const selectedDevice = devices.find((device) => device.id === routeState.deviceId);
  const dashboardQuery = useEnergyDashboard(
    {
      scope: routeState.scope,
      deviceId: routeState.deviceId,
      preset: routeState.preset,
      timezone: routeState.timezone,
      includeComparison: routeState.includeComparison,
      gridPricePerKwh: Number.parseFloat(gridPricePerKwhInput) || undefined,
      currency,
      token
    },
    {
      authKey,
      enabled: authReady && allowed
    }
  );
  const pvHistoryQuery = useEnergyPvPortHistory(
    {
      scope: routeState.scope,
      deviceId: routeState.deviceId,
      preset: routeState.preset,
      timezone: routeState.timezone,
      token
    },
    {
      authKey,
      enabled: authReady && allowed && dashboardQuery.isSuccess
    }
  );
  const comparisonInsightQuery = useEnergyComparisonInsight(
    {
      scope: routeState.scope,
      deviceId: routeState.deviceId,
      preset: routeState.preset,
      timezone: routeState.timezone,
      gridPricePerKwh: Number.parseFloat(gridPricePerKwhInput) || undefined,
      currency,
      token
    },
    {
      authKey,
      enabled: authReady && allowed
    }
  );

  const powerSeries = useMemo(
    () => buildPowerTrendSeries(dashboardQuery.data?.currentPowerPoints ?? []),
    [dashboardQuery.data?.currentPowerPoints]
  );
  const previousPowerSeries = useMemo(
    () => buildPowerTrendSeries(dashboardQuery.data?.previousPowerPoints ?? []),
    [dashboardQuery.data?.previousPowerPoints]
  );
  const energySeries = useMemo(
    () => buildEnergyTrendSeries(dashboardQuery.data?.currentEnergyPoints ?? []),
    [dashboardQuery.data?.currentEnergyPoints]
  );
  const pvEnvelope = useMemo(
    () => buildPvEnvelopeSummary(devices, routeState.scope, routeState.deviceId),
    [devices, routeState.deviceId, routeState.scope]
  );
  const pvHistoryRows = pvHistoryQuery.data ?? dashboardQuery.data?.pvPortHistory ?? [];
  const insights = useMemo(
    () =>
      buildEnergyInsights(
        dashboardQuery.data?.currentPowerPoints ?? [],
        routeState.timezone,
        routeState.preset,
        pvEnvelope.rows
      ),
    [dashboardQuery.data?.currentPowerPoints, pvEnvelope.rows, routeState.preset, routeState.timezone]
  );

  const updateRoute = (partial: Partial<EnergyRouteState>) => {
    const nextState: EnergyRouteState = {
      ...routeState,
      ...partial
    };
    if (nextState.scope === 'all') {
      nextState.deviceId = undefined;
    } else if (!nextState.deviceId) {
      nextState.deviceId = devices[0]?.id;
    }
    startTransition(() => {
      router.replace({
        pathname: '/(tabs)/energy',
        params: buildEnergyRouteParams(nextState)
      });
    });
  };

  const comparisonEnabled = routeState.includeComparison;
  const pvCardWidth = width >= 1500 ? Math.max(320, Math.floor((width - 116) / 3)) : width >= 980 ? Math.max(320, Math.floor((width - 92) / 2)) : undefined;
  const { containerStyle, closeToHome } = useCloseToHomeTransition(router);

  if (waiting || !allowed) {
    return <BrandedLoadingState minHeight={260} message="Checking session…" />;
  }

  return (
    <Animated.View style={containerStyle} testID="screen-energy">
    <YStack flex={1} backgroundColor="$background">
      <TopBar
        left={<CloseToHomeButton onClose={closeToHome} />}
        title="Energy"
        subtitle={
          dashboardQuery.data
            ? `${routeState.scope === 'all' ? 'Fleet overview' : selectedDevice?.name ?? 'Device view'} · ${buildWindowLabel(dashboardQuery.data)}`
            : 'Local-calendar energy view with server-side comparisons'
        }
        right={<AppMenu />}
      />

      <ScrollView style={{ flex: 1 }} contentContainerStyle={{ paddingHorizontal: 20, paddingBottom: 28, gap: 18 }}>
        {stormGuardBanner ? <StormGuardBanner {...stormGuardBanner} /> : null}
        <Card
          gap="$4"
          style={{
            backgroundColor: semantics.energyCardBackground,
            borderColor: semantics.energyCardBorder,
            padding: 24
          }}
        >
          <XStack justifyContent="space-between" alignItems="flex-start" gap="$3" flexWrap="wrap">
            <YStack gap="$2" flex={1} minWidth={260}>
              <Text fontSize="$7" fontWeight="800">
                Solar against load
              </Text>
              <Text color="$colorMuted">
                Compare generated energy, load, battery movement, and estimated value over a local-calendar window.
              </Text>
            </YStack>
            <Button
              size="$3"
              borderWidth={1}
              style={buttonStyles(controlsExpanded, semantics)}
              onPress={() => setControlsExpanded((value) => !value)}
            >
              {controlsExpanded ? 'Collapse ^' : 'Expand >'}
            </Button>
          </XStack>

          {controlsExpanded ? (
            <>
              <YStack gap="$3">
                <Text fontSize="$3" fontWeight="700">
                  Scope
                </Text>
                <XStack gap="$3" flexWrap="wrap">
                  <Button
                    size="$3"
                    borderWidth={1}
                    style={buttonStyles(routeState.scope === 'all', semantics)}
                    onPress={() => updateRoute({ scope: 'all' })}
                  >
                    All devices
                  </Button>
                  {devices.map((device) => (
                    <Button
                      key={device.id}
                      size="$3"
                      borderWidth={1}
                      style={buttonStyles(routeState.scope === 'device' && routeState.deviceId === device.id, semantics)}
                      onPress={() => updateRoute({ scope: 'device', deviceId: device.id })}
                    >
                      {device.name}
                    </Button>
                  ))}
                </XStack>
              </YStack>

              <YStack gap="$3">
                <Text fontSize="$3" fontWeight="700">
                  Window
                </Text>
                <XStack gap="$3" flexWrap="wrap">
                  {ENERGY_PRESETS.map((preset) => (
                    <Button
                      key={preset}
                      size="$3"
                      borderWidth={1}
                      style={buttonStyles(routeState.preset === preset, semantics)}
                      onPress={() => updateRoute({ preset })}
                    >
                      {energyPresetLabel(preset)}
                    </Button>
                  ))}
                </XStack>
              </YStack>

              <YStack gap="$3">
                <Text fontSize="$3" fontWeight="700">
                  Comparison
                </Text>
                <XStack gap="$3" flexWrap="wrap">
                  <Button
                    size="$3"
                    borderWidth={1}
                    style={buttonStyles(routeState.includeComparison, semantics)}
                    onPress={() => updateRoute({ includeComparison: true })}
                  >
                    Compare on
                  </Button>
                  <Button
                    size="$3"
                    borderWidth={1}
                    style={buttonStyles(!routeState.includeComparison, semantics)}
                    onPress={() => updateRoute({ includeComparison: false })}
                  >
                    Compare off
                  </Button>
                </XStack>
              </YStack>
            </>
          ) : null}
        </Card>

        {devicesQuery.isLoading && !devices.length ? (
          <BrandedLoadingState minHeight={200} message="Loading energy scope…" />
        ) : null}

        {dashboardQuery.isLoading && !dashboardQuery.data ? (
          <BrandedLoadingState minHeight={220} message="Loading energy dashboard…" />
        ) : null}

        {dashboardQuery.isError ? (
          <Card gap="$2">
            <Text fontSize="$5" fontWeight="700">
              Failed to load energy dashboard
            </Text>
            <Text color="$colorMuted">
              {isAuthRequired(dashboardQuery.error)
                ? 'The Energy dashboard requires a valid signed-in session.'
                : String(dashboardQuery.error)}
            </Text>
          </Card>
        ) : null}

        {dashboardQuery.data ? (
          <>
            <EnergyComparisonWidget
              data={comparisonInsightQuery.data}
              loading={comparisonInsightQuery.isLoading}
            />

            <EnergyImpactCard
              solarWh={dashboardQuery.data.summary.solarGeneratedKwh.current * 1000}
              displayPeriodLabel={buildWindowLabel(dashboardQuery.data)}
              showPeriodControls={false}
            />

            <XStack gap="$3" flexWrap="wrap">
              <SectionCard title="Solar generated" minWidth={220}>
                <Stat label="Current" value={formatKWh(dashboardQuery.data.summary.solarGeneratedKwh.current)} />
                {comparisonEnabled ? (
                  <>
                    <Stat label="Previous" value={formatKWh(dashboardQuery.data.summary.solarGeneratedKwh.previous)} tone="muted" />
                    <Text color="$colorMuted">
                      {formatDeltaSummary(
                        dashboardQuery.data.summary.solarGeneratedKwh.delta,
                        formatKWh(Math.abs(dashboardQuery.data.summary.solarGeneratedKwh.delta)),
                        dashboardQuery.data.summary.solarGeneratedKwh.deltaPct,
                        dashboardQuery.data.summary.solarGeneratedKwh.previous,
                        MIN_MEANINGFUL_SOLAR_COMPARISON_BASELINE_KWH
                      )}
                    </Text>
                  </>
                ) : null}
              </SectionCard>
              <SectionCard title="Load consumed" minWidth={220}>
                <Stat label="Current" value={formatKWh(dashboardQuery.data.summary.loadConsumedKwh.current)} />
                {comparisonEnabled ? (
                  <>
                    <Stat label="Previous" value={formatKWh(dashboardQuery.data.summary.loadConsumedKwh.previous)} tone="muted" />
                    <Text color="$colorMuted">
                      {formatDeltaSummary(
                        dashboardQuery.data.summary.loadConsumedKwh.delta,
                        formatKWh(Math.abs(dashboardQuery.data.summary.loadConsumedKwh.delta)),
                        dashboardQuery.data.summary.loadConsumedKwh.deltaPct,
                        dashboardQuery.data.summary.loadConsumedKwh.previous,
                        MIN_MEANINGFUL_SOLAR_COMPARISON_BASELINE_KWH
                      )}
                    </Text>
                  </>
                ) : null}
              </SectionCard>
              <SectionCard title="Self-sufficiency" minWidth={220}>
                <Stat label="Current" value={formatPercent(dashboardQuery.data.summary.selfSufficiencyPct.current)} />
                {comparisonEnabled ? (
                  <>
                    <Stat label="Previous" value={formatPercent(dashboardQuery.data.summary.selfSufficiencyPct.previous)} tone="muted" />
                    <Text color="$colorMuted">
                      {formatDeltaSummary(
                        dashboardQuery.data.summary.selfSufficiencyPct.delta,
                        formatPercent(Math.abs(dashboardQuery.data.summary.selfSufficiencyPct.delta)),
                        dashboardQuery.data.summary.selfSufficiencyPct.deltaPct
                      )}
                    </Text>
                  </>
                ) : null}
              </SectionCard>
              <SectionCard title="Battery net" minWidth={220}>
                <Stat label="Current" value={formatKWh(dashboardQuery.data.summary.batteryNetKwh.current)} />
                {comparisonEnabled ? (
                  <>
                    <Stat label="Previous" value={formatKWh(dashboardQuery.data.summary.batteryNetKwh.previous)} tone="muted" />
                    <Text color="$colorMuted">
                      {formatDeltaSummary(
                        dashboardQuery.data.summary.batteryNetKwh.delta,
                        formatKWh(Math.abs(dashboardQuery.data.summary.batteryNetKwh.delta)),
                        dashboardQuery.data.summary.batteryNetKwh.deltaPct
                      )}
                    </Text>
                  </>
                ) : null}
              </SectionCard>
              <SectionCard title="End SOC" minWidth={220}>
                <Stat label="SoC" value={formatSoc(dashboardQuery.data.battery.socEndPct)} />
                {comparisonEnabled ? (
                  <Stat label="Previous baseline" value={formatSoc(dashboardQuery.data.battery.socStartPct)} tone="muted" />
                ) : null}
                <Text color="$colorMuted">
                  {`Band ${formatSoc(dashboardQuery.data.battery.socMinPct)} - ${formatSoc(dashboardQuery.data.battery.socMaxPct)}`}
                </Text>
              </SectionCard>
              <SectionCard title="Estimated value" minWidth={220}>
                <Stat
                  label="Current"
                  value={formatCurrency(dashboardQuery.data.summary.estimatedValue.current, dashboardQuery.data.summary.currency)}
                />
                {comparisonEnabled ? (
                  <>
                    <Stat
                      label="Previous"
                      value={formatCurrency(dashboardQuery.data.summary.estimatedValue.previous, dashboardQuery.data.summary.currency)}
                      tone="muted"
                    />
                    <Text color="$colorMuted">
                      {formatDeltaSummary(
                        dashboardQuery.data.summary.estimatedValue.delta,
                        formatCurrency(
                          Math.abs(dashboardQuery.data.summary.estimatedValue.delta),
                          dashboardQuery.data.summary.currency
                        ),
                        dashboardQuery.data.summary.estimatedValue.deltaPct,
                        dashboardQuery.data.summary.estimatedValue.previous,
                        MIN_MEANINGFUL_CURRENCY_BASELINE
                      )}
                    </Text>
                  </>
                ) : null}
              </SectionCard>
            </XStack>

            <XStack gap="$3" flexWrap="wrap">
              <SectionCard title="Battery flow" minWidth={320} flex={2}>
                <BatteryWindowSummary
                  chargeKwh={dashboardQuery.data.battery.chargeKwh}
                  dischargeKwh={dashboardQuery.data.battery.dischargeKwh}
                  netKwh={dashboardQuery.data.battery.netKwh}
                  socStartPct={dashboardQuery.data.battery.socStartPct}
                  socEndPct={dashboardQuery.data.battery.socEndPct}
                  socMinPct={dashboardQuery.data.battery.socMinPct}
                  socMaxPct={dashboardQuery.data.battery.socMaxPct}
                />
              </SectionCard>
              <SectionCard title="AC input cost" minWidth={220}>
                <Stat
                  label="Current"
                  value={formatCurrency(dashboardQuery.data.summary.estimatedAcInputCost.current, dashboardQuery.data.summary.currency)}
                />
                {comparisonEnabled ? (
                  <>
                    <Stat
                      label="Previous"
                      value={formatCurrency(
                        dashboardQuery.data.summary.estimatedAcInputCost.previous,
                        dashboardQuery.data.summary.currency
                      )}
                      tone="muted"
                    />
                    <Text color="$colorMuted">
                      {formatDeltaPct(dashboardQuery.data.summary.estimatedAcInputCost.deltaPct, {
                        previousValue: dashboardQuery.data.summary.estimatedAcInputCost.previous,
                        minBaseline: MIN_MEANINGFUL_CURRENCY_BASELINE
                      })}
                    </Text>
                  </>
                ) : null}
              </SectionCard>
              <SectionCard title="Scope details" minWidth={220}>
                <Stat label="Resolved devices" value={String(dashboardQuery.data.scope.resolvedDeviceIds.length)} />
                <Stat label="Timezone" value={dashboardQuery.data.window.timezone} compact />
                <Stat label="Preset" value={energyPresetLabel(routeState.preset)} compact />
              </SectionCard>
            </XStack>

            <ChartSection
              title="Power profile"
              subtitle="Server-returned power buckets from the selected window."
            >
              {dashboardQuery.data.currentPowerPoints.length > 1 ? (
                <YStack gap="$2">
                  <PowerTrendChart
                    solar={powerSeries.solar}
                    ac={powerSeries.ac}
                    dc={powerSeries.dc}
                    load={powerSeries.load}
                    battery={powerSeries.battery}
                    previousSolar={previousPowerSeries.solar}
                    previousAc={previousPowerSeries.ac}
                    previousDc={previousPowerSeries.dc}
                    previousLoad={previousPowerSeries.load}
                    previousBattery={previousPowerSeries.battery}
                    points={dashboardQuery.data.currentPowerPoints.length}
                    bucketSeconds={resolveBucketSeconds(routeState.preset)}
                  />
                  <Text color="$colorMuted">
                    {comparisonEnabled
                      ? `Current points: ${dashboardQuery.data.currentPowerPoints.length} · Previous points: ${dashboardQuery.data.previousPowerPoints.length}`
                      : `Current points: ${dashboardQuery.data.currentPowerPoints.length}`}
                  </Text>
                </YStack>
              ) : (
                <Text color="$colorMuted">
                  Power bucket history is not populated yet for this window.
                </Text>
              )}
            </ChartSection>

            <ChartSection
              title="Energy history"
              subtitle="Server-returned energy buckets for solar, grid input, and load over the selected local-calendar window."
            >
              {dashboardQuery.data.currentEnergyPoints.length > 1 ? (
                <YStack gap="$2">
                  <EnergyTrendChart
                    solar={energySeries.solar}
                    grid={energySeries.grid}
                    acOutput={energySeries.acOutput}
                    load={energySeries.load}
                    dcOutput={energySeries.dcOutput}
                    batteryCharge={energySeries.batteryCharge}
                    batteryDischarge={energySeries.batteryDischarge}
                    points={dashboardQuery.data.currentEnergyPoints.length}
                    bucketSeconds={resolveEnergyBucketSeconds(routeState.preset)}
                  />
                  <Text color="$colorMuted">
                    {comparisonEnabled
                      ? `Current points: ${dashboardQuery.data.currentEnergyPoints.length} · Previous points: ${dashboardQuery.data.previousEnergyPoints.length}`
                      : `Current points: ${dashboardQuery.data.currentEnergyPoints.length}`}
                  </Text>
                </YStack>
              ) : (
                <Text color="$colorMuted">
                  Energy bucket history is not populated yet for this window.
                </Text>
              )}
            </ChartSection>

            <XStack gap="$3" flexWrap="wrap">
              {insights.map((insight) => (
                <SectionCard key={insight.title} title={insight.title} minWidth={240}>
                  <Text color="$colorMuted">{insight.body}</Text>
                </SectionCard>
              ))}
            </XStack>

            <SectionCard
              title="PV operating envelope"
              fullWidth
              right={(
                <Text color="$colorMuted">
                  {pvEnvelope.utilizationPct === null ? 'No PV capability metadata' : `${pvEnvelope.utilizationPct.toFixed(1)}% of configured power`}
                </Text>
              )}
            >
              <XStack gap="$3" flexWrap="wrap">
                <Stat label="Observed power" value={`${Math.round(pvEnvelope.observedPower)} W`} />
                <Stat label="Configured power" value={`${Math.round(pvEnvelope.configuredPower)} W`} />
                <Stat label="Observed volts" value={`${Math.round(pvEnvelope.observedVolts)} V`} />
                <Stat label="Observed amps" value={`${pvEnvelope.observedAmps.toFixed(1)} A`} />
                <Stat label="Top device peak" value={pvEnvelope.topDevicePeakLabel ?? '—'} compact />
              </XStack>
              {pvEnvelope.rows.length ? (
                routeState.scope === 'all' ? (
                  <XStack gap="$2" flexWrap="wrap">
                    {pvEnvelope.rows.map((row) => {
                      const historyRow = findPVHistoryRow(
                        pvHistoryRows,
                        row.deviceId,
                        row.portId
                      );
                      return (
                        <YStack
                          key={`${row.deviceId}:${row.portId}`}
                          gap="$2"
                          padding="$3"
                          borderRadius="$3"
                          borderWidth={1}
                          width={pvCardWidth}
                          style={{
                            borderColor: semantics.mutedPanelBorder,
                            backgroundColor: semantics.mutedPanelBackground
                          }}
                        >
                          <XStack justifyContent="space-between" alignItems="flex-start" gap="$2" flexWrap="wrap">
                            <YStack gap="$1" minWidth={160} flex={1}>
                              <Text fontWeight="700">{row.deviceName}</Text>
                              <Text color="$colorMuted">{`${row.portLabel} · ${Math.round(row.maxPower)}W configured`}</Text>
                              <Text color="$colorMuted">{`${row.maxVolts.toFixed(0)}V / ${row.maxAmps.toFixed(1)}A envelope`}</Text>
                            </YStack>
                            <YStack alignItems="flex-end" gap="$1">
                              <Text fontSize="$1" color="$colorMuted">Bottleneck</Text>
                              <Text color="$colorMuted">{row.bottleneckHint}</Text>
                            </YStack>
                          </XStack>
                          <XStack gap="$3" flexWrap="wrap">
                            <Stat label="Observed" value={`${Math.round(row.observedPower)}W`} compact />
                            <Stat label="Observed V/A" value={`${row.observedVolts.toFixed(1)}V · ${row.observedAmps.toFixed(2)}A`} compact />
                            <Stat
                              label="Utilization"
                              value={row.powerUtilizationPct === null ? '—' : `${row.powerUtilizationPct.toFixed(1)}%`}
                              compact
                            />
                            <Stat
                              label="Headroom"
                              value={
                                row.voltageHeadroom === null || row.currentHeadroom === null
                                  ? '—'
                                  : `${row.voltageHeadroom.toFixed(1)}V · ${row.currentHeadroom.toFixed(1)}A`
                              }
                              compact
                            />
                            <Stat
                              label="Last seen"
                              value={pvHistoryQuery.isLoading ? '…' : historyRow ? formatObservedAtLabel(historyRow.lastObservedUnixMs) : '—'}
                              compact
                            />
                          </XStack>
                          <XStack gap="$3" flexWrap="wrap">
                            <Stat
                              label="Hist max"
                              value={pvHistoryQuery.isLoading ? '…' : historyRow ? `${Math.round(historyRow.maxObservedWatts)}W` : '—'}
                              compact
                            />
                            <Stat
                              label="Hist V/A"
                              value={
                                pvHistoryQuery.isLoading
                                  ? 'Loading…'
                                  : historyRow
                                  ? `${historyRow.maxObservedVolts.toFixed(1)}V · ${historyRow.maxObservedAmps.toFixed(2)}A`
                                  : 'No history'
                              }
                              compact
                            />
                          </XStack>
                        </YStack>
                      );
                    })}
                  </XStack>
                ) : (
                  <YStack gap="$2">
                    {pvEnvelope.rows.map((row) => {
                      const historyRow = findPVHistoryRow(
                        pvHistoryRows,
                        row.deviceId,
                        row.portId
                      );
                      return (
                        <YStack
                          key={`${row.deviceId}:${row.portId}`}
                          gap="$1"
                          padding="$2"
                          borderRadius="$3"
                          borderWidth={1}
                          style={{
                            borderColor: semantics.mutedPanelBorder,
                            backgroundColor: semantics.mutedPanelBackground
                          }}
                        >
                          <XStack justifyContent="space-between" alignItems="center" gap="$2">
                            <Text fontWeight="700">{row.portLabel}</Text>
                            <Text color="$colorMuted">{row.bottleneckHint}</Text>
                          </XStack>
                          <XStack gap="$3" flexWrap="wrap">
                            <Stat label="Observed" value={`${Math.round(row.observedPower)}W`} compact />
                            <Stat label="Max" value={`${Math.round(row.maxPower)}W`} compact />
                            <Stat label="Utilization" value={row.powerUtilizationPct === null ? '—' : `${row.powerUtilizationPct.toFixed(1)}%`} compact />
                            <Stat label="Headroom V" value={row.voltageHeadroom === null ? '—' : `${row.voltageHeadroom.toFixed(1)}V`} compact />
                            <Stat label="Headroom A" value={row.currentHeadroom === null ? '—' : `${row.currentHeadroom.toFixed(1)}A`} compact />
                          </XStack>
                          {pvHistoryQuery.isLoading ? (
                            <Text color="$colorMuted">
                              Loading historical PV observations…
                            </Text>
                          ) : historyRow ? (
                            <XStack gap="$3" flexWrap="wrap">
                              <Stat label="Hist max W" value={`${Math.round(historyRow.maxObservedWatts)}W`} compact />
                              <Stat label="Hist max V" value={`${historyRow.maxObservedVolts.toFixed(1)}V`} compact />
                              <Stat label="Hist max A" value={`${historyRow.maxObservedAmps.toFixed(2)}A`} compact />
                              <Stat label="Samples" value={String(historyRow.sampleCount)} compact />
                              <Stat label="Last seen" value={formatObservedAtLabel(historyRow.lastObservedUnixMs)} compact />
                            </XStack>
                          ) : (
                            <Text color="$colorMuted">
                              Historical PV observations are not available for this port in the selected window.
                            </Text>
                          )}
                        </YStack>
                      );
                    })}
                  </YStack>
                )
              ) : (
                <Text color="$colorMuted">
                  No PV port capability data is available for the selected scope yet.
                </Text>
              )}
            </SectionCard>
          </>
        ) : null}
      </ScrollView>
    </YStack>
    </Animated.View>
  );
}

function resolveBucketSeconds(preset: EnergyRouteState['preset']): number {
  switch (preset) {
    case 'today':
    case 'past24h':
    case 'yesterday':
      return 300;
    case 'thisWeek':
    case 'previousWeek':
    case 'last7d':
      return 3600;
    case 'last30d':
    case 'lastMonth':
    case 'thisMonth':
      return 86400;
    case 'last12m':
      return 86400;
  }
}

function resolveEnergyBucketSeconds(preset: EnergyRouteState['preset']): number {
  switch (preset) {
    case 'today':
    case 'past24h':
    case 'yesterday':
      return 3600;
    case 'last7d':
    case 'last30d':
    case 'thisWeek':
    case 'previousWeek':
    case 'thisMonth':
    case 'lastMonth':
    case 'last12m':
      return 86400;
  }
}
