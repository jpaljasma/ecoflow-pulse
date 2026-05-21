import type { DeviceSummary } from '@/features/devices/api';
import type { EnergyDashboard, EnergyPreset, EnergyRollupPoint } from '@/features/energy/api';
import { repairPowerTrendDropouts } from '@/features/history/powerTrend';
import { MIN_MEANINGFUL_SOLAR_COMPARISON_BASELINE_WH } from '@/shared/ui/solarLegend';

export const ENERGY_PRESETS: EnergyPreset[] = [
  'today',
  'yesterday',
  'past24h',
  'last7d',
  'last30d',
  'thisWeek',
  'previousWeek',
  'thisMonth',
  'lastMonth',
  'last12m'
];

export const MIN_MEANINGFUL_CURRENCY_BASELINE = 0.01;
export const MIN_MEANINGFUL_SOLAR_COMPARISON_BASELINE_KWH = MIN_MEANINGFUL_SOLAR_COMPARISON_BASELINE_WH / 1000;
export const ENERGY_CALENDAR_LIVE_STALE_MS = 30_000;
export const ENERGY_CALENDAR_LIVE_GC_MS = 10 * 60_000;
export const ENERGY_CALENDAR_LIVE_REFETCH_MS = 60_000;
export const ENERGY_CALENDAR_HISTORICAL_STALE_MS = 12 * 60 * 60_000;
export const ENERGY_CALENDAR_HISTORICAL_GC_MS = 24 * 60 * 60_000;
export const ENERGY_DASHBOARD_LIVE_STALE_MS = 60_000;
export const ENERGY_DASHBOARD_HISTORICAL_STALE_MS = 30 * 60_000;
export const ENERGY_DASHBOARD_GC_MS = 60 * 60_000;

export const ENERGY_PANELS = ['overview', 'solar', 'impact'] as const;

export type EnergyPanel = (typeof ENERGY_PANELS)[number];

export type EnergyRouteState = {
  scope: 'device' | 'all';
  deviceId?: string;
  preset: EnergyPreset;
  timezone: string;
  includeComparison: boolean;
  panel: EnergyPanel;
  date?: string;
};

export type EnergyCalendarRouteState = {
  scope: 'device' | 'all';
  deviceId?: string;
  year: number;
  month: number;
  timezone: string;
  gridPricePerKwh?: number;
  currency?: string;
};

export function detectLocalTimezone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
}

export function resolveEnergyRouteState(
  params: Record<string, string | string[] | undefined>,
  availableDeviceIds: string[] = [],
  fallbackTimezone = detectLocalTimezone(),
  now = new Date()
): EnergyRouteState {
  void now;
  const requestedDevice = normalizeScalar(params.device);
  const requestedScope = normalizeScalar(params.scope);
  const requestedDeviceId = requestedDevice && requestedDevice !== 'all' ? requestedDevice : normalizeScalar(params.deviceId);
  const requestedPreset = normalizeScalar(params.preset);
  const compareParam = normalizeScalar(params.compare);
  const requestedPanel = normalizeScalar(params.panel);
  const requestedDate = normalizeCalendarDateIso(normalizeScalar(params.date));
  const includeComparison =
    compareParam !== undefined
      ? parseBooleanParam(compareParam === '1' ? 'true' : compareParam === '0' ? 'false' : compareParam, true)
      : parseBooleanParam(normalizeScalar(params.includeComparison), true);

  let scope: 'device' | 'all' =
    requestedDevice === 'all'
      ? 'all'
      : requestedScope === 'device' || requestedScope === 'all'
        ? requestedScope
        : requestedDeviceId
          ? 'device'
          : 'all';

  let deviceId = requestedDeviceId && availableDeviceIds.includes(requestedDeviceId) ? requestedDeviceId : undefined;

  if (scope === 'device' && !deviceId && availableDeviceIds.length > 0) {
    deviceId = availableDeviceIds[0];
  }
  if (scope === 'device' && !deviceId) {
    scope = 'all';
  }

  return {
    scope,
    deviceId: scope === 'device' ? deviceId : undefined,
    preset: ENERGY_PRESETS.includes(requestedPreset as EnergyPreset) ? (requestedPreset as EnergyPreset) : 'today',
    timezone: fallbackTimezone || 'UTC',
    includeComparison,
    panel: ENERGY_PANELS.includes(requestedPanel as EnergyPanel) ? (requestedPanel as EnergyPanel) : 'overview',
    ...(requestedDate ? { date: requestedDate } : {})
  };
}

