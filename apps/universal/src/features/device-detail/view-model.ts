import { useMemo } from 'react';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot, TelemetryEngineStatus } from '@/features/telemetry/engine/types';
import { formatEtaMinutes, formatW } from '@/features/telemetry/format';
import { getCapacityKWh } from '@/features/devices/capacity';
import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';
import { getEcoFlowAsset, getEcoFlowDefaultSize } from '@/shared/assets/ecoflowAssets';
import { getBundledDeviceFallback } from '@/shared/assets/deviceFallbacks';
import { getStatusGlyph } from '@/shared/ui/statusGlyph';
import {
  clampPercent,
  formatSolarState,
  isInactivePvPort,
  isMutedMetric,
  signalTone,
  toneFromState,
  toPctOfMax,
  type MetricTone,
  type UiTone
} from '@/shared/ui/uiMappings';

export type DetailMetricCellVM =
  | {
      key: string;
      kind: 'stat';
      label: string;
      value: string;
      tone?: MetricTone;
    }
  | {
      key: 'today';
      kind: 'today';
      valueWh?: number;
    };

export type DetailSignalPillVM = {
  key: string;
  label: string;
  on?: boolean;
  tone: UiTone;
};

export type DetailEstimatePillVM = {
  key: string;
  label: string;
  tone: UiTone;
};

export type DetailBatteryPackVM = {
  id: string;
  socPct?: number;
  powerText: string;
  tempText: string;
  heatingOn: boolean;
};

export type DetailSolarPortVM = {
  id: string;
  name: string;
  stateLabel: string;
  stateTone: UiTone;
  inactive: boolean;
  wattsText: string;
  voltsText: string;
  ampsText: string;
  capText: string;
  watts?: number;
  volts?: number;
  amps?: number;
  maxWatts?: number;
  pvLoadPct: number | null;
  pvLoadClamped: number;
};

export type DeviceDetailViewModel = {
  modelLower: string;
  detailState: 'charging' | 'discharging' | 'idle';
  connectionGlyph: string;
  deviceAsset: {
    slug: string;
    uri?: string;
    emoji: string;
  } | null;
  detailFallback?: ReturnType<typeof getBundledDeviceFallback>;
  capacityKWh: number | null;
  isColdTemp: boolean;
  metricCells: DetailMetricCellVM[];
  batteryPacks: DetailBatteryPackVM[];
  solarPorts: DetailSolarPortVM[];
  signalPills: DetailSignalPillVM[];
  estimatePills: DetailEstimatePillVM[];
  desktopScale: number;
  desktopOffsetY: number;
  details?: DeviceSummary['details'];
};

function metricToneFromValue(value: number | undefined): MetricTone | undefined {
  return isMutedMetric(value) ? 'muted' : 'default';
}

function fallbackState(
  state: DeviceSummary['state'] | string | undefined
): 'charging' | 'discharging' | 'idle' {
  return state === 'charging' || state === 'discharging' || state === 'idle' ? state : 'idle';
}

function solarPortView(port: {
  id: string;
  name: string;
  state?: string;
  volts?: number;
  amps?: number;
  watts?: number;
  maxVolts?: number;
  maxAmps?: number;
  maxWatts?: number;
}): DetailSolarPortVM {
  const inactive = isInactivePvPort(port.volts);
  const stateLabelRaw = formatSolarState(port.state).toLowerCase();
  const hasFlow =
    (Number.isFinite(port.watts as number) && (port.watts as number) > 1) ||
    (Number.isFinite(port.amps as number) && (port.amps as number) > 0.03);
  const stateLabel = inactive ? 'inactive' : stateLabelRaw === 'unknown' && hasFlow ? 'active' : stateLabelRaw;
  const stateTone = inactive ? 'neutral' : toneFromState(stateLabel);
  const pvLoadPct = toPctOfMax(port.watts, port.maxWatts);

  return {
    id: port.id,
    name: port.name,
    stateLabel,
    stateTone,
    inactive,
    wattsText: formatW(port.watts),
    voltsText: Number.isFinite(port.volts as number) ? `${port.volts?.toFixed(1)}V` : '—',
    ampsText: Number.isFinite(port.amps as number) ? `${port.amps?.toFixed(2)}A` : '—',
    capText: port.maxWatts ? `${port.maxWatts}W · ${port.maxVolts ?? '—'}V · ${port.maxAmps ?? '—'}A` : '—',
    watts: port.watts,
    volts: port.volts,
    amps: port.amps,
    maxWatts: port.maxWatts,
    pvLoadPct,
    pvLoadClamped: clampPercent(pvLoadPct ?? 0)
  };
}

