import { startTransition, useMemo, useState, type ComponentProps } from 'react';
import { Animated, Platform, ScrollView, type DimensionValue } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { MaterialCommunityIcons } from '@expo/vector-icons';
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
  ENERGY_PANELS,
  ENERGY_PRESETS,
  energyPanelLabel,
  energyPresetLabel,
  formatDeltaPct,
  MIN_MEANINGFUL_CURRENCY_BASELINE,
  MIN_MEANINGFUL_SOLAR_COMPARISON_BASELINE_KWH,
  normalizePvObservedPower,
  resolveEnergyRouteState,
  type EnergyRouteState
} from '@/features/energy/model';
import { DeviceEnergyImpactCard } from '@/features/energy-impact/DeviceEnergyImpactCard';
import { FleetEnergyImpactCard } from '@/features/energy-impact/FleetEnergyImpactCard';
import { useEnergySettingsStore } from '@/features/energy/store';
import { useCurrentUser } from '@/features/profile/hooks';
import { useProfileWeather } from '@/features/weather/hooks';
import { resolveProfileWeatherState } from '@/features/weather/model';
import { WeatherCurrentWidget } from '@/features/weather/WeatherCurrentWidget';
import { WeatherForecastCard } from '@/features/weather/WeatherForecastCard';
import { formatKWh, formatSoc } from '@/features/telemetry/format';
import { ApiError } from '@/shared/api/restClient';
import { AppMenu } from '@/shared/ui/AppMenu';
import { BatteryWindowSummary } from '@/shared/ui/BatteryWindowSummary';
import { BreadcrumbTrail } from '@/shared/ui/BreadcrumbTrail';
import { Card } from '@/shared/ui/Card';
import { ChartSection } from '@/shared/ui/ChartSection';
import { EnergyTrendChart } from '@/shared/ui/EnergyTrendChart';
import { EnergyComparisonWidget } from '@/shared/ui/EnergyComparisonWidget';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';
import { PowerTrendChart } from '@/shared/ui/PowerTrendChart';
import { SectionCard } from '@/shared/ui/SectionCard';
import { Stat } from '@/shared/ui/Stat';
import { StormGuardBanner } from '@/shared/ui/StormGuardBanner';
import { TopBar } from '@/shared/ui/TopBar';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';
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

