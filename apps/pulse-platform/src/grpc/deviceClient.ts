import type { FastifyRequest } from 'fastify';

import type { AppConfig } from '../config.js';
import type {
  AvailableProviderDevice,
  ControlPlaneClient,
  ProviderDevice,
  ProviderDeviceGroup,
  ProviderDeviceMQTTTestResult
} from './controlPlaneClient.js';
import type { TelemetrySnapshotClient } from './telemetryClient.js';
import {
  deriveBatteryPower,
  deriveTelemetryEtaMinutes,
  deriveTelemetryMetrics,
  deriveTelemetryState
} from '../telemetry/deriveMetrics.js';
import {
  buildProviderDevicePresentation,
  deriveDeviceDetailsEtaMinutes,
  type BatteryPackDetail,
  type DeviceCapabilities,
  type DeviceTelemetryDetails,
  type SolarPortDetail
} from '../devices/providerDeviceMapper.js';
import { deriveStormGuardState } from '../devices/stormGuardState.js';

export type DeviceSummary = {
  id: string;
  serialNumber: string;
  name: string;
  model: string;
  online: boolean;
  batteryPct: number;
  state: 'charging' | 'discharging' | 'idle';
  etaMinutes: number;
  pvW?: number;
  acInW?: number;
  dcW?: number;
  loadW?: number;
  netW?: number;
  tempC?: number;
  telemetryTsMs?: number;
  capabilities?: DeviceCapabilities;
  details?: DeviceTelemetryDetails;
};

export type AvailableDeviceSummary = {
  provider: string;
  providerDeviceId: string;
  credentialId: string;
  serialNumber: string;
  name: string;
  model: string;
  capabilities?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
};

export type AvailableDevicesResult = {
  devices: AvailableDeviceSummary[];
  hasActiveCredentials: boolean;
};

const providerMQTTValidationDeadlinePaddingMs = 12_000;
const DEVICE_SNAPSHOT_FRESH_MS = 2 * 60_000;
const SNAPSHOT_PV_PORT_FIELD = /^params\.pv(\d+)(InVol|InAmp|ChargeWatts|InWatts|ChgState)$/;

export interface DeviceClient {
  listDevices(request: FastifyRequest): Promise<DeviceSummary[]>;
  getDevice(request: FastifyRequest, routeDeviceId: string): Promise<DeviceSummary | null>;
  listAvailableDevices(request: FastifyRequest): Promise<AvailableDevicesResult>;
  testAvailableDeviceMQTT(
    request: FastifyRequest,
    input: { provider: string; credentialId: string; providerDeviceId: string }
  ): Promise<ProviderDeviceMQTTTestResult>;
  enableAvailableDevice(
    request: FastifyRequest,
    input: { provider: string; credentialId: string; providerDeviceId: string }
  ): Promise<{ deviceId: string }>;
  importAvailableDevice(
    request: FastifyRequest,
    input: { provider: string; credentialId: string; providerDeviceId: string; isActive: boolean; ingestDesiredState?: string }
  ): Promise<{ deviceId: string }>;
  close(): void;
}