export function useDeviceDetailViewModel({
  device,
  snapshot,
  connectionStatus,
  useRemoteImage
}: {
  device?: DeviceSummary;
  snapshot?: DeviceSnapshot;
  connectionStatus: TelemetryEngineStatus;
  useRemoteImage: boolean;
}): DeviceDetailViewModel {
  const modelLower = (device?.model ?? '').toLowerCase();
  const details = device?.details;

  const acInW = device?.acInW;
  const dcW = device?.dcW;
  const pvW = snapshot?.metrics?.pvW;
  const loadW = snapshot?.metrics?.loadW;
  const batteryW = snapshot?.metrics?.batteryW;
  const tempC = snapshot?.metrics?.tempC;
  const isColdTemp = typeof tempC === 'number' && tempC <= 2;
  const netW = snapshot?.metrics ? snapshot.metrics.pvW - snapshot.metrics.loadW : device?.netW;
  const capacityKWh = device ? getCapacityKWh(device) : null;

  const detailState = useMemo<'charging' | 'discharging' | 'idle'>(() => {
    if (snapshot && !snapshot.stale && snapshot.status !== 'stale') {
      return snapshot.status;
    }
    return fallbackState(device?.state);
  }, [device?.state, snapshot]);

  const connectionGlyph = useMemo(() => {
    if (snapshot?.stale) return getStatusGlyph('stale');
    if (connectionStatus === 'connected') return getStatusGlyph('online');
    if (connectionStatus === 'connecting' || connectionStatus === 'reconnecting') {
      return getStatusGlyph('processing');
    }
    return getStatusGlyph('waiting');
  }, [snapshot?.stale, connectionStatus]);

  const deviceAsset = useMemo(() => {
    const model = device?.model;
    if (!model) return null;
    const batteryCount =
      device?.details?.bpCount ??
      ((device?.capabilities as { batteryPacks?: number } | undefined)?.batteryPacks ?? 1);
    const match = getDeviceAssetMatch(model, { batteryCount });
    if (!match.slug) return null;
    return {
      slug: match.slug,
      uri: useRemoteImage ? getEcoFlowAsset(match.slug, getEcoFlowDefaultSize('detail')) : undefined,
      emoji: match.glyph.emoji
    };
  }, [device?.model, device?.details?.bpCount, device?.capabilities, useRemoteImage]);

  const detailFallback = useMemo(
    () => (deviceAsset?.slug ? getBundledDeviceFallback(deviceAsset.slug, '512') : undefined),
    [deviceAsset?.slug]
  );

  const supportsEvCharging =
    modelLower.includes('delta pro ultra') ||
    (device?.capabilities as Record<string, unknown> | undefined)?.evCharging === true ||
    (device?.capabilities as Record<string, unknown> | undefined)?.evCharger === true ||
    (device?.capabilities as Record<string, unknown> | undefined)?.evOutput === true;

  const supportsBatteryHeating =
    modelLower.includes('delta pro ultra') ||
    (device?.capabilities as Record<string, unknown> | undefined)?.batteryHeating === true ||
    (device?.capabilities as Record<string, unknown> | undefined)?.preconditioning === true;

  const preconditioningOn =
    details?.batteryHeatingOn === true ||
    (details?.packs ?? []).some((pack) => pack.heatingOn === true);

  const metricCells = useMemo<DetailMetricCellVM[]>(() => {
    return [
      { key: 'ac', kind: 'stat', label: '∿ AC', value: formatW(acInW), tone: metricToneFromValue(acInW) },
      { key: 'dc', kind: 'stat', label: '⎓ DC', value: formatW(dcW), tone: metricToneFromValue(dcW) },
      { key: 'pv', kind: 'stat', label: '☼ PV', value: formatW(pvW), tone: metricToneFromValue(pvW) },
      { key: 'today', kind: 'today', valueWh: device?.solarTodayWh },
      {
        key: 'load',
        kind: 'stat',
        label: '⌂ Load',
        value: formatW(loadW),
        tone: metricToneFromValue(loadW)
      },
      { key: 'net', kind: 'stat', label: '⚖ Net', value: formatW(netW) },
      { key: 'battery', kind: 'stat', label: '🔋 Battery', value: formatW(batteryW) },
      {
        key: 'temp',
        kind: 'stat',
        label: isColdTemp ? '❄ Temp' : '🌡 Temp',
        value: snapshot?.metrics ? `${snapshot.metrics.tempC.toFixed(1)}°C` : '—',
        tone: isColdTemp ? 'cold' : 'default'
      },
      { key: 'state', kind: 'stat', label: '◉ State', value: device ? detailState : '—' },
      { key: 'eta', kind: 'stat', label: '⏱ ETA', value: formatEtaMinutes(device?.etaMinutes) }
    ];
  }, [acInW, dcW, pvW, loadW, netW, batteryW, isColdTemp, snapshot?.metrics, device, detailState]);

  const batteryPacks = useMemo<DetailBatteryPackVM[]>(
    () =>
      (details?.packs ?? []).map((pack) => ({
        id: pack.id,
        socPct: pack.socPct,
        powerText: Number.isFinite(pack.powerW as number) ? formatW(pack.powerW) : '—',
        tempText: Number.isFinite(pack.tempC as number) ? `${pack.tempC?.toFixed(1)}°C` : '—',
        heatingOn: pack.heatingOn === true
      })),
    [details?.packs]
  );

  const solarPorts = useMemo<DetailSolarPortVM[]>(
    () => (details?.solarPorts ?? []).map((port) => solarPortView(port)),
    [details?.solarPorts]
  );

  const estimatePills = useMemo<DetailEstimatePillVM[]>(() => {
    const estimateLabel = details?.estimateMode
      ? `${details.estimateMode}${details.estimateSource ? ` · ${details.estimateSource}` : ''}`
      : details?.estimateSource ?? 'n/a';
    return [
      { key: 'mode', label: `mode: ${estimateLabel}`, tone: 'neutral' },
      {
        key: 'eta',
        label: `eta: ${formatEtaMinutes(details?.estimateEtaMin ?? device?.etaMinutes)}`,
        tone: 'info'
      },
      {
        key: 'queue',
        label: `queue: ${details?.mqttQueueDepth ?? 0}`,
        tone: (details?.mqttQueueDepth ?? 0) > 48 ? 'warning' : 'success'
      },
      {
        key: 'dropped',
        label: `dropped: ${details?.mqttQueueDroppedOldest ?? 0}`,
        tone: (details?.mqttQueueDroppedOldest ?? 0) > 0 ? 'danger' : 'neutral'
      }
    ];
  }, [details?.estimateMode, details?.estimateSource, details?.estimateEtaMin, details?.mqttQueueDepth, details?.mqttQueueDroppedOldest, device?.etaMinutes]);

  const signalPills = useMemo<DetailSignalPillVM[]>(() => {
    const dcSignalOn = details?.dcOn === true || details?.usbOn === true || details?.dc12vOn === true;
    const signals: Array<{ key: string; label: string; on?: boolean }> = [
      { key: 'ac', label: 'AC On', on: details?.acOn },
      { key: 'dc', label: 'DC On', on: dcSignalOn },
      { key: 'usb', label: 'USB On', on: details?.usbOn },
      { key: 'dc12', label: '12V On', on: details?.dc12vOn },
      ...(supportsEvCharging ? [{ key: 'ev', label: 'EV Charging', on: details?.evChargingOn }] : []),
      ...(supportsBatteryHeating ? [{ key: 'preconditioning', label: 'Preconditioning', on: preconditioningOn }] : []),
      { key: 'fan', label: 'Fan', on: details?.fanOn },
      { key: 'solar', label: 'Solar Charging', on: details?.solarChargingOn }
    ];
    return signals.map((signal) => ({
      key: signal.key,
      label: signal.label,
      on: signal.on,
      tone: signalTone(signal.on)
    }));
  }, [details, preconditioningOn, supportsBatteryHeating, supportsEvCharging]);

  const isDelta2Max = modelLower.includes('delta 2 max');
  const isDeltaProUltra = modelLower.includes('delta pro ultra');
  const desktopScale = isDelta2Max ? 1.46 : isDeltaProUltra ? 1.5 : 1.48;
  const desktopOffsetY = isDelta2Max ? 8 : 0;

  return {
    modelLower,
    detailState,
    connectionGlyph,
    deviceAsset,
    detailFallback,
    capacityKWh,
    isColdTemp,
    metricCells,
    batteryPacks,
    solarPorts,
    signalPills,
    estimatePills,
    desktopScale,
    desktopOffsetY,
    details
  };
}
