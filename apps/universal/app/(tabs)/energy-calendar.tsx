import { createElement, startTransition, useEffect, useMemo, type ComponentProps } from 'react';
import { Animated, Platform, ScrollView } from 'react-native';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useIsFetching, useQueryClient } from '@tanstack/react-query';
import { Button, Text, XStack, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { useRequireAuth } from '@/features/auth/useRequireAuth';
import { useDevices } from '@/features/devices/hooks';
import { fetchEnergyCalendar, type EnergyCalendarVisibleDay } from '@/features/energy/api';
import { buildEnergyCalendarQueryKey, useEnergyCalendar } from '@/features/energy/hooks';
import { useEnergySettingsStore } from '@/features/energy/store';
import { useCurrentUser } from '@/features/profile/hooks';
import {
  buildEnergyCalendarCachePolicy,
  buildEnergyCalendarRouteParams,
  buildEnergyRouteParams,
  formatEnergyCalendarMonthLabel,
  getTimezoneDateIso,
  resolveEnergyCalendarRouteState,
  type EnergyCalendarRouteState
} from '@/features/energy/model';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import type { ThemeSpec } from '@/shared/theme/catalog';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';
import { Card } from '@/shared/ui/Card';
import {
  resolvePageHorizontalPaddingPx,
  useNavigationShellMetrics
} from '@/shared/ui/navigationShell';
import { PulseHeroBackground } from '@/shared/ui/PulseHeroBackground';

const ENERGY_CALENDAR_WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'] as const;
const ENERGY_CALENDAR_MONTHS = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December'
] as const;
const ENERGY_CALENDAR_MONTH_SHORT = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'] as const;
const ENERGY_CALENDAR_DESKTOP_TILE_HEIGHT = 106;

type CalendarSurfaceTheme = ReturnType<typeof buildCalendarSurfaceTheme>;
type CalendarCellHoverStyle = NonNullable<ComponentProps<typeof Button>['hoverStyle']>;

function formatCurrency(amount: number, currency: string): string {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(amount);
}

function formatTotalKwh(amount: number): string {
  return `${new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1
  }).format(amount)} kWh`;
}

function formatTileKwh(amount: number, compact = false): string {
  return `${new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1
  }).format(amount)}${compact ? '' : ' kWh'}`;
}

function shiftCalendarMonth(year: number, month: number, delta: number): { year: number; month: number } {
  const date = new Date(Date.UTC(year, month - 1 + delta, 1));
  return {
    year: date.getUTCFullYear(),
    month: date.getUTCMonth() + 1
  };
}

function formatDayHeader(day: EnergyCalendarVisibleDay, compact = false): string {
  if (compact) {
    return String(day.day);
  }
  if (day.day === 1) {
    return `${ENERGY_CALENDAR_MONTH_SHORT[day.month - 1] ?? ''} 1`.trim();
  }
  return String(day.day);
}

function resolveSelectedDateLabel(state: EnergyCalendarRouteState): string {
  return formatEnergyCalendarMonthLabel(state.year, state.month, state.timezone);
}

function buildYearOptions(year: number): number[] {
  const currentYear = new Date().getFullYear();
  const start = Math.min(year, currentYear) - 4;
  const end = Math.max(year, currentYear) + 4;
  return Array.from({ length: end - start + 1 }, (_, index) => start + index);
}

function chunkCalendarWeeks(days: EnergyCalendarVisibleDay[]): EnergyCalendarVisibleDay[][] {
  const weeks: EnergyCalendarVisibleDay[][] = [];
  for (let index = 0; index < days.length; index += 7) {
    weeks.push(days.slice(index, index + 7));
  }
  return weeks;
}

function metricBarWidth(value: number, max: number): number {
  if (max <= 0 || value <= 0) {
    return 5;
  }
  return Math.max(10, Math.min(34, Math.round((value / max) * 34)));
}