export function createDeviceClient(
  config: AppConfig,
  controlPlaneClient: ControlPlaneClient,
  telemetryClient: TelemetrySnapshotClient
): DeviceClient {
  return {
    async listDevices(request) {
      const userSubject = resolveUserSubject(config, request);
      const groups = await controlPlaneClient.listDevices({
        userSubject,
        activeOnly: true,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: config.grpcDeadlineMs
      });
      const providerDevices = flattenProviderDevices(groups);
      const hydrated = await Promise.all(
        providerDevices.map((device) => hydrateDevice(device, request, config, telemetryClient))
      );
      return hydrated;
    },
    async getDevice(request, routeDeviceId) {
      const devices = await this.listDevices(request);
      return devices.find((device) => device.id === routeDeviceId || device.serialNumber === routeDeviceId) ?? null;
    },
    async listAvailableDevices(request) {
      const userSubject = resolveUserSubject(config, request);
      const response = await controlPlaneClient.listAvailableProviderDevices({
        userSubject,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: config.grpcDeadlineMs
      });
      return {
        hasActiveCredentials: response.hasActiveCredentials,
        devices: response.devices.map(mapAvailableProviderDevice)
      };
    },
    async testAvailableDeviceMQTT(request, input) {
      const userSubject = resolveUserSubject(config, request);
      return controlPlaneClient.testProviderDeviceMQTT({
        userSubject,
        provider: input.provider,
        credentialId: input.credentialId,
        providerDeviceId: input.providerDeviceId,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: providerMQTTValidationDeadline(config.grpcDeadlineMs)
      });
    },
    async enableAvailableDevice(request, input) {
      const userSubject = resolveUserSubject(config, request);
      const response = await controlPlaneClient.enableProviderDevice({
        userSubject,
        provider: input.provider,
        credentialId: input.credentialId,
        providerDeviceId: input.providerDeviceId,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: providerMQTTValidationDeadline(config.grpcDeadlineMs)
      });
      return { deviceId: response.userDevice.deviceId };
    },
    async importAvailableDevice(request, input) {
      const userSubject = resolveUserSubject(config, request);
      const response = await controlPlaneClient.importProviderDevice({
        userSubject,
        provider: input.provider,
        credentialId: input.credentialId,
        providerDeviceId: input.providerDeviceId,
        isActive: input.isActive,
        ingestDesiredState: input.ingestDesiredState,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: input.isActive ? providerMQTTValidationDeadline(config.grpcDeadlineMs) : config.grpcDeadlineMs
      });
      return { deviceId: response.userDevice.deviceId };
    },
    close() {
      controlPlaneClient.close();
      telemetryClient.close();
    }
  };
}

function providerMQTTValidationDeadline(baseDeadlineMs: number): number {
  return baseDeadlineMs + providerMQTTValidationDeadlinePaddingMs;
}

async function hydrateDevice(
  device: ProviderDevice,
  request: FastifyRequest,
  config: AppConfig,
  telemetryClient: TelemetrySnapshotClient
): Promise<DeviceSummary> {
  const presentation = buildProviderDevicePresentation(device);
  const base = baseDeviceSummary(device, presentation);
  try {
    const response = await telemetryClient.getSnapshot({
      deviceId: device.deviceId,
      authHeader: getAuthHeader(request),
      requestID: getRequestID(request),
      deadlineMs: config.grpcDeadlineMs
    });
    const rawMetrics = response.snapshot?.metrics ?? {};
    const telemetryTsMs = parsePositiveInt(response.snapshot?.cursor?.tsUnixMs);
    const online = telemetryTsMs !== null
      ? Date.now() - telemetryTsMs <= DEVICE_SNAPSHOT_FRESH_MS && hasFreshCurrentTelemetryMetric(rawMetrics)
      : false;
    const derived = deriveTelemetryMetrics(rawMetrics);
    const details = finalizeSnapshotDetails(
      mergeSnapshotDetails(prepareStaticDetailsForSnapshot(presentation.details, rawMetrics, online), rawMetrics),
      online
    );
    const pvW = deriveSummaryPvWatts(derived.pvW, details);
    const state = deriveTelemetryState(derived.batteryW);
    if (!online) {
      return {
        ...base,
        online: false,
        batteryPct: clampPercent(details?.overallSocPct ?? derived.soc),
        state: 'idle',
        etaMinutes: 0,
        pvW: 0,
        acInW: 0,
        dcW: 0,
        loadW: 0,
        netW: 0,
        telemetryTsMs: telemetryTsMs ?? undefined,
        details
      };
    }
    return {
      ...base,
      online,
      batteryPct: clampPercent(details?.overallSocPct ?? derived.soc),
      state,
      etaMinutes: deriveSummaryEtaMinutes(deriveTelemetryEtaMinutes(rawMetrics, derived.batteryW), details),
      pvW,
      acInW: derived.acW,
      dcW: derived.dcW,
      loadW: derived.loadW,
      netW: pvW - derived.loadW,
      tempC: derived.tempC,
      telemetryTsMs: telemetryTsMs ?? undefined,
      details
    };
  } catch {
    return base;
  }
}