export function buildEnergyRouteParams(state: EnergyRouteState): Record<string, string> {
  const params: Record<string, string> = {
    device: state.scope === 'device' && state.deviceId ? state.deviceId : 'all',
    preset: state.preset,
    compare: state.includeComparison ? '1' : '0',
    panel: state.panel
  };
  if (state.date) {
    params.date = state.date;
  }
  return params;
}

export function resolveEnergyCalendarRouteState(
  params: Record<string, string | string[] | undefined>,
  availableDeviceIds: string[] = [],
  fallbackTimezone = detectLocalTimezone(),
  now = new Date()
): EnergyCalendarRouteState {
  const requestedScope = normalizeScalar(params.scope);
  const requestedDevice = normalizeScalar(params.device);
  const requestedDeviceId =
    normalizeScalar(params.deviceId) || (requestedDevice && requestedDevice !== 'all' ? requestedDevice : undefined);
  const requestedYear = Number.parseInt(normalizeScalar(params.year) ?? '', 10);
  const requestedMonth = Number.parseInt(normalizeScalar(params.month) ?? '', 10);
  const gridPricePerKwh = normalizeNumericParam(normalizeScalar(params.gridPricePerKwh));
  const currency = normalizeScalar(params.currency) || undefined;

  let scope: 'device' | 'all' =
    requestedScope === 'device' || requestedScope === 'all' ? requestedScope : requestedDeviceId ? 'device' : 'all';
  let deviceId = requestedDeviceId && availableDeviceIds.includes(requestedDeviceId) ? requestedDeviceId : undefined;

  if (scope === 'device' && !deviceId && availableDeviceIds.length > 0) {
    deviceId = availableDeviceIds[0];
  }
  if (scope === 'device' && !deviceId) {
    scope = 'all';
  }

  const fallbackMonth = getTimezoneMonthParts(now, fallbackTimezone || 'UTC');
  const year = Number.isFinite(requestedYear) && requestedYear > 0 ? requestedYear : fallbackMonth.year;
  const month = Number.isFinite(requestedMonth) && requestedMonth >= 1 && requestedMonth <= 12 ? requestedMonth : fallbackMonth.month;

  return {
    scope,
    deviceId: scope === 'device' ? deviceId : undefined,
    year,
    month,
    timezone: fallbackTimezone || 'UTC',
    ...(gridPricePerKwh !== undefined ? { gridPricePerKwh } : {}),
    ...(currency ? { currency } : {})
  };
}

export function buildEnergyCalendarRouteParams(state: EnergyCalendarRouteState): Record<string, string> {
  const params: Record<string, string> = {
    scope: state.scope,
    year: String(state.year),
    month: String(state.month)
  };
  if (state.scope === 'device' && state.deviceId) {
    params.deviceId = state.deviceId;
  }
  if (state.gridPricePerKwh !== undefined && Number.isFinite(state.gridPricePerKwh)) {
    params.gridPricePerKwh = String(state.gridPricePerKwh);
  }
  if (state.currency) {
    params.currency = state.currency;
  }
  return params;
}

export function energyPanelLabel(panel: EnergyPanel): string {
  switch (panel) {
    case 'overview':
      return 'Overview';
    case 'solar':
      return 'Solar';
    case 'impact':
      return 'Impact';
  }
}