function buildCalendarSurfaceTheme(
  spec: ThemeSpec,
  isDark: boolean,
  semantics: ReturnType<typeof useThemeSemantics>
) {
  const elevated = spec.colors.backgroundElevated;
  const background = spec.colors.background;
  const mutedBase = isDark ? spec.colors.colorMuted : spec.colors.borderColor;
  const tileBase = isDark ? mix(elevated, '#000000', 0.1) : mix(elevated, '#ffffff', 0.46);

  return {
    panelBackground: isDark ? withAlpha(elevated, 0.62) : withAlpha(elevated, 0.82),
    panelBorder: semantics.heroBorder,
    panelShadow: isDark ? '0 22px 70px rgba(0, 0, 0, 0.22)' : '0 24px 70px rgba(21, 45, 70, 0.12)',
    topScrim: isDark
      ? 'linear-gradient(180deg, rgba(9, 18, 30, 0.2) 0%, rgba(9, 18, 30, 0.52) 100%)'
      : 'linear-gradient(180deg, rgba(255, 255, 255, 0.18) 0%, rgba(255, 255, 255, 0.56) 100%)',
    text: spec.colors.color,
    mutedText: semantics.subtleStrongText,
    dimText: semantics.subtleText,
    accent: spec.colors.accentColor,
    selectedDot: spec.semantic.info,
    solar: spec.semantic.solar,
    money: spec.semantic.success,
    controlBackground: isDark ? withAlpha(tileBase, 0.9) : withAlpha(tileBase, 0.92),
    controlBorder: withAlpha(mutedBase, isDark ? 0.34 : 0.42),
    controlText: spec.colors.color,
    totalBackground: isDark ? withAlpha(background, 0.42) : withAlpha('#ffffff', 0.68),
    totalBorder: withAlpha(mutedBase, isDark ? 0.28 : 0.3),
    weekdayBorder: withAlpha(mutedBase, isDark ? 0.22 : 0.36),
    tileBackground: isDark ? withAlpha(tileBase, 0.62) : withAlpha(tileBase, 0.78),
    tileHoverBackground: isDark ? withAlpha(tileBase, 0.74) : withAlpha(tileBase, 0.88),
    tileMutedBackground: isDark ? withAlpha(tileBase, 0.2) : withAlpha(tileBase, 0.28),
    tileMutedHoverBackground: isDark ? withAlpha(tileBase, 0.3) : withAlpha(tileBase, 0.38),
    tileBorder: withAlpha(mutedBase, isDark ? 0.22 : 0.32),
    tileHoverShadow: isDark ? '0 16px 38px rgba(0, 0, 0, 0.26)' : '0 14px 34px rgba(26, 59, 88, 0.16)',
    unavailableText: withAlpha(spec.colors.colorMuted, isDark ? 0.62 : 0.7),
    heatLow: isDark ? mix(tileBase, spec.semantic.info, 0.22) : mix(tileBase, spec.semantic.info, 0.12),
    heatHigh: isDark ? mix(spec.semantic.success, spec.semantic.solar, 0.18) : mix(spec.semantic.success, spec.semantic.solar, 0.14),
    heatAlpha: isDark ? 0.7 : 0.38
  };
}

function heatmapBackground(
  theme: CalendarSurfaceTheme,
  intensity: number,
  hasData: boolean,
  muted: boolean,
  inactive: boolean,
  hover = false
): string {
  if (inactive || !hasData) {
    if (hover) {
      return muted ? theme.tileMutedHoverBackground : theme.tileHoverBackground;
    }
    return muted ? theme.tileMutedBackground : theme.tileBackground;
  }
  const heat = mix(theme.heatLow, theme.heatHigh, clamp(intensity, 0, 1));
  const alpha = muted ? Math.min(theme.heatAlpha, 0.38) : theme.heatAlpha;
  return withAlpha(heat, hover ? Math.min(alpha + (muted ? 0.08 : 0.12), 0.96) : alpha);
}

function calendarCellStyle({
  theme,
  active,
  selected,
  muted,
  intensity,
  hasData,
  compact
}: {
  theme: CalendarSurfaceTheme;
  active: boolean;
  selected: boolean;
  muted: boolean;
  intensity: number;
  hasData: boolean;
  compact: boolean;
}) {
  return {
    backgroundColor: heatmapBackground(theme, intensity, hasData, muted, !active),
    borderColor: selected ? theme.selectedDot : theme.tileBorder,
    opacity: muted ? 0.78 : active ? 1 : 0.62,
    minHeight: compact ? 82 : ENERGY_CALENDAR_DESKTOP_TILE_HEIGHT,
    padding: compact ? 7 : 13,
    borderRadius: compact ? 6 : 0,
    justifyContent: 'space-between' as const
  };
}

function calendarCellTransitionStyle(active: boolean) {
  if (Platform.OS !== 'web') {
    return {};
  }
  return {
    cursor: active ? 'pointer' : 'default',
    transition: 'background-color 150ms ease, box-shadow 150ms ease, opacity 150ms ease, transform 150ms ease'
  };
}

function calendarCellHoverStyle(theme: CalendarSurfaceTheme, active: boolean, backgroundColor: string) {
  if (!active) {
    return undefined;
  }
  return {
    transform: [{ translateY: -2 }],
    backgroundColor: backgroundColor as CalendarCellHoverStyle['backgroundColor'],
    shadowOpacity: 0.12,
    opacity: 1,
    ...(Platform.OS === 'web'
      ? {
          boxShadow: theme.tileHoverShadow
        }
      : {})
  };
}

function calendarCellPressStyle(active: boolean) {
  if (!active) {
    return undefined;
  }
  return {
    scale: 0.995,
    opacity: 0.95
  };
}