function displayHistoricalPvWatts(historyRow: EnergyPVPortHistory): number {
  return normalizePvObservedPower(historyRow.maxObservedWatts);
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

function EnergyMetricTile({
  icon,
  label,
  value,
  detail,
  accent,
  width
}: {
  icon: ComponentProps<typeof MaterialCommunityIcons>['name'];
  label: string;
  value: string;
  detail?: string;
  accent: string;
  width?: DimensionValue;
}) {
  const semantics = useThemeSemantics();

  return (
    <YStack
      width={width}
      minWidth={220}
      gap="$3"
      padding="$4"
      borderRadius="$4"
      borderWidth={1}
      style={{
        backgroundColor: semantics.tileBackground,
        borderColor: semantics.tileBorder
      }}
    >
      <XStack alignItems="center" justifyContent="space-between" gap="$2">
        <Text fontSize="$2" style={{ color: semantics.subtleStrongText }} textTransform="uppercase" letterSpacing={0.6}>
          {label}
        </Text>
        <YStack
          width={34}
          height={34}
          borderRadius={999}
          alignItems="center"
          justifyContent="center"
          style={{ backgroundColor: `${accent}1f` }}
        >
          <MaterialCommunityIcons name={icon} size={18} color={accent} />
        </YStack>
      </XStack>
      <Text fontSize="$8" fontWeight="800" letterSpacing={-0.5} numberOfLines={1}>
        {value}
      </Text>
      <Text fontSize="$2" style={{ color: semantics.subtleText }} numberOfLines={2}>
        {detail ?? ' '}
      </Text>
    </YStack>
  );
}

function isAuthRequired(error: unknown): boolean {
  return error instanceof ApiError && error.status === 401;
}

function describeQueryError(error: unknown): string | undefined {
  if (!error) {
    return undefined;
  }
  return error instanceof Error ? error.message : String(error);
}

function buildEnergyHref(state: EnergyRouteState): string {
  const query = new URLSearchParams(buildEnergyRouteParams(state));
  return `/(tabs)/energy?${query.toString()}`;
}

function splitSectionWrap(isWideLayout: boolean): 'nowrap' | 'wrap' {
  return isWideLayout ? 'nowrap' : 'wrap';
}

export default function EnergyScreen() {
  const router = useRouter();
  const params = useLocalSearchParams();
  const semantics = useThemeSemantics();
  const { contentWidth: width } = useNavigationShellMetrics();
  const { authReady, authKey, token } = useAuthSession();
  const { allowed, waiting } = useRequireAuth();
  const gridPricePerKwhInput = useEnergySettingsStore((state) => state.gridPricePerKwh);
  const currency = useEnergySettingsStore((state) => state.currency);
  const [controlsExpanded, setControlsExpanded] = useState(false);
  const [verificationRequested, setVerificationRequested] = useState(false);
  const devicesQuery = useDevices({
    token,
    authKey,
    enabled: authReady && allowed
  });
  const currentUserQuery = useCurrentUser({
    token,
    authKey,
    enabled: authReady && allowed
  });
  const devices = useMemo(() => devicesQuery.data?.devices ?? [], [devicesQuery.data?.devices]);
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
  const resolvedWeatherState = resolveProfileWeatherState(currentUserQuery.data?.user);
  const profileWeather = useProfileWeather({
    token,
    authKey,
    locationKey: resolvedWeatherState.locationKey,
    enabled:
      authReady &&
      allowed &&
      routeState.panel === 'solar' &&
      resolvedWeatherState.enabled &&
      (routeState.scope === 'all' || Boolean(routeState.deviceId)),
    verificationEnabled: routeState.panel === 'solar' && verificationRequested,
    scope: routeState.scope,
    deviceId: routeState.scope === 'device' ? routeState.deviceId : undefined
  });
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
  const previousEnergySeries = useMemo(
    () => buildEnergyTrendSeries(dashboardQuery.data?.previousEnergyPoints ?? []),
    [dashboardQuery.data?.previousEnergyPoints]
  );
  const previousEnergyNet = useMemo(
    () =>
      previousEnergySeries.solar.map((_, idx) =>
        (previousEnergySeries.solar[idx] ?? 0)
        + (previousEnergySeries.grid[idx] ?? 0)
        + (previousEnergySeries.batteryCharge[idx] ?? 0)
        - (previousEnergySeries.load[idx] ?? 0)
        - (previousEnergySeries.acOutput[idx] ?? 0)
        - (previousEnergySeries.dcOutput[idx] ?? 0)
        - (previousEnergySeries.batteryDischarge[idx] ?? 0)
      ),
    [previousEnergySeries]
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
  const weatherErrorText =
    describeQueryError(profileWeather.forecastQuery.error) ??
    describeQueryError(profileWeather.solarOutlookQuery.error);
  const weatherEnabled = authReady && allowed && resolvedWeatherState.enabled;
  const weatherIsLoading =
    routeState.panel === 'solar' &&
    (profileWeather.forecastQuery.isLoading || profileWeather.solarOutlookQuery.isLoading);

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
  const isWideLayout = width >= 1180;
  const selectedScopeLabel = routeState.scope === 'all' ? 'Fleet' : selectedDevice?.name ?? 'Device';
  const breadcrumbScopeLabel = routeState.scope === 'all' ? 'All devices' : selectedDevice?.name ?? 'Device';
  const topCardTitle =
    routeState.panel === 'solar'
      ? 'Solar forecast'
      : routeState.panel === 'impact'
        ? 'Energy impact'
        : 'Solar against load';
  const topCardSubtitle =
    routeState.panel === 'solar'
      ? 'Weather-aware solar guidance for the current fleet or device scope.'
      : routeState.panel === 'impact'
        ? 'Measured solar generation translated into avoided emissions and other real-world equivalents.'
        : 'Compare generated energy, load, battery movement, and estimated value over a local-calendar window.';

  if (waiting || !allowed) {
    return <BrandedLoadingState minHeight={260} message="Checking session…" />;
  }

  return (
    <Animated.View style={{ flex: 1 }} testID="screen-energy">
    <YStack flex={1} backgroundColor="$background">
      <TopBar
        eyebrow={(
          <BreadcrumbTrail
            items={
              routeState.scope === 'all'
                ? [
                    {
                      label: 'Home',
                      href: '/(tabs)/devices',
                      icon: 'home-variant-outline',
                      hideLabel: true
                    },
                    {
                      label: 'Energy',
                      href: buildEnergyHref({
                        ...routeState,
                        scope: 'all',
                        deviceId: undefined,
                        panel: 'overview'
                      })
                    },
                    {
                      label: energyPanelLabel(routeState.panel),
                      href:
                        routeState.panel === 'overview'
                          ? undefined
                          : buildEnergyHref({
                              ...routeState,
                              scope: 'all',
                              deviceId: undefined
                            })
                    },
                    {
                      label: breadcrumbScopeLabel,
                      current: true
                    }
                  ]
                : [
                    {
                      label: 'Home',
                      href: '/(tabs)/devices',
                      icon: 'home-variant-outline',
                      hideLabel: true
                    },
                    {
                      label: 'Energy',
                      href: buildEnergyHref({
                        ...routeState,
                        scope: 'all',
                        deviceId: undefined,
                        panel: 'overview'
                      })
                    },
                    {
                      label: energyPanelLabel(routeState.panel),
                      href: buildEnergyHref({
                        ...routeState,
                        scope: 'all',
                        deviceId: undefined
                      })
                    },
                    {
                      label: breadcrumbScopeLabel,
                      current: true
                    }
                  ]
            }
          />
        )}
        title="Energy"
        subtitle={
          routeState.panel === 'overview' && dashboardQuery.data
            ? `${routeState.scope === 'all' ? 'Fleet overview' : selectedDevice?.name ?? 'Device view'} · ${buildWindowLabel(dashboardQuery.data)}`
            : routeState.panel === 'solar'
              ? `${selectedScopeLabel} solar forecast and weather-aware generation outlook`
              : 'Measured solar translated into avoided-emissions summaries and lifecycle equivalents'
        }
        right={(
          <AppMenu
            weatherScope={routeState.scope}
            weatherDeviceId={routeState.scope === 'device' ? routeState.deviceId : undefined}
          />
        )}
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
                {topCardTitle}
              </Text>
              <Text color="$colorMuted">
                {topCardSubtitle}
              </Text>
            </YStack>
            <Button
              size="$3"
              circular
              borderWidth={1}
              style={buttonStyles(controlsExpanded, semantics)}
              accessibilityLabel={controlsExpanded ? 'Collapse solar against load controls' : 'Expand solar against load controls'}
              onPress={() => setControlsExpanded((value) => !value)}
            >
              <MaterialCommunityIcons
                name={controlsExpanded ? 'chevron-up' : 'chevron-down'}
                size={20}
                color={controlsExpanded ? semantics.periodActiveText : semantics.periodIdleText}
              />
            </Button>
          </XStack>

          <YStack gap="$3">
            <Text fontSize="$3" fontWeight="700">
              Panel
            </Text>
            <XStack gap="$3" flexWrap="wrap">
              {ENERGY_PANELS.map((panel) => (
                <Button
                  key={panel}
                  size="$3"
                  borderWidth={1}
                  style={buttonStyles(routeState.panel === panel, semantics)}
                  onPress={() => updateRoute({ panel })}
                >
                  {energyPanelLabel(panel)}
                </Button>
              ))}
            </XStack>
          </YStack>

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

              {routeState.panel === 'overview' ? (
                <>
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
            </>
          ) : null}
        </Card>

        {devicesQuery.isLoading && !devices.length ? (
          <BrandedLoadingState minHeight={200} message="Loading energy scope…" />
        ) : null}

        {routeState.panel === 'overview' && dashboardQuery.isLoading && !dashboardQuery.data ? (
          <BrandedLoadingState minHeight={220} message="Loading energy dashboard…" />
        ) : null}

        {routeState.panel === 'overview' && dashboardQuery.isError ? (
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

        {routeState.panel === 'solar' ? (
          <YStack gap="$3">
            <Card
              gap="$3"
              style={{
                backgroundColor: semantics.energyCardBackground,
                borderColor: semantics.energyCardBorder
              }}
            >
              <XStack justifyContent="space-between" alignItems="flex-start" gap="$3" flexWrap="wrap">
                <YStack gap="$1" flex={1} minWidth={240}>
                  <Text fontSize="$6" fontWeight="800">
                    {routeState.scope === 'all'
                      ? 'Fleet solar forecast'
                      : `${selectedDevice?.name ?? 'Device'} solar forecast`}
                  </Text>
                  <Text color="$colorMuted">
                    Weather-aware production outlook for the current Energy scope. Use Scope controls above to deep-link this pane for a specific device.
                  </Text>
                </YStack>
                <YStack
                  paddingHorizontal="$3"
                  paddingVertical="$2"
                  borderRadius="$4"
                  borderWidth={1}
                  style={{
                    backgroundColor: semantics.mutedPanelBackground,
                    borderColor: semantics.mutedPanelBorder
                  }}
                >
                  <Text fontSize="$2" fontWeight="700" style={{ color: semantics.subtleStrongText }}>
                    {routeState.scope === 'all' ? 'Site totals' : 'Device scoped'}
                  </Text>
                </YStack>
              </XStack>
            </Card>

            <XStack
              gap="$3"
              flexWrap={splitSectionWrap(isWideLayout)}
              flexDirection={isWideLayout ? 'row' : 'column'}
            >
              <YStack flex={1} minWidth={0}>
                <WeatherCurrentWidget
                  forecast={profileWeather.forecastQuery.data?.forecast}
                  solarOutlook={profileWeather.solarOutlook}
                  isLoading={weatherIsLoading}
                  enabled={weatherEnabled}
                  errorText={weatherErrorText}
                />
              </YStack>
              <YStack width={isWideLayout ? 320 : '100%'}>
                <Card
                  gap="$3"
                  minHeight={220}
                  style={{
                    backgroundColor: semantics.tileBackground,
                    borderColor: semantics.tileBorder
                  }}
                >
                  <Text fontSize="$5" fontWeight="700">
                    Scope summary
                  </Text>
                  <Stat label="Scope" value={selectedScopeLabel} />
                  <Stat label="Mode" value={routeState.scope === 'all' ? 'Site forecast' : 'Per-device forecast'} compact />
                  <Stat label="Weather consent" value={resolvedWeatherState.enabled ? 'Enabled' : 'Needed'} compact />
                  <Button
                    size="$4"
                    borderRadius={999}
                    borderWidth={1}
                    paddingHorizontal="$4"
                    minHeight={42}
                    alignSelf="flex-start"
                    style={{
                      backgroundColor: semantics.actionBackground,
                      borderColor: semantics.actionBorder
                    }}
                    onPress={() => router.push('/profile')}
                  >
                    <XStack alignItems="center" gap="$2">
                      <MaterialCommunityIcons name="cog-outline" size={18} color={semantics.actionText} />
                      <Text style={{ color: semantics.actionText }} fontWeight="700">
                        Weather settings
                      </Text>
                    </XStack>
                  </Button>
                </Card>
              </YStack>
            </XStack>

            <WeatherForecastCard
              forecast={profileWeather.forecastQuery.data?.forecast}
              solarOutlook={profileWeather.solarOutlook}
              verification={profileWeather.verificationQuery.data?.verification}
              isLoading={weatherIsLoading}
              verificationIsLoading={profileWeather.verificationQuery.isLoading}
              enabled={weatherEnabled}
              errorText={weatherErrorText}
              verificationErrorText={describeQueryError(profileWeather.verificationQuery.error)}
              onVerificationExpand={() => {
                setVerificationRequested(true);
              }}
            />
          </YStack>
        ) : routeState.panel === 'impact' ? (
          <YStack gap="$3">
            <Card
              gap="$3"
              style={{
                backgroundColor: semantics.energyCardBackground,
                borderColor: semantics.energyCardBorder
              }}
            >
              <Text fontSize="$6" fontWeight="800">
                Energy Impact
              </Text>
              <Text color="$colorMuted">
                Measured solar generation only. No annualized estimates, no invented totals, just the currently selected fleet or device scope translated into avoided-emissions summaries.
              </Text>
            </Card>
            {routeState.scope === 'all' ? (
              <FleetEnergyImpactCard devices={devices} variant="detailed" />
            ) : (
              <DeviceEnergyImpactCard
                deviceId={routeState.deviceId}
                todaySolarWh={(dashboardQuery.data?.summary.solarGeneratedKwh.current ?? 0) * 1000}
                variant="detailed"
              />
            )}
          </YStack>
        ) : dashboardQuery.data ? (
          <>
            <Card
              gap="$4"
              style={
                Platform.OS === 'web'
                  ? {
                      backgroundImage: `${semantics.heroBackground}, radial-gradient(circle at 72% 20%, ${semantics.heroGlow} 0%, rgba(0,0,0,0) 42%)`,
                      borderColor: semantics.heroBorder,
                      padding: 28
                    }
                  : {
                      backgroundColor: semantics.surfaceRaised,
                      borderColor: semantics.heroBorder,
                      padding: 28
                    }
              }
            >
              <XStack justifyContent="space-between" alignItems="flex-start" gap="$4" flexWrap="wrap">
                <YStack gap="$3" flex={1} minWidth={280}>
                  <YStack gap="$2">
                    <Text
                      fontSize="$2"
                      fontWeight="700"
                      textTransform="uppercase"
                      letterSpacing={0.8}
                      style={{ color: semantics.subtleStrongText }}
                    >
                      Solar first
                    </Text>
                    <Text fontSize="$8" fontWeight="800" letterSpacing={-0.8}>
                      {formatKWh(dashboardQuery.data.summary.solarGeneratedKwh.current)}
                    </Text>
                    <Text fontSize="$4" style={{ color: semantics.subtleStrongText }}>
                      {`${routeState.scope === 'all' ? 'Fleet overview' : selectedDevice?.name ?? 'Device view'} · ${buildWindowLabel(dashboardQuery.data)}`}
                    </Text>
                  </YStack>

                  <XStack gap="$2" flexWrap="wrap">
                    {[
                      `Timezone ${dashboardQuery.data.window.timezone}`,
                      energyPresetLabel(routeState.preset),
                      comparisonEnabled ? 'Comparison on' : 'Comparison off'
                    ].map((label) => (
                      <YStack
                        key={label}
                        paddingHorizontal="$3"
                        paddingVertical="$2"
                        borderRadius={999}
                        borderWidth={1}
                        style={{
                          backgroundColor: semantics.mutedPanelBackground,
                          borderColor: semantics.mutedPanelBorder
                        }}
                      >
                        <Text fontSize="$2" fontWeight="700" style={{ color: semantics.subtleStrongText }}>
                          {label}
                        </Text>
                      </YStack>
                    ))}
                  </XStack>
                </YStack>

                <XStack gap="$4" flexWrap="wrap" minWidth={260}>
                  <YStack gap="$1" minWidth={120}>
                    <Text fontSize="$2" style={{ color: semantics.subtleText }} textTransform="uppercase" letterSpacing={0.6}>
                      Estimated value
                    </Text>
                    <Text fontSize="$6" fontWeight="800" letterSpacing={-0.4}>
                      {formatCurrency(
                        dashboardQuery.data.summary.estimatedValue.current,
                        dashboardQuery.data.summary.currency
                      )}
                    </Text>
                  </YStack>
                  <YStack gap="$1" minWidth={120}>
                    <Text fontSize="$2" style={{ color: semantics.subtleText }} textTransform="uppercase" letterSpacing={0.6}>
                      Self-sufficiency
                    </Text>
                    <Text fontSize="$6" fontWeight="800" letterSpacing={-0.4}>
                      {formatPercent(dashboardQuery.data.summary.selfSufficiencyPct.current)}
                    </Text>
                  </YStack>
                  <YStack gap="$1" minWidth={120}>
                    <Text fontSize="$2" style={{ color: semantics.subtleText }} textTransform="uppercase" letterSpacing={0.6}>
                      Battery net
                    </Text>
                    <Text fontSize="$6" fontWeight="800" letterSpacing={-0.4}>
                      {formatKWh(dashboardQuery.data.summary.batteryNetKwh.current)}
                    </Text>
                  </YStack>
                </XStack>
              </XStack>
            </Card>

            <EnergyComparisonWidget
              data={comparisonInsightQuery.data}
              loading={comparisonInsightQuery.isLoading}
            />

            <XStack
              gap="$3"
              flexWrap={splitSectionWrap(isWideLayout)}
              flexDirection={isWideLayout ? 'row' : 'column'}
            >
              <YStack gap="$3" width={isWideLayout ? 320 : '100%'}>
                <EnergyMetricTile
                  icon="solar-power-variant-outline"
                  label="Solar generated"
                  value={formatKWh(dashboardQuery.data.summary.solarGeneratedKwh.current)}
                  detail={
                    comparisonEnabled
                      ? formatDeltaSummary(
                          dashboardQuery.data.summary.solarGeneratedKwh.delta,
                          formatKWh(Math.abs(dashboardQuery.data.summary.solarGeneratedKwh.delta)),
                          dashboardQuery.data.summary.solarGeneratedKwh.deltaPct,
                          dashboardQuery.data.summary.solarGeneratedKwh.previous,
                          MIN_MEANINGFUL_SOLAR_COMPARISON_BASELINE_KWH
                        )
                      : 'Measured from the selected local-calendar window.'
                  }
                  accent={semantics.chartSolar}
                />
                <EnergyMetricTile
                  icon="home-lightning-bolt-outline"
                  label="Load consumed"
                  value={formatKWh(dashboardQuery.data.summary.loadConsumedKwh.current)}
                  detail={
                    comparisonEnabled
                      ? formatDeltaSummary(
                          dashboardQuery.data.summary.loadConsumedKwh.delta,
                          formatKWh(Math.abs(dashboardQuery.data.summary.loadConsumedKwh.delta)),
                          dashboardQuery.data.summary.loadConsumedKwh.deltaPct,
                          dashboardQuery.data.summary.loadConsumedKwh.previous,
                          MIN_MEANINGFUL_SOLAR_COMPARISON_BASELINE_KWH
                        )
                      : 'Whole-window delivered energy to loads.'
                  }
                  accent={semantics.chartLoad}
                />
                <EnergyMetricTile
                  icon="battery-charging-high"
                  label="Battery net"
                  value={formatKWh(dashboardQuery.data.summary.batteryNetKwh.current)}
                  detail={
                    comparisonEnabled
                      ? formatDeltaSummary(
                          dashboardQuery.data.summary.batteryNetKwh.delta,
                          formatKWh(Math.abs(dashboardQuery.data.summary.batteryNetKwh.delta)),
                          dashboardQuery.data.summary.batteryNetKwh.deltaPct
                        )
                      : `End SOC ${formatSoc(dashboardQuery.data.battery.socEndPct)}`
                  }
                  accent={semantics.chartBatteryCharge}
                />
                <EnergyMetricTile
                  icon="transmission-tower"
                  label="Grid + value"
                  value={formatCurrency(
                    dashboardQuery.data.summary.estimatedValue.current,
                    dashboardQuery.data.summary.currency
                  )}
                  detail={formatKWh(dashboardQuery.data.summary.estimatedAcInputCost.current)}
                  accent={semantics.chartAc}
                />
              </YStack>

              <YStack gap="$3" flex={1} minWidth={0}>
                <ChartSection
                  title="Energy balance"
                  subtitle="Solar, grid, storage, and load over the selected local-calendar window."
                >
                  {dashboardQuery.data.currentEnergyPoints.length > 1 ? (
                    <YStack gap="$3">
                      <EnergyTrendChart
                        solar={energySeries.solar}
                        grid={energySeries.grid}
                        acOutput={energySeries.acOutput}
                        load={energySeries.load}
                        dcOutput={energySeries.dcOutput}
                        batteryCharge={energySeries.batteryCharge}
                        batteryDischarge={energySeries.batteryDischarge}
                        previousNet={comparisonEnabled ? previousEnergyNet : undefined}
                        points={dashboardQuery.data.currentEnergyPoints.length}
                        bucketSeconds={resolveEnergyBucketSeconds(routeState.preset)}
                      />
                      <XStack gap="$3" flexWrap="wrap">
                        <Stat
                          label="Points"
                          value={String(dashboardQuery.data.currentEnergyPoints.length)}
                          compact
                        />
                        {comparisonEnabled ? (
                          <Stat
                            label="Previous"
                            value={String(dashboardQuery.data.previousEnergyPoints.length)}
                            compact
                            tone="muted"
                          />
                        ) : null}
                        <Stat
                          label="Self-sufficiency"
                          value={formatPercent(dashboardQuery.data.summary.selfSufficiencyPct.current)}
                          compact
                          tone="cold"
                        />
                      </XStack>
                    </YStack>
                  ) : (
                    <Text color="$colorMuted">
                      Energy bucket history is not populated yet for this window.
                    </Text>
                  )}
                </ChartSection>

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
                  <SectionCard title="Scope details" minWidth={220}>
                    <Stat label="Resolved devices" value={String(dashboardQuery.data.scope.resolvedDeviceIds.length)} />
                    <Stat label="Timezone" value={dashboardQuery.data.window.timezone} compact />
                    <Stat label="Preset" value={energyPresetLabel(routeState.preset)} compact />
                  </SectionCard>
                  <SectionCard title="AC input cost" minWidth={220}>
                    <Stat
                      label="Current"
                      value={formatCurrency(
                        dashboardQuery.data.summary.estimatedAcInputCost.current,
                        dashboardQuery.data.summary.currency
                      )}
                    />
                    {comparisonEnabled ? (
                      <Text color="$colorMuted">
                        {formatDeltaPct(dashboardQuery.data.summary.estimatedAcInputCost.deltaPct, {
                          previousValue: dashboardQuery.data.summary.estimatedAcInputCost.previous,
                          minBaseline: MIN_MEANINGFUL_CURRENCY_BASELINE
                        })}
                      </Text>
                    ) : (
                      <Text color="$colorMuted">
                        Previous-period comparison stays available when comparison is enabled.
                      </Text>
                    )}
                  </SectionCard>
                </XStack>
              </YStack>
            </XStack>

            <XStack
              gap="$3"
              flexWrap={splitSectionWrap(isWideLayout)}
              flexDirection={isWideLayout ? 'row' : 'column'}
            >
              <YStack flex={1} minWidth={0}>
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
              </YStack>

            </XStack>

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
                              value={pvHistoryQuery.isLoading ? '…' : historyRow ? `${Math.round(displayHistoricalPvWatts(historyRow))}W` : '—'}
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
                              <Stat label="Hist max W" value={`${Math.round(displayHistoricalPvWatts(historyRow))}W`} compact />
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
