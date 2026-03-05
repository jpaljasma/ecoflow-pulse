import http from 'k6/http';
import { check, sleep } from 'k6';

import { buildRequestHeaders } from './config.js';

export function setupShared(cfg) {
  const headers = buildRequestHeaders(cfg);
  const device = resolveDevice(cfg, headers);

  let seeded = 0;
  for (let i = 0; i < cfg.seedCount; i += 1) {
    const response = http.post(
      cfg.ingestUrl,
      JSON.stringify({
        device_id: device.id,
        serial_number: device.serialNumber,
        observed_unix_ms: Date.now(),
        metrics: seedMetrics(i)
      }),
      {
        headers,
        timeout: cfg.requestTimeout
      }
    );
    if (response.status === 202) {
      seeded += 1;
    }
    sleep(0.1);
  }

  if (seeded === 0) {
    throw new Error('seed ingest failed: no accepted /ingest responses');
  }

  return {
    config: {
      apiBaseUrl: cfg.apiBaseUrl,
      wsUrl: cfg.wsUrl,
      ingestUrl: cfg.ingestUrl,
      requestTimeout: cfg.requestTimeout,
      queryWindowMs: cfg.queryWindowMs,
      queryResolution: cfg.queryResolution,
      wsSessionTimeoutMs: cfg.wsSessionTimeoutMs,
      wsPostTelemetryHoldMs: cfg.wsPostTelemetryHoldMs,
      wsThinkTimeMs: cfg.wsThinkTimeMs,
      headers
    },
    device
  };
}

function resolveDevice(cfg, headers) {
  if (cfg.deviceID && cfg.serialNumber) {
    return {
      id: cfg.deviceID,
      serialNumber: cfg.serialNumber
    };
  }

  const listResponse = http.get(`${cfg.apiBaseUrl}/api/devices`, {
    headers,
    timeout: cfg.requestTimeout
  });

  check(listResponse, {
    'setup /api/devices status is 200': (res) => res.status === 200
  });

  if (listResponse.status !== 200) {
    throw new Error(`/api/devices request failed: status=${listResponse.status} body=${listResponse.body}`);
  }

  const payload = parseJSON(listResponse.body, '/api/devices');
  const devices = Array.isArray(payload.devices) ? payload.devices : [];
  if (devices.length === 0) {
    throw new Error('/api/devices returned zero devices; run local seeding before load tests');
  }

  let selected = devices[0];
  if (cfg.deviceID) {
    selected = devices.find((row) => String(row.id || '').trim() === cfg.deviceID) || null;
    if (!selected) {
      throw new Error(`requested K6_DEVICE_ID not found in /api/devices: ${cfg.deviceID}`);
    }
  }

  const id = String(selected.id || '').trim();
  const serialNumber = String(selected.serialNumber || '').trim();
  if (!id || !serialNumber) {
    throw new Error('selected device is missing id or serialNumber in /api/devices response');
  }

  return { id, serialNumber };
}

function parseJSON(body, label) {
  try {
    return JSON.parse(String(body || '{}'));
  } catch (error) {
    throw new Error(`invalid JSON from ${label}: ${error}`);
  }
}

function seedMetrics(i) {
  const phase = i % 5;
  return {
    soc: 55 + phase,
    pv_w: 220 + phase * 12,
    load_w: 180 + phase * 6,
    ac_w: 35 + phase,
    dc_w: 24 + phase,
    battery_in_w: 260 + phase * 8,
    battery_out_w: 110 + phase * 5,
    temp_c: 24 + phase * 0.4
  };
}