function prepareStaticDetailsForSnapshot(
  details: DeviceTelemetryDetails | undefined,
  metrics: Record<string, number>,
  online: boolean
): DeviceTelemetryDetails | undefined {
  if (!details) {
    return undefined;
  }
  let next = details;
  if (!online || !hasFreshSolarMetric(metrics)) {
    next = clearSolarLiveDetails(next, online ? 'idle' : 'inactive');
  }
  if (!online || !hasFreshEtaMetric(metrics)) {
    next = clearEtaDetails(next);
  }
  return next;
}

function finalizeSnapshotDetails(
  details: DeviceTelemetryDetails | undefined,
  online: boolean
): DeviceTelemetryDetails | undefined {
  if (!details || online) {
    return details;
  }
  return {
    ...clearEtaDetails(details),
    packs: details.packs?.map((pack) => ({
      ...pack,
      powerW: 0,
      remainMinutes: undefined
    })),
    solarPorts: details.solarPorts?.map((port) => ({
      ...port,
      state: 'inactive',
      volts: 0,
      amps: 0,
      watts: 0
    })),
    acOn: false,
    dcOn: false,
    usbOn: false,
    dc12vOn: false,
    evChargingOn: false,
    fanOn: false,
    solarChargingOn: false,
    batteryHeatingOn: false
  };
}

function clearEtaDetails(details: DeviceTelemetryDetails): DeviceTelemetryDetails {
  return {
    ...details,
    packs: details.packs?.map((pack) => ({ ...pack, remainMinutes: undefined })),
    estimateEtaMin: undefined,
    estimateMode: undefined,
    estimateSource: undefined,
    remainChargeMin: undefined,
    remainDischargeMin: undefined,
    remainGlobalMin: undefined
  };
}

function clearSolarLiveDetails(details: DeviceTelemetryDetails, state: string): DeviceTelemetryDetails {
  return {
    ...details,
    solarPorts: details.solarPorts?.map((port) => ({
      ...port,
      state,
      volts: undefined,
      amps: undefined,
      watts: undefined
    })),
    solarChargingOn: false
  };
}

function hasFreshSolarMetric(metrics: Record<string, number>): boolean {
  return Object.keys(metrics).some((key) => {
    if (key === 'pvW') {
      return true;
    }
    if (!key.startsWith('params.')) {
      return false;
    }
    return (
      key.includes('Mppt') ||
      key === 'params.inVol' ||
      key === 'params.inAmp' ||
      key === 'params.chgState' ||
      SNAPSHOT_PV_PORT_FIELD.test(key)
    );
  });
}

function hasFreshEtaMetric(metrics: Record<string, number>): boolean {
  return Object.keys(metrics).some((key) => key.endsWith('remainTime') || key.endsWith('RemainTime'));
}

function hasFreshCurrentTelemetryMetric(metrics: Record<string, number>): boolean {
  return Object.keys(metrics).some(isCurrentTelemetryMetricKey);
}

function isCurrentTelemetryMetricKey(key: string): boolean {
  const clean = key.trim().toLowerCase();
  if (!clean) {
    return false;
  }
  if (['pvw', 'acw', 'dcw', 'loadw', 'batteryw'].includes(clean)) {
    return true;
  }
  if (clean.includes('remaintime')) {
    return true;
  }
  if (clean.includes('watt') || clean.includes('pwr') || clean.includes('mppt')) {
    return true;
  }
  if (
    clean.includes('.pv') &&
    (clean.includes('invol') ||
      clean.includes('inamp') ||
      clean.includes('inwatt') ||
      clean.includes('chargewatt') ||
      clean.includes('chgstate'))
  ) {
    return true;
  }
  if (
    clean.endsWith('.invol') ||
    clean.endsWith('.inamp') ||
    clean.endsWith('.batvol') ||
    clean.endsWith('.batamp')
  ) {
    return true;
  }
  return clean.includes('.out') && (clean.includes('vol') || clean.includes('amp'));
}