export function energyPresetLabel(preset: EnergyPreset): string {
  switch (preset) {
    case 'today':
      return 'Today';
    case 'past24h':
      return 'Last 24h';
    case 'yesterday':
      return 'Yesterday';
    case 'last7d':
      return 'Last 7d';
    case 'last30d':
      return 'Last 30d';
    case 'thisWeek':
      return 'This week';
    case 'previousWeek':
      return 'Previous week';
    case 'thisMonth':
      return 'This month';
    case 'lastMonth':
      return 'Last month';
    case 'last12m':
      return 'Last 12 months';
  }
}

export function formatEnergyCalendarMonthLabel(year: number, month: number, timezone: string): string {
  const firstOfMonth = new Date(Date.UTC(year, month - 1, 1, 12));
  try {
    return new Intl.DateTimeFormat(undefined, {
      month: 'long',
      year: 'numeric',
      timeZone: timezone
    }).format(firstOfMonth);
  } catch {
    return new Intl.DateTimeFormat(undefined, {
      month: 'long',
      year: 'numeric'
    }).format(firstOfMonth);
  }
}

export function getTimezoneMonthParts(date: Date, timezone: string): { year: number; month: number } {
  try {
    const parts = new Intl.DateTimeFormat('en-CA', {
      timeZone: timezone,
      year: 'numeric',
      month: '2-digit'
    }).formatToParts(date);
    const year = Number(parts.find((part) => part.type === 'year')?.value);
    const month = Number(parts.find((part) => part.type === 'month')?.value);
    if (Number.isFinite(year) && Number.isFinite(month)) {
      return { year, month };
    }
  } catch {
    // Fall through to UTC fallback below.
  }
  return { year: date.getUTCFullYear(), month: date.getUTCMonth() + 1 };
}

export function getTimezoneDateIso(date: Date, timezone: string): string {
  try {
    const parts = new Intl.DateTimeFormat('en-CA', {
      timeZone: timezone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit'
    }).formatToParts(date);
    const year = parts.find((part) => part.type === 'year')?.value;
    const month = parts.find((part) => part.type === 'month')?.value;
    const day = parts.find((part) => part.type === 'day')?.value;
    if (year && month && day) {
      return `${year}-${month}-${day}`;
    }
  } catch {
    // Fall through to UTC fallback below.
  }
  return date.toISOString().slice(0, 10);
}

export function msUntilNextTimezoneDate(date: Date, timezone: string): number {
  const startMs = date.getTime();
  const currentDateIso = getTimezoneDateIso(date, timezone);
  let lowMs = startMs;
  let highMs = startMs + 36 * 60 * 60_000;

  while (getTimezoneDateIso(new Date(highMs), timezone) === currentDateIso) {
    highMs += 12 * 60 * 60_000;
    if (highMs - startMs > 72 * 60 * 60_000) {
      return 24 * 60 * 60_000;
    }
  }

  while (highMs - lowMs > 1) {
    const midMs = Math.floor((lowMs + highMs) / 2);
    if (getTimezoneDateIso(new Date(midMs), timezone) === currentDateIso) {
      lowMs = midMs;
    } else {
      highMs = midMs;
    }
  }

  return Math.max(1, highMs - startMs);
}

export function buildEnergyCalendarCachePolicy({
  year,
  month,
  timezone,
  now = new Date()
}: {
  year: number;
  month: number;
  timezone: string;
  now?: Date;
}): {
  liveDayKey: string | null;
  staleTime: number;
  gcTime: number;
  refetchInterval: number | false;
  midnightRefreshMs: number | null;
} {
  const currentMonth = getTimezoneMonthParts(now, timezone);
  const isLiveMonth = currentMonth.year === year && currentMonth.month === month;

  if (!isLiveMonth) {
    return {
      liveDayKey: null,
      staleTime: ENERGY_CALENDAR_HISTORICAL_STALE_MS,
      gcTime: ENERGY_CALENDAR_HISTORICAL_GC_MS,
      refetchInterval: false,
      midnightRefreshMs: msUntilNextTimezoneDate(now, timezone)
    };
  }

  return {
    liveDayKey: getTimezoneDateIso(now, timezone),
    staleTime: ENERGY_CALENDAR_LIVE_STALE_MS,
    gcTime: ENERGY_CALENDAR_LIVE_GC_MS,
    refetchInterval: ENERGY_CALENDAR_LIVE_REFETCH_MS,
    midnightRefreshMs: msUntilNextTimezoneDate(now, timezone)
  };
}