function CalendarSelect({
  label,
  testID,
  value,
  options,
  onValueChange,
  theme,
  minWidth = 112,
  width
}: {
  label: string;
  testID: string;
  value: string;
  options: { value: string; label: string }[];
  onValueChange: (value: string) => void;
  theme: CalendarSurfaceTheme;
  minWidth?: number;
  width?: number | string;
}) {
  if (Platform.OS === 'web') {
    return createElement(
      'select',
      {
        value,
        'aria-label': label,
        'data-testid': testID,
        onChange: (event: { target: { value: string } }) => onValueChange(event.target.value),
        style: {
          width,
          minWidth,
          minHeight: 40,
          borderRadius: 12,
          borderWidth: 1,
          borderStyle: 'solid',
          borderColor: theme.controlBorder,
          backgroundColor: theme.controlBackground,
          color: theme.controlText,
          padding: '0 34px 0 14px',
          fontSize: 14,
          fontWeight: 700,
          outline: 'none'
        }
      },
      options.map((option) =>
        createElement(
          'option',
          {
            key: option.value,
            value: option.value
          },
          option.label
        )
      )
    );
  }

  return (
    <Button
      size="$3"
      minWidth={minWidth}
      borderWidth={1}
      borderRadius={12}
      testID={testID}
      style={{
        backgroundColor: theme.controlBackground,
        borderColor: theme.controlBorder
      }}
      onPress={() => {
        const index = options.findIndex((option) => option.value === value);
        const next = options[(index + 1) % options.length];
        if (next) {
          onValueChange(next.value);
        }
      }}
    >
      <Text fontWeight="700" style={{ color: theme.controlText }}>
        {options.find((option) => option.value === value)?.label ?? value}
      </Text>
    </Button>
  );
}

function IconButton({
  testID,
  label,
  icon,
  loading,
  onPress,
  theme
}: {
  testID: string;
  label: string;
  icon: keyof typeof MaterialCommunityIcons.glyphMap;
  loading?: boolean;
  onPress: () => void;
  theme: CalendarSurfaceTheme;
}) {
  return (
    <Button
      size="$3"
      circular
      borderWidth={1}
      minWidth={42}
      minHeight={42}
      testID={testID}
      onPress={onPress}
      style={{
        backgroundColor: theme.controlBackground,
        borderColor: theme.controlBorder
      }}
      accessibilityLabel={label}
    >
      <MaterialCommunityIcons name={loading ? 'loading' : icon} size={20} color={theme.controlText} />
    </Button>
  );
}

function withAlpha(hex: string, alpha: number): string {
  const { red, green, blue } = hexToRgb(hex);
  return `rgba(${red}, ${green}, ${blue}, ${clamp(alpha, 0, 1)})`;
}

function mix(hexA: string, hexB: string, ratio: number): string {
  const a = hexToRgb(hexA);
  const b = hexToRgb(hexB);
  const clamped = clamp(ratio, 0, 1);
  return rgbToHex({
    red: Math.round(a.red + (b.red - a.red) * clamped),
    green: Math.round(a.green + (b.green - a.green) * clamped),
    blue: Math.round(a.blue + (b.blue - a.blue) * clamped)
  });
}

function hexToRgb(hex: string): { red: number; green: number; blue: number } {
  const normalized = hex.replace('#', '');
  const value =
    normalized.length === 3
      ? normalized
          .split('')
          .map((part) => part + part)
          .join('')
      : normalized;

  return {
    red: Number.parseInt(value.slice(0, 2), 16),
    green: Number.parseInt(value.slice(2, 4), 16),
    blue: Number.parseInt(value.slice(4, 6), 16)
  };
}