function baseDeviceSummary(device: ProviderDevice, presentation: ReturnType<typeof buildProviderDevicePresentation>): DeviceSummary {
  return {
    id: device.deviceId,
    serialNumber: presentation.serialNumber,
    name: device.productName || device.model || presentation.serialNumber || device.deviceId,
    model: device.model || device.productName || 'Unknown EcoFlow',
    online: false,
    batteryPct: 0,
    state: 'idle',
    etaMinutes: 0,
    capabilities: presentation.capabilities,
    details: presentation.details
  };
}

function mergeSnapshotDetails(
  details: DeviceTelemetryDetails | undefined,
  metrics: Record<string, number>
): DeviceTelemetryDetails | undefined {
  const snapshotStorm = deriveStormGuardFromSnapshotMetrics(metrics);
  const snapshotLive = deriveLiveDetailsFromSnapshotMetrics(metrics, details);
  if (!snapshotStorm && !snapshotLive) {
    return details;
  }
  return {
    ...(details ?? {}),
    ...(snapshotLive ?? {}),
    ...snapshotStorm
  };
}

function deriveLiveDetailsFromSnapshotMetrics(
  metrics: Record<string, number>,
  details: DeviceTelemetryDetails | undefined
): DeviceTelemetryDetails | undefined {
  const next: DeviceTelemetryDetails = {};
  const aggregateSocPct = firstDefinedNumber(
    metrics['params.f32LcdShowSoc'],
    metrics['params.lcdShowSoc'],
    metrics['params.f32ShowSoc']
  );
  const fallbackSocPct = firstDefinedNumber(
    metrics['params.soc']
  );
  const overallSocPct = aggregateSocPct ?? (details?.overallSocPct === undefined ? fallbackSocPct : undefined);
  if (overallSocPct !== undefined) {
    next.overallSocPct = overallSocPct;
  }

  const livePack = deriveLiveBatteryPack(metrics, details?.packs);
  if (livePack) {
    next.packs = mergeLiveBatteryPack(details?.packs, livePack);
  }

  const liveSolarPorts = deriveLiveSolarPorts(metrics, details?.solarPorts);
  if (liveSolarPorts && liveSolarPorts.length > 0) {
    next.solarPorts = liveSolarPorts;
    next.solarChargingOn = liveSolarPorts.some((port) => port.state === 'charging' || (port.watts ?? 0) > 1);
  }

  assignDefinedBool(next, 'acOn', toBooleanMetric(firstDefinedNumber(metrics['params.cfgAcEnabled'])));
  const dcOn = toBooleanMetric(firstDefinedNumber(metrics['params.dcOutState']));
  assignDefinedBool(next, 'dcOn', dcOn);
  assignDefinedBool(next, 'dc12vOn', dcOn);
  assignDefinedBool(next, 'batteryHeatingOn', toBooleanMetric(firstDefinedNumber(metrics['params.batteryHeatingOn'])));

  const remainGlobalMin = firstDefinedNumber(metrics['params.remainTime']);
  const remainChargeMin = firstDefinedNumber(metrics['params.chgRemainTime']);
  const remainDischargeMin = firstDefinedNumber(metrics['params.dsgRemainTime']);
  if (remainGlobalMin !== undefined) {
    next.remainGlobalMin = remainGlobalMin;
  }
  if (remainChargeMin !== undefined) {
    next.remainChargeMin = remainChargeMin;
  }
  if (remainDischargeMin !== undefined) {
    next.remainDischargeMin = remainDischargeMin;
  }

  return Object.keys(next).length > 0 ? next : undefined;
}