export function buildEnergyDashboardCachePolicy({
  preset,
  date,
  timezone,
  now = new Date()
}: {
  preset: EnergyPreset;
  date?: string;
  timezone: string;
  now?: Date;
}): { staleTime: number; gcTime: number; refetchInterval: number | false } {
  const todayIso = getTimezoneDateIso(now, timezone);
  const isLiveWindow =
    preset === 'past24h' ||
    preset === 'thisWeek' ||
    preset === 'thisMonth' ||
    (preset === 'today' && (!date || date === todayIso));

  return {
    staleTime: isLiveWindow ? ENERGY_DASHBOARD_LIVE_STALE_MS : ENERGY_DASHBOARD_HISTORICAL_STALE_MS,
    gcTime: ENERGY_DASHBOARD_GC_MS,
    refetchInterval: isLiveWindow ? ENERGY_DASHBOARD_LIVE_STALE_MS : false
  };
}

export function buildPowerTrendSeries(points: EnergyRollupPoint[]): {
  solar: number[];
  ac: number[];
  dc: number[];
  load: number[];
  battery: number[];
} {
  const repaired = repairPowerTrendDropouts({
    solar: points.map((point) => point.metrics.pvAvgW),
    ac: points.map((point) => point.metrics.acInAvgW),
    dc: points.map((point) => point.metrics.dcAvgW),
    load: points.map((point) => point.metrics.loadAvgW)
  });

  return {
    ...repaired,
    battery: points.map((point) => point.metrics.batteryAvgW)
  };
}

export function buildEnergyTrendSeries(points: EnergyRollupPoint[]): {
  solar: number[];
  grid: number[];
  acOutput: number[];
  load: number[];
  dcOutput: number[];
  batteryCharge: number[];
  batteryDischarge: number[];
} {
  return {
    solar: points.map((point) => point.metrics.solarGeneratedWh / 1000),
    grid: points.map((point) => point.metrics.acInputEnergyWh / 1000),
    acOutput: points.map((point) => point.metrics.acOutputEnergyWh / 1000),
    load: points.map((point) => point.metrics.loadEnergyWh / 1000),
    dcOutput: points.map((point) => point.metrics.dcOutputEnergyWh / 1000),
    batteryCharge: points.map((point) => point.metrics.batteryChargeEnergyWh / 1000),
    batteryDischarge: points.map((point) => point.metrics.batteryDischargeEnergyWh / 1000)
  };
}

export function buildWindowLabel(dashboard: EnergyDashboard): string {
  const from = new Date(Number.parseInt(dashboard.window.fromUnixMs, 10));
  const to = new Date(Number.parseInt(dashboard.window.toUnixMs, 10));
  const formatter = new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric'
  });
  const fromLabel = formatter.format(from);
  const toLabel = formatter.format(to);
  return fromLabel === toLabel ? fromLabel : `${fromLabel} - ${toLabel}`;
}

export function formatDeltaPct(
  deltaPct: number | null,
  options?: {
    previousValue?: number | null;
    minBaseline?: number;
  }
): string {
  if (deltaPct === null || !Number.isFinite(deltaPct)) {
    return 'No prior baseline';
  }
  const previousValue = options?.previousValue;
  const minBaseline = options?.minBaseline ?? 0;
  if (previousValue !== null && previousValue !== undefined && Number.isFinite(previousValue)) {
    const absolutePrevious = Math.abs(previousValue);
    if (absolutePrevious <= 0) {
      return 'No prior baseline';
    }
    if (minBaseline > 0 && absolutePrevious < minBaseline) {
      return 'vs previous';
    }
  }
  const rounded = Math.round(deltaPct * 10) / 10;
  const sign = rounded > 0 ? '+' : '';
  return `${sign}${rounded}% vs previous`;
}

