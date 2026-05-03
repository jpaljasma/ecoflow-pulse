import type { DeviceSummary } from '@/features/devices/schema';
import type { DeviceLiveSignals } from '@/features/telemetry/engine/types';
import { signalTone } from '@/shared/ui/uiMappings';
import type { UiTone } from '@/shared/ui/uiMappings';

type DeviceDetailDiagnosticsEntry = {
  key: string;
  label: string;
  value: string;
  tone?: UiTone;
};

type DeviceDetailExtras = {
  acOn?: boolean;
  dcOn?: boolean;
  usbOn?: boolean;
  dc12vOn?: boolean;
  evChargingOn?: boolean;
  fanOn?: boolean;
  solarChargingOn?: boolean;
  batteryHeatingOn?: boolean;
  xBoostOn?: boolean;
  solarMode?: string;
  passthroughMode?: string;
  acAutoOnMode?: string;
  energyManagementOn?: boolean;
  diagnostics?: DeviceDetailDiagnosticsEntry[];
};

type DeviceDetailPowerBalance = {
  acInW?: number;
  pvW?: number;
  loadW?: number;
  netW?: number;
};

export type DetailSignalPillVM = {
  key: string;
  label: string;
  on?: boolean;
  value?: string;
  standalone?: boolean;
  tone: UiTone;
};

export type DetailDiagnosticPillVM = {
  key: string;
  label: string;
  value: string;
  tone: UiTone;
};

function aggregateSignalState(...values: Array<boolean | undefined>): boolean | undefined {
  if (values.some((value) => value === true)) {
    return true;
  }
  if (values.some((value) => value !== undefined)) {
    return false;
  }
  return undefined;
}

function getDetailExtras(details: DeviceSummary['details'] | undefined): DeviceDetailExtras | undefined {
  return details as DeviceDetailExtras | undefined;
}

function normalizeSignalValue(value: string | undefined): string | undefined {
  const trimmed = value?.trim();
  if (!trimmed) {
    return undefined;
  }
  return trimmed
    .replaceAll('_', ' ')
    .replaceAll('-', ' ')
    .replaceAll(/\s+/g, ' ')
    .replace(/\b[a-z]/g, (match) => match.toUpperCase());
}

function modeTone(value: string | undefined): UiTone {
  const normalized = (value ?? '').trim().toLowerCase();
  if (!normalized) {
    return 'warning';
  }
  if (
    normalized.includes('off') ||
    normalized.includes('disabled') ||
    normalized === '0' ||
    normalized === 'false'
  ) {
    return 'neutral';
  }
  if (
    normalized.includes('on') ||
    normalized.includes('enabled') ||
    normalized.includes('priority') ||
    normalized.includes('transfer') ||
    normalized.includes('solar')
  ) {
    return 'success';
  }
  return 'info';
}

function finiteNumber(value: number | undefined): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function isSolarPassthroughBalance(powerBalance: DeviceDetailPowerBalance | undefined): boolean {
  const pvW = finiteNumber(powerBalance?.pvW);
  const loadW = finiteNumber(powerBalance?.loadW);
  const netW = finiteNumber(powerBalance?.netW);
  const acInW = finiteNumber(powerBalance?.acInW) ?? 0;
  if (pvW === undefined || loadW === undefined || netW === undefined) {
    return false;
  }

  if (pvW < 25 || loadW < 20) {
    return false;
  }

  const acInputNoiseFloorW = Math.max(8, pvW * 0.02);
  if (acInW > acInputNoiseFloorW) {
    return false;
  }

  const selfLoadCeilingW = Math.max(45, Math.min(80, pvW * 0.12));
  return netW >= -5 && netW <= selfLoadCeilingW;
}