function deriveLiveBatteryPack(
  metrics: Record<string, number>,
  staticPacks: BatteryPackDetail[] | undefined
): BatteryPackDetail | undefined {
  const socPct = firstDefinedNumber(
    metrics['params.f32LcdShowSoc'],
    metrics['params.lcdShowSoc'],
    metrics['params.f32ShowSoc'],
    metrics['params.soc']
  );
  const powerW = deriveBatteryPower(metrics);
  const tempC = firstDefinedNumber(metrics['params.temp']);
  const remainMinutes = firstDefinedNumber(metrics['params.remainTime']);
  if (socPct === undefined && powerW === undefined && tempC === undefined && remainMinutes === undefined) {
    return undefined;
  }
  const base = staticPacks?.find((pack) => pack.id === 'main') ?? staticPacks?.[0] ?? { id: 'main' };
  return {
    ...base,
    ...(socPct !== undefined ? { socPct } : {}),
    ...(powerW !== undefined ? { powerW } : {}),
    ...(tempC !== undefined ? { tempC } : {}),
    ...(remainMinutes !== undefined ? { remainMinutes } : {})
  };
}

function mergeLiveBatteryPack(
  staticPacks: BatteryPackDetail[] | undefined,
  livePack: BatteryPackDetail
): BatteryPackDetail[] {
  if (!staticPacks || staticPacks.length === 0) {
    return [livePack];
  }
  let merged = false;
  const packs = staticPacks.map((pack, index) => {
    if (pack.id === livePack.id || (!merged && index === 0)) {
      merged = true;
      return { ...pack, ...livePack };
    }
    return pack;
  });
  return merged ? packs : [livePack, ...packs];
}

function deriveLiveSolarPorts(
  metrics: Record<string, number>,
  staticPorts: SolarPortDetail[] | undefined
): SolarPortDetail[] | undefined {
  const indexes = collectSnapshotPVPortIndexes(metrics);
  if (indexes.length === 0) {
    return undefined;
  }
  const livePorts = indexes.map((index) => {
    const prefix = `params.pv${index}`;
    const volts = firstDefinedNumber(metrics[`${prefix}InVol`]);
    const amps = firstDefinedNumber(metrics[`${prefix}InAmp`]);
    const watts = firstDefinedNumber(metrics[`${prefix}ChargeWatts`], metrics[`${prefix}InWatts`]);
    const rawState = firstDefinedNumber(metrics[`${prefix}ChgState`]);
    return {
      id: `pv-${index}`,
      name: `PV ${index}`,
      state: deriveSnapshotSolarPortState(rawState, volts, watts, amps),
      ...(volts !== undefined ? { volts } : {}),
      ...(amps !== undefined ? { amps } : {}),
      ...(watts !== undefined ? { watts } : {})
    };
  });

  if (!staticPorts || staticPorts.length === 0) {
    return livePorts;
  }
  const liveByID = new Map(livePorts.map((port) => [port.id, port]));
  const merged = staticPorts.map((port) => ({ ...port, ...(liveByID.get(port.id) ?? {}) }));
  const staticIDs = new Set(staticPorts.map((port) => port.id));
  for (const port of livePorts) {
    if (!staticIDs.has(port.id)) {
      merged.push(port);
    }
  }
  return merged;
}

function collectSnapshotPVPortIndexes(metrics: Record<string, number>): number[] {
  const indexes = new Set<number>();
  for (const [key, value] of Object.entries(metrics)) {
    if (!Number.isFinite(value)) {
      continue;
    }
    const matches = SNAPSHOT_PV_PORT_FIELD.exec(key);
    if (!matches) {
      continue;
    }
    const index = Number(matches[1]);
    if (Number.isInteger(index) && index > 0) {
      indexes.add(index);
    }
  }
  return [...indexes].sort((left, right) => left - right);
}