function rgbToHex({ red, green, blue }: { red: number; green: number; blue: number }): string {
  return `#${[red, green, blue]
    .map((value) => clamp(Math.round(value), 0, 255).toString(16).padStart(2, '0'))
    .join('')}`;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export default function EnergyCalendarScreen() {
  const router = useRouter();
  const params = useLocalSearchParams();
  const queryClient = useQueryClient();
  const semantics = useThemeSemantics();
  const { spec, isDark } = useAppTheme();
  const { contentWidth } = useNavigationShellMetrics();
  const compactCalendar = contentWidth < 720;
  const calendarTheme = useMemo(() => buildCalendarSurfaceTheme(spec, isDark, semantics), [isDark, semantics, spec]);
  const { authReady, authKey, token } = useAuthSession();
  const { allowed, waiting } = useRequireAuth();
  const gridPricePerKwhInput = useEnergySettingsStore((state) => state.gridPricePerKwh);
  const currency = useEnergySettingsStore((state) => state.currency);
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
  const fallbackTimezone = useMemo(
    () => currentUserQuery.data?.user.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
    [currentUserQuery.data?.user.timezone]
  );
  const routeState = useMemo(
    () =>
      resolveEnergyCalendarRouteState(
        params,
        devices.map((device) => device.id),
        fallbackTimezone
      ),
    [devices, fallbackTimezone, params]
  );
  const todayDateIso = useMemo(() => getTimezoneDateIso(new Date(), routeState.timezone), [routeState.timezone]);
  const currentQuery = useEnergyCalendar(
    {
      scope: routeState.scope,
      deviceId: routeState.deviceId,
      year: routeState.year,
      month: routeState.month,
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

  const previousMonth = useMemo(() => shiftCalendarMonth(routeState.year, routeState.month, -1), [routeState.month, routeState.year]);
  const nextMonth = useMemo(() => shiftCalendarMonth(routeState.year, routeState.month, 1), [routeState.month, routeState.year]);
  const previousCachePolicy = useMemo(
    () =>
      buildEnergyCalendarCachePolicy({
        year: previousMonth.year,
        month: previousMonth.month,
        timezone: routeState.timezone
      }),
    [previousMonth.month, previousMonth.year, routeState.timezone]
  );
  const nextCachePolicy = useMemo(
    () =>
      buildEnergyCalendarCachePolicy({
        year: nextMonth.year,
        month: nextMonth.month,
        timezone: routeState.timezone
      }),
    [nextMonth.month, nextMonth.year, routeState.timezone]
  );
  const previousQueryKey = useMemo(
    () =>
      buildEnergyCalendarQueryKey({
        authKey,
        scope: routeState.scope,
        deviceId: routeState.deviceId,
        year: previousMonth.year,
        month: previousMonth.month,
        timezone: routeState.timezone,
        liveDayKey: previousCachePolicy.liveDayKey,
        gridPricePerKwh: Number.parseFloat(gridPricePerKwhInput) || undefined,
        currency
      }),
    [
      authKey,
      currency,
      gridPricePerKwhInput,
      previousMonth.month,
      previousMonth.year,
      previousCachePolicy.liveDayKey,
      routeState.deviceId,
      routeState.scope,
      routeState.timezone
    ]
  );
  const nextQueryKey = useMemo(
    () =>
      buildEnergyCalendarQueryKey({
        authKey,
        scope: routeState.scope,
        deviceId: routeState.deviceId,
        year: nextMonth.year,
        month: nextMonth.month,
        timezone: routeState.timezone,
        liveDayKey: nextCachePolicy.liveDayKey,
        gridPricePerKwh: Number.parseFloat(gridPricePerKwhInput) || undefined,
        currency
      }),
    [
      authKey,
      currency,
      gridPricePerKwhInput,
      nextMonth.month,
      nextMonth.year,
      nextCachePolicy.liveDayKey,
      routeState.deviceId,
      routeState.scope,
      routeState.timezone
    ]
  );

  const previousPrefetching = useIsFetching({ queryKey: previousQueryKey, exact: true }) > 0;
  const nextPrefetching = useIsFetching({ queryKey: nextQueryKey, exact: true }) > 0;
  const shouldPrefetchPreviousMonth = previousCachePolicy.liveDayKey === null;
  const shouldPrefetchNextMonth = nextCachePolicy.liveDayKey === null;
  const monthOptions = useMemo(
    () =>
      ENERGY_CALENDAR_MONTHS.map((label, index) => ({
        value: String(index + 1),
        label
      })),
    []
  );
  const yearOptions = useMemo(
    () =>
      buildYearOptions(routeState.year).map((year) => ({
        value: String(year),
        label: String(year)
      })),
    [routeState.year]
  );
  const deviceOptions = useMemo(
    () => [
      { value: 'all', label: 'All devices' },
      ...devices.map((device) => ({
        value: device.id,
        label: device.name
      }))
    ],
    [devices]
  );

  useEffect(() => {
    if (!authReady || !allowed || (!currentQuery.data && currentQuery.isLoading)) {
      return;
    }

    const calendarArgs = {
      scope: routeState.scope,
      deviceId: routeState.deviceId,
      gridPricePerKwh: Number.parseFloat(gridPricePerKwhInput) || undefined,
      currency,
      token
    };

    if (shouldPrefetchPreviousMonth) {
      void queryClient.prefetchQuery({
        queryKey: previousQueryKey,
        queryFn: () =>
          fetchEnergyCalendar({
            ...calendarArgs,
            year: previousMonth.year,
            month: previousMonth.month
          }),
        staleTime: previousCachePolicy.staleTime,
        gcTime: previousCachePolicy.gcTime
      });
    }
    if (shouldPrefetchNextMonth) {
      void queryClient.prefetchQuery({
        queryKey: nextQueryKey,
        queryFn: () =>
          fetchEnergyCalendar({
            ...calendarArgs,
            year: nextMonth.year,
            month: nextMonth.month
          }),
        staleTime: nextCachePolicy.staleTime,
        gcTime: nextCachePolicy.gcTime
      });
    }
  }, [
    allowed,
    authReady,
    currentQuery.data,
    currentQuery.isLoading,
    currency,
    gridPricePerKwhInput,
    nextMonth.month,
    nextMonth.year,
    nextCachePolicy.gcTime,
    nextCachePolicy.staleTime,
    nextQueryKey,
    previousMonth.month,
    previousMonth.year,
    previousCachePolicy.gcTime,
    previousCachePolicy.staleTime,
    previousQueryKey,
    queryClient,
    routeState.deviceId,
    routeState.scope,
    routeState.timezone,
    shouldPrefetchNextMonth,
    shouldPrefetchPreviousMonth,
    token
  ]);

  const updateRoute = (partial: Partial<EnergyCalendarRouteState>) => {
    const nextState: EnergyCalendarRouteState = {
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
        pathname: '/(tabs)/energy-calendar',
        params: buildEnergyCalendarRouteParams(nextState)
      });
    });
  };

  const openEnergyForDate = (dateIso: string) => {
    startTransition(() => {
      router.push({
        pathname: '/(tabs)/energy',
        params: buildEnergyRouteParams({
          scope: routeState.scope,
          deviceId: routeState.deviceId,
          preset: 'today',
          timezone: routeState.timezone,
          includeComparison: true,
          panel: 'overview',
          date: dateIso
        })
      });
    });
  };

  const goToCurrentMonth = () => {
    const current = getTimezoneDateIso(new Date(), routeState.timezone);
    const currentParts = current.split('-');
    updateRoute({
      year: Number.parseInt(currentParts[0] ?? '', 10),
      month: Number.parseInt(currentParts[1] ?? '', 10)
    });
  };

  if (waiting || !allowed) {
    return <BrandedLoadingState minHeight={260} message="Checking session..." />;
  }

  if (devicesQuery.isLoading && !devices.length) {
    return <BrandedLoadingState minHeight={260} message="Loading energy scope..." />;
  }

  const monthLabel = resolveSelectedDateLabel(routeState);
  const selectedMonthTotals = currentQuery.data?.selectedMonth.totals;
  const calendarDays = currentQuery.data?.visibleDays ?? [];
  const maxSolar = Math.max(...calendarDays.map((day) => day.solarGeneratedKwh), 0);
  const maxValue = Math.max(...calendarDays.map((day) => day.estimatedValue), 0);
  const calendarWeeks = chunkCalendarWeeks(calendarDays);
  const selectedDeviceOption = routeState.scope === 'device' && routeState.deviceId ? routeState.deviceId : 'all';
  const webGridViewportStyle =
    Platform.OS === 'web' ? ({ overflowX: 'hidden', minWidth: 0 } as Record<string, string | number>) : undefined;
  const desktopSevenColumnGridStyle =
    Platform.OS === 'web' && !compactCalendar
      ? ({
          display: 'grid',
          gridTemplateColumns: 'repeat(7, minmax(0, 1fr))',
          columnGap: 0,
          rowGap: 0,
          width: '100%',
          minWidth: 0
        } as Record<string, string | number>)
      : undefined;
  const desktopCellSizingStyle =
    Platform.OS === 'web' && !compactCalendar
      ? ({
          width: '100%',
          minWidth: 0,
          maxWidth: '100%',
          boxSizing: 'border-box',
          overflow: 'hidden'
        } as Record<string, string | number>)
      : undefined;
  const desktopGridFrameStyle =
    Platform.OS === 'web' && !compactCalendar
      ? ({
          borderWidth: 1,
          borderStyle: 'solid',
          borderColor: calendarTheme.tileBorder,
          borderRadius: 8,
          overflow: 'hidden'
        } as Record<string, string | number>)
      : undefined;
  const calendarColumnWidth = Platform.OS === 'web' ? undefined : `${100 / 7}%`;
  const monthTitleSize = compactCalendar ? 36 : 48;
  const monthTitleLineHeight = compactCalendar ? 42 : 54;
  const tileIconSize = compactCalendar ? 10 : 17;
  const tileFontSize = compactCalendar ? '$1' : '$3';
  const dayHeaderFontSize = compactCalendar ? '$3' : '$4';
  const surfacePadding = compactCalendar ? 18 : 24;
  const pagePadding = resolvePageHorizontalPaddingPx(contentWidth);

  return (
    <Animated.View style={{ flex: 1 }} testID="screen-energy-calendar">
      <YStack flex={1} backgroundColor="$background">
        <ScrollView
          style={{ flex: 1 }}
          contentContainerStyle={{
            paddingHorizontal: pagePadding,
            paddingTop: compactCalendar ? 12 : 24,
            paddingBottom: compactCalendar ? 92 : 24
          }}
          showsVerticalScrollIndicator
        >
          <Card
            padding={0}
            overflow="hidden"
            position="relative"
            style={{
              backgroundColor: calendarTheme.panelBackground,
              borderColor: calendarTheme.panelBorder,
              boxShadow: calendarTheme.panelShadow
            }}
          >
            <PulseHeroBackground variant="fleet" sizes="(min-width: 1600px) 2000px, 100vw" />
            <YStack
              pointerEvents="none"
              position="absolute"
              top={0}
              right={0}
              bottom={0}
              left={0}
              zIndex={0}
              style={{ backgroundImage: calendarTheme.topScrim }}
            />
            <YStack gap={compactCalendar ? '$4' : '$5'} padding={surfacePadding} style={{ position: 'relative', zIndex: 1 }}>
              <XStack justifyContent="space-between" alignItems="flex-start" gap="$4" flexWrap="wrap">
                <Text fontSize="$7" fontWeight="800" letterSpacing={0} style={{ color: calendarTheme.text }}>
                  Energy Calendar
                </Text>
                {!compactCalendar ? (
                  <XStack alignItems="center" justifyContent="flex-end" gap="$2" flexWrap="wrap">
                    <CalendarSelect
                      label="Month"
                      testID="energy-calendar-month-select"
                      value={String(routeState.month)}
                      options={monthOptions}
                      theme={calendarTheme}
                      minWidth={132}
                      onValueChange={(value) => updateRoute({ month: Number.parseInt(value, 10) })}
                    />
                    <CalendarSelect
                      label="Year"
                      testID="energy-calendar-year-select"
                      value={String(routeState.year)}
                      options={yearOptions}
                      theme={calendarTheme}
                      minWidth={96}
                      onValueChange={(value) => updateRoute({ year: Number.parseInt(value, 10) })}
                    />
                    <CalendarSelect
                      label="Device"
                      testID="energy-calendar-device-select"
                      value={selectedDeviceOption}
                      options={deviceOptions}
                      theme={calendarTheme}
                      minWidth={160}
                      onValueChange={(value) =>
                        updateRoute(value === 'all' ? { scope: 'all' } : { scope: 'device', deviceId: value })
                      }
                    />
                  </XStack>
                ) : null}
              </XStack>

              {compactCalendar ? (
                <YStack gap="$2">
                  <XStack gap="$2" width="100%">
                    <CalendarSelect
                      label="Month"
                      testID="energy-calendar-month-select"
                      value={String(routeState.month)}
                      options={monthOptions}
                      theme={calendarTheme}
                      minWidth={0}
                      width="50%"
                      onValueChange={(value) => updateRoute({ month: Number.parseInt(value, 10) })}
                    />
                    <CalendarSelect
                      label="Year"
                      testID="energy-calendar-year-select"
                      value={String(routeState.year)}
                      options={yearOptions}
                      theme={calendarTheme}
                      minWidth={0}
                      width="50%"
                      onValueChange={(value) => updateRoute({ year: Number.parseInt(value, 10) })}
                    />
                  </XStack>
                  <CalendarSelect
                    label="Device"
                    testID="energy-calendar-device-select"
                    value={selectedDeviceOption}
                    options={deviceOptions}
                    theme={calendarTheme}
                    minWidth={0}
                    width="100%"
                    onValueChange={(value) =>
                      updateRoute(value === 'all' ? { scope: 'all' } : { scope: 'device', deviceId: value })
                    }
                  />
                </YStack>
              ) : null}

              <XStack justifyContent="space-between" alignItems="center" gap="$4" flexWrap="wrap">
                <XStack
                  alignItems="center"
                  gap="$2"
                  flexWrap={compactCalendar ? 'nowrap' : 'wrap'}
                  width={compactCalendar ? '100%' : undefined}
                  justifyContent={compactCalendar ? 'space-between' : undefined}
                >
                  <IconButton
                    testID="energy-calendar-prev-month"
                    label="Previous month"
                    icon="chevron-left"
                    loading={previousPrefetching}
                    theme={calendarTheme}
                    onPress={() => updateRoute({ ...shiftCalendarMonth(routeState.year, routeState.month, -1) })}
                  />
                  <Text
                    fontWeight="800"
                    letterSpacing={0}
                    style={{
                      color: calendarTheme.text,
                      fontSize: monthTitleSize,
                      lineHeight: monthTitleLineHeight
                    }}
                  >
                    {monthLabel}
                  </Text>
                  <IconButton
                    testID="energy-calendar-next-month"
                    label="Next month"
                    icon="chevron-right"
                    loading={nextPrefetching}
                    theme={calendarTheme}
                    onPress={() => updateRoute({ ...shiftCalendarMonth(routeState.year, routeState.month, 1) })}
                  />
                  {!compactCalendar ? (
                    <Button
                      size="$3"
                      borderWidth={1}
                      borderRadius={999}
                      minHeight={42}
                      testID="energy-calendar-today"
                      style={{
                        backgroundColor: calendarTheme.controlBackground,
                        borderColor: calendarTheme.controlBorder
                      }}
                      onPress={goToCurrentMonth}
                    >
                      <Text fontWeight="700" style={{ color: calendarTheme.controlText }}>
                        Today
                      </Text>
                    </Button>
                  ) : null}
                </XStack>

                <XStack
                  borderWidth={1}
                  borderRadius={14}
                  overflow="hidden"
                  style={{
                    backgroundColor: calendarTheme.totalBackground,
                    borderColor: calendarTheme.totalBorder
                  }}
                >
                  <XStack alignItems="center" gap="$2" paddingHorizontal="$4" paddingVertical="$3">
                    <MaterialCommunityIcons name="white-balance-sunny" size={24} color={calendarTheme.solar} />
                    <YStack>
                      <Text fontSize="$5" fontWeight="800" lineHeight={22} style={{ color: calendarTheme.text }}>
                        {selectedMonthTotals ? formatTotalKwh(selectedMonthTotals.solarGeneratedKwh) : '-'}
                      </Text>
                      <Text fontSize="$1" fontWeight="700" style={{ color: calendarTheme.mutedText }}>
                        Solar generated
                      </Text>
                    </YStack>
                  </XStack>
                  <YStack width={1} alignSelf="stretch" style={{ backgroundColor: calendarTheme.totalBorder }} />
                  <XStack alignItems="center" gap="$2" paddingHorizontal="$4" paddingVertical="$3">
                    <MaterialCommunityIcons name="currency-usd" size={24} color={calendarTheme.money} />
                    <YStack>
                      <Text fontSize="$5" fontWeight="800" lineHeight={22} style={{ color: calendarTheme.text }}>
                        {selectedMonthTotals ? formatCurrency(selectedMonthTotals.estimatedValue, selectedMonthTotals.currency) : '-'}
                      </Text>
                      <Text fontSize="$1" fontWeight="700" style={{ color: calendarTheme.mutedText }}>
                        Saved
                      </Text>
                    </YStack>
                  </XStack>
                </XStack>
              </XStack>

              {currentQuery.isLoading && !currentQuery.data ? (
                <BrandedLoadingState minHeight={220} message="Loading calendar..." />
              ) : null}

              {currentQuery.isError ? (
                <YStack
                  gap="$2"
                  borderWidth={1}
                  borderRadius={12}
                  padding="$4"
                  style={{
                    backgroundColor: calendarTheme.totalBackground,
                    borderColor: calendarTheme.totalBorder
                  }}
                >
                  <Text fontSize="$5" fontWeight="700" style={{ color: calendarTheme.text }}>
                    Calendar unavailable
                  </Text>
                  <Text style={{ color: calendarTheme.mutedText }}>{String(currentQuery.error)}</Text>
                </YStack>
              ) : null}

              {currentQuery.data ? (
                compactCalendar ? (
                  <YStack gap="$3">
                    <XStack
                      borderBottomWidth={1}
                      paddingBottom="$2"
                      style={{ borderBottomColor: calendarTheme.weekdayBorder }}
                    >
                      {ENERGY_CALENDAR_WEEKDAYS.map((weekday) => (
                        <Text
                          key={weekday}
                          width={`${100 / 7}%`}
                          textAlign="center"
                          fontSize="$2"
                          fontWeight="700"
                          style={{ color: calendarTheme.mutedText }}
                        >
                          {weekday}
                        </Text>
                      ))}
                    </XStack>

                    <YStack>
                      {calendarWeeks.map((week, weekIndex) => (
                        <XStack
                          key={week[0]?.dateIso ?? String(weekIndex)}
                          minHeight={92}
                          borderBottomWidth={weekIndex === calendarWeeks.length - 1 ? 0 : 1}
                          style={{ borderBottomColor: calendarTheme.weekdayBorder }}
                        >
                          {week.map((day) => {
                            const dayIntensity = maxSolar > 0 ? Math.max(0.14, day.solarGeneratedKwh / maxSolar) : 0.14;
                            const selected = day.isToday || day.dateIso === todayDateIso;
                            const isMuted = !day.isCurrentMonth;
                            const isInactive = day.isFuture;
                            const dayTextColor =
                              isMuted || isInactive ? calendarTheme.unavailableText : calendarTheme.text;
                            const moneyBar = metricBarWidth(day.estimatedValue, maxValue);
                            const solarBar = metricBarWidth(day.solarGeneratedKwh, maxSolar);
                            const hoverBackground = heatmapBackground(
                              calendarTheme,
                              dayIntensity,
                              day.hasData,
                              isMuted,
                              isInactive,
                              true
                            );
                            return (
                              <Button
                                key={day.dateIso}
                                width={`${100 / 7}%`}
                                minHeight={92}
                                borderWidth={0}
                                disabled={isInactive}
                                cursor={isInactive ? 'default' : 'pointer'}
                                hoverStyle={calendarCellHoverStyle(calendarTheme, !isInactive, hoverBackground)}
                                pressStyle={calendarCellPressStyle(!isInactive)}
                                testID={`energy-calendar-day-${day.dateIso}`}
                                onPress={() => openEnergyForDate(day.dateIso)}
                                style={{
                                  backgroundColor: 'transparent',
                                  borderRadius: 0,
                                  padding: 0,
                                  opacity: isMuted ? 0.62 : isInactive ? 0.52 : 1,
                                  ...calendarCellTransitionStyle(!isInactive)
                                }}
                              >
                                <YStack flex={1} alignItems="center" justifyContent="flex-start" gap="$2" paddingVertical="$2">
                                  <YStack
                                    width={selected ? 38 : 34}
                                    height={selected ? 38 : 34}
                                    borderRadius={999}
                                    alignItems="center"
                                    justifyContent="center"
                                    style={{ backgroundColor: selected ? calendarTheme.selectedDot : 'transparent' }}
                                  >
                                    <Text
                                      fontSize="$5"
                                      fontWeight="800"
                                      lineHeight={26}
                                      style={{ color: selected ? '#ffffff' : dayTextColor }}
                                    >
                                      {formatDayHeader(day, true)}
                                    </Text>
                                  </YStack>

                                  <YStack width="82%" alignItems="center" gap={4}>
                                    <YStack
                                      height={5}
                                      borderRadius={999}
                                      style={{
                                        width: moneyBar,
                                        backgroundColor: withAlpha(calendarTheme.money, day.hasData && !isInactive ? 0.78 : 0.18)
                                      }}
                                    />
                                    <YStack
                                      height={5}
                                      borderRadius={999}
                                      style={{
                                        width: solarBar,
                                        backgroundColor: withAlpha(calendarTheme.solar, day.hasData && !isInactive ? 0.78 : 0.18)
                                      }}
                                    />
                                  </YStack>
                                </YStack>
                              </Button>
                            );
                          })}
                        </XStack>
                      ))}
                    </YStack>

                    <Button
                      size="$3"
                      borderWidth={1}
                      borderRadius={999}
                      alignSelf="flex-start"
                      minHeight={40}
                      testID="energy-calendar-today"
                      style={{
                        backgroundColor: calendarTheme.controlBackground,
                        borderColor: calendarTheme.controlBorder
                      }}
                      onPress={goToCurrentMonth}
                    >
                      <Text fontWeight="700" style={{ color: calendarTheme.controlText }}>
                        Today
                      </Text>
                    </Button>
                  </YStack>
                ) : (
                <YStack width="100%" style={webGridViewportStyle}>
                  <YStack width="100%" minWidth={0}>
                    <XStack
                      gap={0}
                      borderBottomWidth={1}
                      paddingBottom="$2"
                      flexWrap={Platform.OS === 'web' ? undefined : 'wrap'}
                      style={{ borderBottomColor: calendarTheme.weekdayBorder, ...desktopSevenColumnGridStyle }}
                    >
                      {ENERGY_CALENDAR_WEEKDAYS.map((weekday) => (
                        <Text
                          key={weekday}
                          width={calendarColumnWidth}
                          textAlign="center"
                          fontSize={dayHeaderFontSize}
                          fontWeight="600"
                          style={{ color: calendarTheme.mutedText, ...desktopCellSizingStyle }}
                        >
                          {weekday}
                        </Text>
                      ))}
                    </XStack>

                    <YStack width="100%" style={desktopGridFrameStyle}>
                    <XStack gap={0} flexWrap={Platform.OS === 'web' ? undefined : 'wrap'} style={desktopSevenColumnGridStyle}>
                      {calendarDays.map((day, dayIndex) => {
                        const dayIntensity = maxSolar > 0 ? Math.max(0.14, day.solarGeneratedKwh / maxSolar) : 0.14;
                        const selected = day.isToday || day.dateIso === todayDateIso;
                        const isMuted = !day.isCurrentMonth;
                        const isInactive = day.isFuture;
                        const rightEdge = (dayIndex + 1) % 7 === 0;
                        const bottomEdge = dayIndex >= calendarDays.length - 7;
                        const hoverBackground = heatmapBackground(
                          calendarTheme,
                          dayIntensity,
                          day.hasData,
                          isMuted || isInactive,
                          isInactive,
                          true
                        );
                        const dayTextColor = selected
                          ? calendarTheme.text
                          : isMuted || isInactive
                            ? calendarTheme.unavailableText
                            : calendarTheme.text;
                        return (
                          <Button
                            key={day.dateIso}
                            width={calendarColumnWidth}
                            borderWidth={0}
                            disabled={isInactive}
                            cursor={isInactive ? 'default' : 'pointer'}
                            hoverStyle={calendarCellHoverStyle(calendarTheme, !isInactive, hoverBackground)}
                            pressStyle={calendarCellPressStyle(!isInactive)}
                            testID={`energy-calendar-day-${day.dateIso}`}
                            onPress={() => openEnergyForDate(day.dateIso)}
                            unstyled
                            style={{
                              ...calendarCellStyle({
                                theme: calendarTheme,
                                active: !isInactive,
                                selected,
                                muted: isMuted || isInactive,
                                intensity: dayIntensity,
                                hasData: day.hasData,
                                compact: compactCalendar
                              }),
                              ...desktopCellSizingStyle,
                              borderRightWidth: rightEdge ? 0 : 1,
                              borderBottomWidth: bottomEdge ? 0 : 1,
                              borderStyle: 'solid',
                              boxShadow: selected ? `inset 0 0 0 1px ${calendarTheme.selectedDot}` : 'none',
                              ...calendarCellTransitionStyle(!isInactive)
                            }}
                          >
                            <YStack flex={1} minWidth={0} gap="$3" justifyContent="space-between">
                              <XStack alignItems="center" justifyContent="space-between" gap="$2">
                                <Text fontSize="$5" fontWeight="800" style={{ color: dayTextColor }}>
                                  {formatDayHeader(day, false)}
                                </Text>
                                <XStack alignItems="center" gap="$1">
                                  {isMuted ? (
                                    <MaterialCommunityIcons name="progress-clock" size={compactCalendar ? 10 : 13} color={calendarTheme.dimText} />
                                  ) : null}
                                  {selected ? (
                                    <YStack
                                      width={8}
                                      height={8}
                                      borderRadius={999}
                                      style={{ backgroundColor: calendarTheme.selectedDot }}
                                    />
                                  ) : null}
                                </XStack>
                              </XStack>

                              <YStack gap="$2" minWidth={0}>
                                <XStack alignItems="center" gap="$1">
                                  <MaterialCommunityIcons name="currency-usd" size={tileIconSize} color={calendarTheme.money} />
                                  <Text fontSize={tileFontSize} fontWeight="800" style={{ color: dayTextColor }} numberOfLines={1}>
                                    {formatCurrency(day.estimatedValue, day.currency)}
                                  </Text>
                                </XStack>
                                <XStack alignItems="center" gap="$1">
                                  <MaterialCommunityIcons
                                    name="white-balance-sunny"
                                    size={tileIconSize}
                                    color={calendarTheme.solar}
                                  />
                                  <Text fontSize={tileFontSize} fontWeight="800" style={{ color: dayTextColor }} numberOfLines={1}>
                                    {formatTileKwh(day.solarGeneratedKwh)}
                                  </Text>
                                </XStack>
                              </YStack>
                            </YStack>
                          </Button>
                        );
                      })}
                    </XStack>
                    </YStack>
                  </YStack>
                </YStack>
                )
              ) : null}
            </YStack>
          </Card>
        </ScrollView>
      </YStack>
    </Animated.View>
  );
}
