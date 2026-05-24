import type { ComponentProps } from 'react';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { useMemo } from 'react';
import type { DeviceSummary } from '@/features/devices/api';
import { resolveNetPowerW } from '@/features/devices/net';
import type { DeviceSnapshot, TelemetryEngineStatus } from '@/features/telemetry/engine/types';
import { formatEtaMinutes, formatSoc, formatW, formatWhAndKWh } from '@/features/telemetry/format';
import { getCapacityKWh } from '@/features/devices/capacity';
import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';
import {
  getDeviceVisualFallbackSource,
  getDeviceVisualImageUri
} from '@/features/devices/deviceVisuals';
import {
  mergeDeviceDetailSolarPorts,
  resolveLiveBatteryHeatingOn,
  sumSolarPortWatts
} from '@/features/device-detail/liveDetail';
import {
  buildDeviceDetailSignalPills,
  type DetailDiagnosticPillVM,
  type DetailSignalPillVM
} from '@/features/device-detail/signalPills';
import { solarPortView } from '@/features/device-detail/solarPort';
import { getStatusIconName } from '@/shared/ui/statusGlyph';
import { isMutedMetric } from '@/shared/ui/uiMappings';
import type { MetricTone, UiTone } from '@/shared/ui/uiMappings';

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
      previousWh?: number;
      deltaPct?: number | null;
    };

export type { DetailDiagnosticPillVM, DetailSignalPillVM } from '@/features/device-detail/signalPills';

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
  summaryText?: string;
  reserveText?: string;
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
  connectionGlyph: ComponentProps<typeof MaterialCommunityIcons>['name'];
  deviceAsset: {
    slug: string;
    uri?: string;
    icon: ComponentProps<typeof MaterialCommunityIcons>['name'];
  } | null;
  detailFallback?: ReturnType<typeof getDeviceVisualFallbackSource>;
  capacityKWh: number | null;
  batterySummaryText?: string;
  isColdTemp: boolean;
  metricCells: DetailMetricCellVM[];
  displayPvW?: number;
  batteryPacks: DetailBatteryPackVM[];
  solarPorts: DetailSolarPortVM[];
  signalPills: DetailSignalPillVM[];
  diagnosticPills: DetailDiagnosticPillVM[];
  estimatePills: DetailEstimatePillVM[];
  desktopScale: number;
  desktopOffsetY: number;
  details?: DeviceSummary['details'];
};

function metricToneFromValue(value: number | undefined): MetricTone | undefined {
  return isMutedMetric(value) ? 'muted' : 'default';
}

function fallbackState(state: DeviceSummary['state'] | string | undefined): 'charging' | 'discharging' | 'idle' {
  return state === 'charging' || state === 'discharging' || state === 'idle' ? state : 'idle';
}