function deriveSnapshotSolarPortState(
  rawState: number | undefined,
  volts: number | undefined,
  watts: number | undefined,
  amps: number | undefined
): string {
  if ((watts ?? 0) > 1 || (amps ?? 0) > 0.03) {
    return 'charging';
  }
  if (volts !== undefined && volts <= 0.1) {
    return 'inactive';
  }
  if (rawState !== undefined) {
    if (rawState >= 2) return 'charging';
    if (rawState === 1) return 'locked';
  }
  return 'idle';
}

function assignDefinedBool(
  details: DeviceTelemetryDetails,
  key: 'acOn' | 'dcOn' | 'dc12vOn' | 'batteryHeatingOn',
  value: boolean | undefined
): void {
  if (value !== undefined) {
    details[key] = value;
  }
}

function deriveStormGuardFromSnapshotMetrics(
  metrics: Record<string, number>
): Pick<DeviceTelemetryDetails, 'stormGuardActive' | 'stormGuardEndsAtUnixMs'> | undefined {
  const stormPatternEnable = firstDefinedNumber(
    metrics['param.stormPatternEnable'],
    metrics['params.stormPatternEnable'],
    metrics['param.stormIsEnable'],
    metrics['params.stormIsEnable']
  );
  const stormPatternOpen = firstDefinedNumber(
    metrics['param.stormPatternOpenFlag'],
    metrics['params.stormPatternOpenFlag'],
    metrics['param.inStormMode'],
    metrics['params.inStormMode']
  );
  const stormPatternEndTimeSeconds = firstDefinedNumber(
    metrics['param.stormPatternEndTime'],
    metrics['params.stormPatternEndTime'],
    metrics['param.stormEndTimestamp'],
    metrics['params.stormEndTimestamp']
  );
  if (
    stormPatternEnable === undefined &&
    stormPatternOpen === undefined &&
    stormPatternEndTimeSeconds === undefined
  ) {
    return undefined;
  }
  const stormGuard = deriveStormGuardState({
    open: toBooleanMetric(stormPatternOpen),
    endTimeSeconds: stormPatternEndTimeSeconds
  });
  return {
    stormGuardActive: stormGuard.active,
    stormGuardEndsAtUnixMs: stormGuard.endsAtUnixMs
  };
}

function firstDefinedNumber(...values: Array<number | undefined>): number | undefined {
  for (const value of values) {
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value;
    }
  }
  return undefined;
}

function toBooleanMetric(value: number | undefined): boolean | undefined {
  if (value === undefined) {
    return undefined;
  }
  return value !== 0;
}

function flattenProviderDevices(groups: ProviderDeviceGroup[]): ProviderDevice[] {
  const byDeviceID = new Map<string, ProviderDevice>();
  for (const group of groups) {
    for (const device of group.devices) {
      const existing = byDeviceID.get(device.deviceId);
      if (!existing) {
        byDeviceID.set(device.deviceId, device);
        continue;
      }
      byDeviceID.set(device.deviceId, mergeProviderDevice(existing, device));
    }
  }
  return [...byDeviceID.values()];
}

function mergeProviderDevice(left: ProviderDevice, right: ProviderDevice): ProviderDevice {
  return {
    ...left,
    ...right,
    capabilities: mergeRecord(left.capabilities, right.capabilities),
    metadata: mergeRecord(left.metadata, right.metadata)
  };
}

function mergeRecord(
  left?: Record<string, unknown>,
  right?: Record<string, unknown>
): Record<string, unknown> | undefined {
  if (!left && !right) {
    return undefined;
  }
  return {
    ...(left ?? {}),
    ...(right ?? {})
  };
}