export type EnergyInsightCard = {
  title: string;
  body: string;
};

export type PvEnvelopeRow = {
  deviceId: string;
  deviceName: string;
  portId: string;
  portLabel: string;
  observedVolts: number;
  observedAmps: number;
  observedPower: number;
  maxVolts: number;
  maxAmps: number;
  maxPower: number;
  powerUtilizationPct: number | null;
  voltageHeadroom: number | null;
  currentHeadroom: number | null;
  bottleneckHint: string;
};

export type PvEnvelopeSummary = {
  observedPower: number;
  configuredPower: number;
  observedVolts: number;
  configuredVolts: number;
  observedAmps: number;
  configuredAmps: number;
  utilizationPct: number | null;
  topDevicePeakLabel: string | null;
  rows: PvEnvelopeRow[];
};

export function normalizePvObservedPower(observedPower: number | null | undefined): number {
  if (observedPower === null || observedPower === undefined || !Number.isFinite(observedPower)) {
    return 0;
  }
  return Math.max(0, observedPower);
}

export function formatEnergyDateLabel(dateIso: string): string {
  const parts = parseCalendarDateIso(dateIso);
  if (!parts) {
    return dateIso;
  }
  const date = new Date(Date.UTC(parts.year, parts.month - 1, parts.day, 12, 0, 0));
  try {
    return new Intl.DateTimeFormat(undefined, {
      weekday: 'short',
      month: 'short',
      day: 'numeric',
      timeZone: 'UTC'
    }).format(date);
  } catch {
    return dateIso;
  }
}

export function buildEnergyInsights(
  points: EnergyRollupPoint[],
  timezone: string,
  preset: EnergyPreset,
  pvRows: PvEnvelopeRow[] = []
): EnergyInsightCard[] {
  const bestSolar = findBestBucket(points, 'pvAvgW');
  const bestLoad = findBestBucket(points, 'loadAvgW');
  const loadValues = points.map((point) => point.metrics.loadAvgW).filter((value) => Number.isFinite(value));
  const sortedLoads = [...loadValues].sort((left, right) => left - right);
  const baseLoad = quantile(sortedLoads, 0.1);
  const spikeLoad = quantile(sortedLoads, 0.95);
  const spikeFactor = baseLoad !== null && spikeLoad !== null && baseLoad > 0 ? spikeLoad / baseLoad : null;
  const clippingInsight = buildClippingInsight(pvRows);

  return [
    {
      title: 'Best solar period',
      body: bestSolar
        ? `${bestSolar.label} peaked near ${Math.round(bestSolar.value)}W solar average.`
        : 'Waiting for enough power history to identify the best solar period.'
    },
    {
      title: 'Best load period',
      body: bestLoad
        ? `${bestLoad.label} hit the strongest load at about ${Math.round(bestLoad.value)}W.`
        : 'Waiting for enough power history to identify the strongest load period.'
    },
    {
      title: 'Base load vs spikes',
      body:
        baseLoad !== null && spikeLoad !== null
          ? `Base load ~${Math.round(baseLoad)}W; spikes to ~${Math.round(spikeLoad)}W${spikeFactor ? ` (${spikeFactor.toFixed(1)}x)` : ''}.`
          : 'Not enough power buckets yet to estimate base load versus spikes.'
    },
    {
      title: 'Comparison status',
      body:
        points.length > 0
          ? `Using ${presetLabelForInsight(preset)} buckets in ${timezone} for current-period diagnostics.`
          : 'Current-period diagnostics will appear here once the selected window returns history.'
    },
    {
      title: 'Likely clipping / bottlenecks',
      body: clippingInsight
    }
  ];
}

type CalendarDateParts = {
  year: number;
  month: number;
  day: number;
};