function buildPrimarySignalPills({
  liveSignals,
  details,
  supportsEvCharging,
  supportsBatteryHeating,
  preconditioningOn,
  powerBalance
}: {
  liveSignals?: DeviceLiveSignals;
  details: DeviceDetailExtras | undefined;
  supportsEvCharging: boolean;
  supportsBatteryHeating: boolean;
  preconditioningOn: boolean | undefined;
  powerBalance?: DeviceDetailPowerBalance;
}): DetailSignalPillVM[] {
  const acOn = liveSignals?.acOn ?? details?.acOn;
  const usbOn = liveSignals?.usbOn ?? details?.usbOn;
  const dc12vOn = liveSignals?.dc12vOn ?? details?.dc12vOn;
  const dcSignalOn = aggregateSignalState(liveSignals?.dcOn ?? details?.dcOn, usbOn, dc12vOn);
  const evChargingOn = liveSignals?.evChargingOn ?? details?.evChargingOn;
  const fanOn = liveSignals?.fanOn ?? details?.fanOn;
  const solarChargingOn = liveSignals?.solarChargingOn ?? details?.solarChargingOn;
  const xBoostOn = details?.xBoostOn;
  const energyManagementOn = details?.energyManagementOn;

  const signalPills: DetailSignalPillVM[] = [
    { key: 'ac', label: 'AC On', on: acOn, tone: signalTone(acOn) },
    { key: 'dc', label: 'DC On', on: dcSignalOn, tone: signalTone(dcSignalOn) },
    { key: 'usb', label: 'USB On', on: usbOn, tone: signalTone(usbOn) },
    { key: 'dc12', label: '12V On', on: dc12vOn, tone: signalTone(dc12vOn) },
    { key: 'xboost', label: 'X-Boost', on: xBoostOn, tone: signalTone(xBoostOn) },
    ...(details?.solarMode
      ? [
          {
            key: 'solar-mode',
            label: 'Solar Priority',
            value: normalizeSignalValue(details.solarMode),
            tone: modeTone(details.solarMode)
          }
        ]
      : []),
    ...(details?.passthroughMode
      ? [
          {
            key: 'passthrough-mode',
            label: 'Passthrough / Transfer Mode',
            value: normalizeSignalValue(details.passthroughMode),
            tone: modeTone(details.passthroughMode)
          }
        ]
      : []),
    ...(details?.acAutoOnMode
      ? [
          {
            key: 'ac-auto-on-mode',
            label: 'AC Auto-On / Always-On',
            value: normalizeSignalValue(details.acAutoOnMode),
            tone: modeTone(details.acAutoOnMode)
          }
        ]
      : []),
    {
      key: 'energy-mgmt',
      label: 'Energy Management',
      on: energyManagementOn,
      tone: signalTone(energyManagementOn)
    },
    ...(supportsEvCharging
      ? [{ key: 'ev', label: 'EV Charging', on: evChargingOn, tone: signalTone(evChargingOn) }]
      : []),
    ...(supportsBatteryHeating
      ? [{ key: 'preconditioning', label: 'Preconditioning', on: preconditioningOn, tone: signalTone(preconditioningOn) }]
      : []),
    { key: 'fan', label: 'Fan', on: fanOn, tone: signalTone(fanOn) },
    { key: 'solar', label: 'Solar Charging', on: solarChargingOn, tone: signalTone(solarChargingOn) },
    ...(isSolarPassthroughBalance(powerBalance)
      ? [
          {
            key: 'solar-passthrough',
            label: 'Solar Passthrough',
            standalone: true,
            tone: 'success' as const
          }
        ]
      : [])
  ];

  return signalPills.filter((pill) => pill.standalone === true || pill.on !== undefined || pill.value !== undefined);
}

function buildDiagnosticPills(details: DeviceDetailExtras | undefined): DetailDiagnosticPillVM[] {
  return (details?.diagnostics ?? []).map((entry) => ({
    key: entry.key,
    label: entry.label,
    value: entry.value,
    tone: entry.tone ?? 'neutral'
  }));
}

export function buildDeviceDetailSignalPills({
  liveSignals,
  details,
  supportsEvCharging,
  supportsBatteryHeating,
  preconditioningOn,
  powerBalance
}: {
  liveSignals?: DeviceLiveSignals;
  details: DeviceSummary['details'] | undefined;
  supportsEvCharging: boolean;
  supportsBatteryHeating: boolean;
  preconditioningOn: boolean | undefined;
  powerBalance?: DeviceDetailPowerBalance;
}): {
  signalPills: DetailSignalPillVM[];
  diagnosticPills: DetailDiagnosticPillVM[];
} {
  const extras = getDetailExtras(details);
  return {
    signalPills: buildPrimarySignalPills({
      liveSignals,
      details: extras,
      supportsEvCharging,
      supportsBatteryHeating,
      preconditioningOn,
      powerBalance
    }),
    diagnosticPills: buildDiagnosticPills(extras)
  };
}