function mapAvailableProviderDevice(device: AvailableProviderDevice): AvailableDeviceSummary {
  return {
    provider: device.provider,
    providerDeviceId: device.providerDeviceId,
    credentialId: device.credentialId,
    serialNumber: device.canonicalSn || device.providerDeviceId,
    name: device.productName || device.model || device.canonicalSn || device.providerDeviceId,
    model: device.model || device.productName || 'Unknown device',
    capabilities: device.capabilities,
    metadata: device.metadata
  };
}

function resolveUserSubject(config: AppConfig, request: FastifyRequest): string {
  if (request.auth?.subject) {
    return request.auth.subject;
  }
  if (config.auth.mode === 'noop') {
    const fromHeader = headerValue(request, 'x-user-subject');
    if (fromHeader) {
      return fromHeader;
    }
    if (config.devUserSubject) {
      return config.devUserSubject;
    }
  }
  throw new Error('missing_user_subject');
}

function getAuthHeader(request: FastifyRequest): string | undefined {
  return headerValue(request, 'authorization');
}

function getRequestID(request: FastifyRequest): string | undefined {
  return headerValue(request, 'x-request-id') ?? request.id;
}

function headerValue(request: FastifyRequest, key: string): string | undefined {
  const value = request.headers[key];
  return typeof value === 'string' && value.trim() ? value : undefined;
}

function parsePositiveInt(value: string | undefined): number | null {
  if (!value) {
    return null;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.min(100, Math.max(0, value));
}

function deriveSummaryPvWatts(rawPvW: number, details?: DeviceTelemetryDetails): number {
  const ports = details?.solarPorts ?? [];
  let sum = 0;
  let found = false;
  let totalMaxWatts = 0;
  for (const port of ports) {
    const maxWatts = sanePositive(port.maxWatts);
    if (maxWatts !== undefined) {
      totalMaxWatts += maxWatts;
    }
    const watts = saneNonNegative(port.watts);
    const volts = saneNonNegative(port.volts);
    const amps = saneNonNegative(port.amps);
    if (watts !== undefined) {
      if (watts > 0) {
        if (maxWatts === undefined || watts <= maxWatts * 2) {
          sum += watts;
          found = true;
        }
        continue;
      }
      if (volts !== undefined && amps !== undefined) {
        const derivedWatts = volts * amps;
        const candidate = derivedWatts > 0 ? derivedWatts : 0;
        if (maxWatts === undefined || candidate <= maxWatts * 2) {
          sum += candidate;
          found = true;
        }
        continue;
      }
      found = true;
      continue;
    }
    if (volts !== undefined && amps !== undefined) {
      const derivedWatts = volts * amps;
      if (maxWatts === undefined || derivedWatts <= maxWatts * 2) {
        sum += derivedWatts;
        found = true;
      }
    }
  }
  const detailPvW = found ? sum : 0;
  const saneRawPvW = sanePositive(rawPvW) ?? 0;
  if (found) {
    if (detailPvW <= 0) {
      return 0;
    }
    if (saneRawPvW <= 0) {
      return detailPvW;
    }
    if (totalMaxWatts > 0 && saneRawPvW > totalMaxWatts * 1.1) {
      return detailPvW;
    }
    const higher = Math.max(saneRawPvW, detailPvW);
    const lower = Math.min(saneRawPvW, detailPvW);
    if (lower > 0 && higher / lower >= 1.5) {
      return detailPvW;
    }
    return saneRawPvW;
  }
  return saneRawPvW;
}

function deriveSummaryEtaMinutes(rawEtaMinutes: number, details?: DeviceTelemetryDetails): number {
  if (Number.isFinite(rawEtaMinutes) && rawEtaMinutes > 0) {
    return Math.round(rawEtaMinutes);
  }
  return deriveDeviceDetailsEtaMinutes(details) ?? 0;
}

function sanePositive(value: number | undefined): number | undefined {
  return value !== undefined && Number.isFinite(value) && value > 0 ? value : undefined;
}

function saneNonNegative(value: number | undefined): number | undefined {
  return value !== undefined && Number.isFinite(value) && value >= 0 ? value : undefined;
}