function parseCalendarDateIso(input: string | undefined | null): CalendarDateParts | null {
  if (!input || !/^\d{4}-\d{2}-\d{2}$/.test(input)) {
    return null;
  }
  const [yearText, monthText, dayText] = input.split('-');
  const year = Number.parseInt(yearText ?? '', 10);
  const month = Number.parseInt(monthText ?? '', 10);
  const day = Number.parseInt(dayText ?? '', 10);
  if (!Number.isFinite(year) || !Number.isFinite(month) || !Number.isFinite(day)) {
    return null;
  }
  const date = new Date(Date.UTC(year, month - 1, day));
  if (date.getUTCFullYear() !== year || date.getUTCMonth() + 1 !== month || date.getUTCDate() !== day) {
    return null;
  }
  return { year, month, day };
}

function normalizeCalendarDateIso(input: string | undefined | null): string | undefined {
  if (!parseCalendarDateIso(input)) {
    return undefined;
  }
  return input ?? undefined;
}

function normalizeNumericParam(value: string | undefined): number | undefined {
  if (value === undefined) {
    return undefined;
  }
  const parsed = Number.parseFloat(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}

export function buildPvEnvelopeSummary(devices: DeviceSummary[], scope: 'device' | 'all', deviceId?: string): PvEnvelopeSummary {
  const scopedDevices = scope === 'device' && deviceId ? devices.filter((device) => device.id === deviceId) : devices;

  const rows: PvEnvelopeRow[] = [];
  for (const device of scopedDevices) {
    for (const port of device.details?.solarPorts ?? []) {
      const maxPower = Math.max(0, port.maxWatts ?? 0);
      const observedPower = normalizePvObservedPower(port.watts);
      const powerUtilizationPct = maxPower > 0 && port.watts !== undefined ? Math.max(0, (observedPower / maxPower) * 100) : null;
      const voltageHeadroom = port.maxVolts !== undefined && port.volts !== undefined ? port.maxVolts - port.volts : null;
      const currentHeadroom = port.maxAmps !== undefined && port.amps !== undefined ? port.maxAmps - port.amps : null;
      rows.push({
        deviceId: device.id,
        deviceName: device.name,
        portId: port.id,
        portLabel: port.name,
        observedVolts: Math.max(0, port.volts ?? 0),
        observedAmps: Math.max(0, port.amps ?? 0),
        observedPower,
        maxVolts: Math.max(0, port.maxVolts ?? 0),
        maxAmps: Math.max(0, port.maxAmps ?? 0),
        maxPower,
        powerUtilizationPct,
        voltageHeadroom,
        currentHeadroom,
        bottleneckHint: buildBottleneckHint({
          observedPower,
          maxPower,
          observedAmps: Math.max(0, port.amps ?? 0),
          maxAmps: Math.max(0, port.maxAmps ?? 0),
          observedVolts: Math.max(0, port.volts ?? 0),
          maxVolts: Math.max(0, port.maxVolts ?? 0)
        })
      });
    }
  }

  const observedPower = rows.reduce((sum, row) => sum + row.observedPower, 0);
  const configuredPower = rows.reduce((sum, row) => sum + row.maxPower, 0);
  const observedVolts = rows.reduce((max, row) => Math.max(max, row.observedVolts), 0);
  const configuredVolts = rows.reduce((max, row) => Math.max(max, row.maxVolts), 0);
  const observedAmps = rows.reduce((max, row) => Math.max(max, row.observedAmps), 0);
  const configuredAmps = rows.reduce((max, row) => Math.max(max, row.maxAmps), 0);
  const perDevicePower = new Map<string, { name: string; power: number }>();
  for (const row of rows) {
    const current = perDevicePower.get(row.deviceId);
    perDevicePower.set(row.deviceId, {
      name: row.deviceName,
      power: (current?.power ?? 0) + row.observedPower
    });
  }
  const topDevice = [...perDevicePower.values()].sort((left, right) => right.power - left.power)[0];

  return {
    observedPower,
    configuredPower,
    observedVolts,
    configuredVolts,
    observedAmps,
    configuredAmps,
    utilizationPct: configuredPower > 0 ? Math.max(0, (observedPower / configuredPower) * 100) : null,
    topDevicePeakLabel: topDevice ? `${topDevice.name} · ${Math.round(topDevice.power)}W` : null,
    rows
  };
}

function normalizeScalar(value: string | string[] | undefined): string | undefined {
  if (Array.isArray(value)) {
    return value[0];
  }
  return value;
}

function parseBooleanParam(value: string | undefined, fallback: boolean): boolean {
  if (!value) return fallback;
  return value === 'true' || value === '1';
}

function presetLabelForInsight(preset: EnergyPreset): string {
  switch (preset) {
    case 'today':
    case 'past24h':
    case 'yesterday':
      return 'minute';
    case 'last7d':
    case 'last30d':
    case 'thisWeek':
    case 'previousWeek':
    case 'thisMonth':
      return 'hour';
    case 'lastMonth':
    case 'last12m':
      return 'day';
  }
}

function findBestBucket(points: EnergyRollupPoint[], metric: 'pvAvgW' | 'loadAvgW'): { label: string; value: number } | null {
  let bestPoint: EnergyRollupPoint | null = null;
  let bestValue = -1;
  for (const point of points) {
    const value = point.metrics[metric];
    if (!Number.isFinite(value) || value <= bestValue) {
      continue;
    }
    bestValue = value;
    bestPoint = point;
  }
  if (!bestPoint) {
    return null;
  }
  return {
    label: formatBucketLabel(bestPoint.bucketStartUnixMs, bestPoint.bucketEndUnixMs),
    value: bestValue
  };
}

function formatBucketLabel(startUnixMs: string, endUnixMs: string): string {
  const start = new Date(Number.parseInt(startUnixMs, 10));
  const end = new Date(Number.parseInt(endUnixMs, 10));
  const formatter = new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit'
  });
  return `${formatter.format(start)} - ${formatter.format(end)}`;
}