export function useDeviceDetailViewModel({
  device,
  snapshot,
  connectionStatus,
  useRemoteImage,
  todayWh,
  yesterdayWh,
  todayDeltaPct
}: {
  device?: DeviceSummary;
  snapshot?: DeviceSnapshot;
  connectionStatus: TelemetryEngineStatus;
  useRemoteImage: boolean;
  todayWh?: number;
  yesterdayWh?: number;
  todayDeltaPct?: number | null;
}): DeviceDetailViewModel {
  const modelLower = (device?.model ?? '').toLowerCase();
  const details = device?.details;
  const liveDetail = snapshot?.liveDetail;
  const liveSignals = liveDetail?.signals;
  const resolvedSolarPortDetails = useMemo(
    () => mergeDeviceDetailSolarPorts(details?.solarPorts, liveDetail?.solarPorts),
    [details?.solarPorts, liveDetail?.solarPorts]
  );
  const liveSolarPortTotalW = useMemo(
    () => (liveDetail?.solarPorts && liveDetail.solarPorts.length > 0 ? sumSolarPortWatts(resolvedSolarPortDetails) : undefined),
    [liveDetail?.solarPorts, resolvedSolarPortDetails]
  );

  const acInW = snapshot?.metrics?.acW ?? device?.acInW;
  const dcW = snapshot?.metrics?.dcW ?? device?.dcW;
  const loadW = snapshot?.metrics?.loadW ?? device?.loadW;
  const pvW = liveSolarPortTotalW ?? snapshot?.metrics?.pvW ?? device?.pvW;
  const batteryW = snapshot?.metrics?.batteryW;
  const tempC = snapshot?.metrics?.tempC ?? device?.tempC;
  const isColdTemp = typeof tempC === 'number' && tempC <= 2;
  const netW = resolveNetPowerW({
    acInW,
    pvW,
    dcW,
    loadW,
    fallbackNetW: snapshot?.metrics
      ? resolveNetPowerW({
          acInW: snapshot.metrics.acW,
          pvW: snapshot.metrics.pvW,
          dcW: snapshot.metrics.dcW,
          loadW: snapshot.metrics.loadW
        })
      : device?.netW
  });
  const capacityKWh = device ? getCapacityKWh(device) : null;

  const detailState = useMemo<'charging' | 'discharging' | 'idle'>(() => {
    if (snapshot && !snapshot.stale && snapshot.status !== 'stale') {
      return snapshot.status;
    }
    return fallbackState(device?.state);
  }, [device?.state, snapshot]);

  const connectionGlyph = useMemo(() => {
    if (snapshot?.stale) return getStatusIconName('stale');
    if (snapshot?.inactive || snapshot?.online === false) return getStatusIconName('offline');
    if (!snapshot && device?.online === false) return getStatusIconName('offline');
    if (connectionStatus === 'connected') return getStatusIconName('online');
    if (connectionStatus === 'connecting' || connectionStatus === 'reconnecting') {
      return getStatusIconName('processing');
    }
    return getStatusIconName('waiting');
  }, [device?.online, snapshot, connectionStatus]);

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
      uri: useRemoteImage ? getDeviceVisualImageUri(match, 'detail') : undefined,
      icon: match.glyph.icon
    };
  }, [device?.model, device?.details?.bpCount, device?.capabilities, useRemoteImage]);

  const detailFallback = useMemo(
    () => {
      const model = device?.model;
      if (!model) return undefined;
      const batteryCount =
        device?.details?.bpCount ??
        ((device?.capabilities as { batteryPacks?: number } | undefined)?.batteryPacks ?? 1);
      return getDeviceVisualFallbackSource(getDeviceAssetMatch(model, { batteryCount }), '512');
    },
    [device?.model, device?.details?.bpCount, device?.capabilities]
  );

  const supportsEvCharging =
    modelLower.includes('delta pro ultra') ||
    (device?.capabilities as Record<string, unknown> | undefined)?.evCharging === true ||
    (device?.capabilities as Record<string, unknown> | undefined)?.evCharger === true ||
    (device?.capabilities as Record<string, unknown> | undefined)?.evOutput === true ||
    liveSignals?.evChargingOn !== undefined ||
    details?.evChargingOn !== undefined;

  const supportsBatteryHeating =
    modelLower.includes('delta pro ultra') ||
    (device?.capabilities as Record<string, unknown> | undefined)?.batteryHeating === true ||
    (device?.capabilities as Record<string, unknown> | undefined)?.preconditioning === true ||
    liveSignals?.batteryHeatingOn !== undefined ||
    details?.batteryHeatingOn !== undefined;

  const preconditioningOn = resolveLiveBatteryHeatingOn(liveSignals, details);
  const { signalPills, diagnosticPills } = buildDeviceDetailSignalPills({
    liveSignals,
    details,
    supportsEvCharging,
    supportsBatteryHeating,
    preconditioningOn,
    powerBalance: {
      acInW,
      pvW,
      loadW,
      netW
    }
  });

  const metricCells = useMemo<DetailMetricCellVM[]>(() => {
    const etaMinutes = details?.estimateEtaMin ?? device?.etaMinutes;
    return [
      { key: 'ac', kind: 'stat', label: 'AC', value: formatW(acInW), tone: metricToneFromValue(acInW) },
      { key: 'dc', kind: 'stat', label: 'DC', value: formatW(dcW), tone: metricToneFromValue(dcW) },
      { key: 'pv', kind: 'stat', label: 'PV', value: formatW(pvW), tone: metricToneFromValue(pvW) },
      { key: 'today', kind: 'today', valueWh: todayWh, previousWh: yesterdayWh, deltaPct: todayDeltaPct },
      {
        key: 'load',
        kind: 'stat',
        label: 'Load',
        value: formatW(loadW),
        tone: metricToneFromValue(loadW)
      },
      { key: 'net', kind: 'stat', label: 'Net', value: formatW(netW) },
      { key: 'battery', kind: 'stat', label: 'Battery', value: formatW(batteryW) },
      {
        key: 'temp',
        kind: 'stat',
        label: 'Temp',
        value: snapshot?.metrics ? `${snapshot.metrics.tempC.toFixed(1)}°C` : '—',
        tone: isColdTemp ? 'cold' : 'default'
      },
      { key: 'state', kind: 'stat', label: 'State', value: device ? detailState : '—' },
      { key: 'eta', kind: 'stat', label: 'ETA', value: formatEtaMinutes(etaMinutes) }
    ];
  }, [
    acInW,
    dcW,
    pvW,
    loadW,
    netW,
    batteryW,
    details?.estimateEtaMin,
    isColdTemp,
    snapshot?.metrics,
    device,
    detailState,
    todayDeltaPct,
    yesterdayWh,
    todayWh
  ]);

  const batteryPacks = useMemo<DetailBatteryPackVM[]>(
    () =>
      (details?.packs ?? []).map((pack) => ({
        id: pack.id,
        socPct: pack.socPct,
        powerText: Number.isFinite(pack.powerW as number) ? formatW(pack.powerW) : '—',
        tempText: Number.isFinite(pack.tempC as number) ? `${pack.tempC?.toFixed(1)}°C` : '—',
        heatingOn: pack.heatingOn === true,
        summaryText: [formatWhAndKWh(pack.energyWh), formatEtaMinutes(pack.remainMinutes)]
          .filter((part) => part !== '—')
          .join(' · '),
        reserveText: [formatSoc(pack.socMinPct), formatSoc(pack.socMaxPct)]
          .filter((part) => part !== '—')
          .join(' - ')
      })),
    [details?.packs]
  );

  const solarPorts = useMemo<DetailSolarPortVM[]>(
    () => (resolvedSolarPortDetails ?? []).map((port) => solarPortView(port)),
    [resolvedSolarPortDetails]
  );

  const batterySummaryText = useMemo(() => {
    if (!details) {
      return undefined;
    }
    const socWindowText =
      details.socWindowMinPct !== undefined && details.socWindowMaxPct !== undefined
        ? `SOC window ${formatSoc(details.socWindowMinPct)} - ${formatSoc(details.socWindowMaxPct)}`
        : details.socWindowMinPct !== undefined
          ? `SOC min ${formatSoc(details.socWindowMinPct)}`
          : details.socWindowMaxPct !== undefined
            ? `SOC max ${formatSoc(details.socWindowMaxPct)}`
            : undefined;
    const segments = [
      socWindowText,
      details.backupReservePct !== undefined ? `Backup ${formatSoc(details.backupReservePct)}` : undefined
    ].filter(Boolean);
    return segments.length > 0 ? segments.join(' · ') : undefined;
  }, [details]);

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
    batterySummaryText,
    isColdTemp,
    metricCells,
    displayPvW: pvW,
    batteryPacks,
    solarPorts,
    signalPills,
    diagnosticPills,
    estimatePills,
    desktopScale,
    desktopOffsetY,
    details
  };
}
