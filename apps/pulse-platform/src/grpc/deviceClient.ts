import type { FastifyRequest } from 'fastify';

import type { AppConfig } from '../config.js';
import type { ControlPlaneClient, UserDevice } from './controlPlaneClient.js';
import type { TelemetrySnapshotClient } from './telemetryClient.js';
import { deriveTelemetryMetrics, deriveTelemetryState, deriveTelemetryEtaMinutes } from '../telemetry/deriveMetrics.js';

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
      const devices = await controlPlaneClient.listUserDevices({
        userSubject,
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: config.grpcDeadlineMs
      });
      const hydrated = await Promise.all(devices.map((device) => hydrateDevice(device, request, config, telemetryClient)));
      return hydrated;
    },
    async getDevice(request, routeDeviceId) {
      const devices = await this.listDevices(request);
      return (
        devices.find((device) => device.id === routeDeviceId || device.serialNumber === routeDeviceId) ?? null
      );
    },
    close() {
      controlPlaneClient.close();
      telemetryClient.close();
    }
  };
}

async function hydrateDevice(
  device: UserDevice,
  request: FastifyRequest,
  config: AppConfig,
  telemetryClient: TelemetrySnapshotClient
): Promise<DeviceSummary> {
  const base = baseDeviceSummary(device);
  try {
    const response = await telemetryClient.getSnapshot({
      deviceId: device.deviceId,
      authHeader: getAuthHeader(request),
      requestID: getRequestID(request),
      deadlineMs: config.grpcDeadlineMs
    });
    const rawMetrics = response.snapshot?.metrics ?? {};
    const derived = deriveTelemetryMetrics(rawMetrics);
    const telemetryTsMs = parsePositiveInt(response.snapshot?.cursor?.tsUnixMs);
    return {
      ...base,
      online: telemetryTsMs !== null ? Date.now()-(telemetryTsMs) <= 30_000 : false,
      batteryPct: clampPercent(derived.soc),
      state: deriveTelemetryState(derived.batteryW),
      etaMinutes: deriveTelemetryEtaMinutes(rawMetrics, derived.batteryW),
      pvW: derived.pvW,
      acInW: derived.acW,
      dcW: derived.dcW,
      loadW: derived.loadW,
      netW: derived.pvW - derived.loadW,
      tempC: derived.tempC,
      telemetryTsMs: telemetryTsMs ?? undefined
    };
  } catch {
    return base;
  }
}

function baseDeviceSummary(device: UserDevice): DeviceSummary {
  return {
    id: device.deviceId,
    serialNumber: device.ecoflowSn,
    name: device.productName || device.model || device.ecoflowSn || device.deviceId,
    model: device.model || device.productName || 'Unknown EcoFlow',
    online: false,
    batteryPct: 0,
    state: 'idle',
    etaMinutes: 0
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