function quantile(values: number[], ratio: number): number | null {
  if (values.length === 0) {
    return null;
  }
  const index = Math.max(0, Math.min(values.length - 1, Math.floor((values.length - 1) * ratio)));
  return values[index] ?? null;
}

function buildBottleneckHint({
  observedPower,
  maxPower,
  observedAmps,
  maxAmps,
  observedVolts,
  maxVolts
}: {
  observedPower: number;
  maxPower: number;
  observedAmps: number;
  maxAmps: number;
  observedVolts: number;
  maxVolts: number;
}): string {
  if (maxPower > 0 && observedPower > maxPower) {
    return 'Above configured power';
  }
  if (maxPower > 0 && observedPower >= maxPower * 0.97) {
    return 'Near power ceiling';
  }
  if (maxAmps > 0 && observedAmps >= maxAmps * 0.97 && maxVolts > 0 && observedVolts < maxVolts * 0.9) {
    return 'Likely current-limited';
  }
  if (maxVolts > 0 && observedVolts >= maxVolts * 0.97) {
    return 'Near voltage ceiling';
  }
  return 'Within envelope';
}

function buildClippingInsight(rows: PvEnvelopeRow[]): string {
  const constrainedRows = rows.filter((row) => row.bottleneckHint !== 'Within envelope');
  if (constrainedRows.length === 0) {
    const topRow = [...rows].sort((left, right) => right.observedPower - left.observedPower)[0];
    if (!topRow) {
      return 'PV envelope diagnostics will appear here once the selected scope reports port capability data.';
    }
    return `${formatPvRowLabel(topRow)} stayed within its configured envelope in the selected window.`;
  }

  const prioritizedRows = [...constrainedRows].sort((left, right) => {
    const leftUtilization = left.powerUtilizationPct ?? 0;
    const rightUtilization = right.powerUtilizationPct ?? 0;
    return rightUtilization - leftUtilization;
  });
  const topRows = prioritizedRows.slice(0, 2);

  return topRows.map((row) => `${formatPvRowLabel(row)} is ${row.bottleneckHint.toLowerCase()}.`).join(' ');
}

function formatPvRowLabel(row: PvEnvelopeRow): string {
  return row.deviceName ? `${row.deviceName} ${row.portLabel}` : row.portLabel;
}
