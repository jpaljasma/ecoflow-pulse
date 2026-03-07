import type { FastifyRequest } from 'fastify';

import type { AppConfig } from '../config.js';
import type { ControlPlaneClient, ProviderDevice, ProviderDeviceGroup } from './controlPlaneClient.js';
import type { TelemetrySnapshotClient } from './telemetryClient.js';
import { deriveTelemetryMetrics, deriveTelemetryState, deriveTelemetryEtaMinutes } from '../telemetry/deriveMetrics.js';
import {
  buildProviderDevicePresentation,
  type DeviceCapabilities,
  type DeviceTelemetryDetails
} from '../devices/providerDeviceMapper.js';

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

export interface DeviceClient {
  listDevices(request: FastifyRequest): Promise<DeviceSummary[]>;
  getDevice(request: FastifyRequest, routeDeviceId: string): Promise<DeviceSummary | null>;
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
    close() {
      controlPlaneClient.close();
      telemetryClient.close();
    }
  };
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
    const derived = deriveTelemetryMetrics(rawMetrics);
    const pvW = deriveSummaryPvWatts(derived.pvW, presentation.details);
    const telemetryTsMs = parsePositiveInt(response.snapshot?.cursor?.tsUnixMs);
    return {
      ...base,
      online: telemetryTsMs !== null ? Date.now() - telemetryTsMs <= 30_000 : false,
      batteryPct: clampPercent(presentation.details?.overallSocPct ?? derived.soc),
      state: deriveTelemetryState(derived.batteryW),
      etaMinutes: deriveTelemetryEtaMinutes(rawMetrics, derived.batteryW),
      pvW,
      acInW: derived.acW,
      dcW: derived.dcW,
      loadW: derived.loadW,
      netW: pvW - derived.loadW,
      tempC: derived.tempC,
      telemetryTsMs: telemetryTsMs ?? undefined
    };
  } catch {
    return base;
  }
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

function sanePositive(value: number | undefined): number | undefined {
  return value !== undefined && Number.isFinite(value) && value > 0 ? value : undefined;
}

function saneNonNegative(value: number | undefined): number | undefined {
  return value !== undefined && Number.isFinite(value) && value >= 0 ? value : undefined;
}
