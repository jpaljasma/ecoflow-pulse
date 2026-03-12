import type { DeviceSummary } from '@/features/devices/api';
import type { EnergyDashboard, EnergyPreset, EnergyRollupPoint } from '@/features/energy/api';

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

export type EnergyRouteState = {
  scope: 'device' | 'all';
  deviceId?: string;
  preset: EnergyPreset;
  timezone: string;
  includeComparison: boolean;
};

export function detectLocalTimezone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
}

export function resolveEnergyRouteState(
  params: Record<string, string | string[] | undefined>,
  availableDeviceIds: string[] = [],
  fallbackTimezone = detectLocalTimezone()
): EnergyRouteState {
  const requestedDevice = normalizeScalar(params.device);
  const requestedScope = normalizeScalar(params.scope);
  const requestedDeviceId = requestedDevice && requestedDevice !== 'all' ? requestedDevice : normalizeScalar(params.deviceId);
  const requestedPreset = normalizeScalar(params.preset);
  const requestedTimezone = normalizeScalar(params.tz) || normalizeScalar(params.timezone);
  const compareParam = normalizeScalar(params.compare);
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

  let deviceId =
    requestedDeviceId && availableDeviceIds.includes(requestedDeviceId)
      ? requestedDeviceId
      : undefined;

  if (scope === 'device' && !deviceId && availableDeviceIds.length > 0) {
    deviceId = availableDeviceIds[0];
  }
  if (scope === 'device' && !deviceId) {
    scope = 'all';
  }

  return {
    scope,
    deviceId: scope === 'device' ? deviceId : undefined,
    preset: ENERGY_PRESETS.includes(requestedPreset as EnergyPreset)
      ? (requestedPreset as EnergyPreset)
      : 'today',
    timezone: requestedTimezone || fallbackTimezone || 'UTC',
    includeComparison
  };
}

export function buildEnergyRouteParams(state: EnergyRouteState): Record<string, string> {
  return {
    device: state.scope === 'device' && state.deviceId ? state.deviceId : 'all',
    preset: state.preset,
    tz: state.timezone,
    compare: state.includeComparison ? '1' : '0'
  };
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

export function buildPowerTrendSeries(points: EnergyRollupPoint[]): {
  solar: number[];
  ac: number[];
  dc: number[];
  load: number[];
  battery: number[];
} {
  return {
    solar: points.map((point) => point.metrics.pvAvgW),
    ac: points.map((point) => point.metrics.acInAvgW),
    dc: points.map((point) => point.metrics.dcAvgW),
    load: points.map((point) => point.metrics.loadAvgW),
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
  return `${formatter.format(from)} - ${formatter.format(to)}`;
}

export function formatDeltaPct(deltaPct: number | null): string {
  if (deltaPct === null || !Number.isFinite(deltaPct)) {
    return 'No prior baseline';
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

export function buildEnergyInsights(
  points: EnergyRollupPoint[],
  timezone: string,
  preset: EnergyPreset,
  pvRows: PvEnvelopeRow[] = []
): EnergyInsightCard[] {
  const bestSolar = findBestBucket(points, 'pvAvgW');
  const bestLoad = findBestBucket(points, 'loadAvgW');
  const loadValues = points
    .map((point) => point.metrics.loadAvgW)
    .filter((value) => Number.isFinite(value));
  const sortedLoads = [...loadValues].sort((left, right) => left - right);
  const baseLoad = quantile(sortedLoads, 0.1);
  const spikeLoad = quantile(sortedLoads, 0.95);
  const spikeFactor =
    baseLoad !== null && spikeLoad !== null && baseLoad > 0 ? spikeLoad / baseLoad : null;
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

export function buildPvEnvelopeSummary(
  devices: DeviceSummary[],
  scope: 'device' | 'all',
  deviceId?: string
): PvEnvelopeSummary {
  const scopedDevices =
    scope === 'device' && deviceId
      ? devices.filter((device) => device.id === deviceId)
      : devices;

  const rows: PvEnvelopeRow[] = [];
  for (const device of scopedDevices) {
    for (const port of device.details?.solarPorts ?? []) {
      const powerUtilizationPct =
        port.maxWatts && port.maxWatts > 0 && port.watts !== undefined
          ? Math.max(0, Math.min(100, (port.watts / port.maxWatts) * 100))
          : null;
      const voltageHeadroom =
        port.maxVolts !== undefined && port.volts !== undefined ? port.maxVolts - port.volts : null;
      const currentHeadroom =
        port.maxAmps !== undefined && port.amps !== undefined ? port.maxAmps - port.amps : null;
      rows.push({
        deviceId: device.id,
        deviceName: device.name,
        portId: port.id,
        portLabel: port.name,
        observedVolts: Math.max(0, port.volts ?? 0),
        observedAmps: Math.max(0, port.amps ?? 0),
        observedPower: Math.max(0, port.watts ?? 0),
        maxVolts: Math.max(0, port.maxVolts ?? 0),
        maxAmps: Math.max(0, port.maxAmps ?? 0),
        maxPower: Math.max(0, port.maxWatts ?? 0),
        powerUtilizationPct,
        voltageHeadroom,
        currentHeadroom,
        bottleneckHint: buildBottleneckHint({
          observedPower: Math.max(0, port.watts ?? 0),
          maxPower: Math.max(0, port.maxWatts ?? 0),
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
    utilizationPct: configuredPower > 0 ? Math.max(0, Math.min(100, (observedPower / configuredPower) * 100)) : null,
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

function findBestBucket(
  points: EnergyRollupPoint[],
  metric: 'pvAvgW' | 'loadAvgW'
): { label: string; value: number } | null {
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

  return topRows
    .map((row) => `${formatPvRowLabel(row)} is ${row.bottleneckHint.toLowerCase()}.`)
    .join(' ');
}

function formatPvRowLabel(row: PvEnvelopeRow): string {
  return row.deviceName ? `${row.deviceName} ${row.portLabel}` : row.portLabel;
}
