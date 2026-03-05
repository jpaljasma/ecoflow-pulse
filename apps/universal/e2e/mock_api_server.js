#!/usr/bin/env node
/* global URL, process, require */

const http = require('http');

const host = process.env.MAESTRO_MOCK_API_HOST || '127.0.0.1';
const port = Number(process.env.MAESTRO_MOCK_API_PORT || '18081');

const devices = [
  {
    id: '019cab9d-bcb3-7587-8dc9-9a57deb48d30',
    serialNumber: 'Y711ZABA9H2P0294',
    name: 'DPU A 12 kWh',
    model: 'DELTA Pro Ultra',
    online: true,
    batteryPct: 24.6,
    state: 'charging',
    etaMinutes: 263,
    pvW: 356,
    acInW: 0,
    dcW: 0,
    loadW: 118,
    netW: 238,
    tempC: 24.1,
    telemetryTsMs: Date.UTC(2026, 2, 5, 11, 0, 0)
  },
  {
    id: '019cab9d-bcab-75c0-9c02-db3ae1105d61',
    serialNumber: 'R351ZABAPH331057',
    name: 'Kitchen Delta 2 Max',
    model: 'DELTA 2 Max',
    online: true,
    batteryPct: 31.8,
    state: 'discharging',
    etaMinutes: 419,
    pvW: 0,
    acInW: 132,
    dcW: 0,
    loadW: 143,
    netW: -11,
    tempC: 20.2,
    telemetryTsMs: Date.UTC(2026, 2, 5, 11, 0, 0)
  }
];

const byKey = new Map();
for (const device of devices) {
  byKey.set(device.id, device);
  byKey.set(device.serialNumber, device);
}

function writeJson(res, statusCode, payload) {
  res.writeHead(statusCode, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(payload));
}

const server = http.createServer((req, res) => {
  if (!req.url) {
    writeJson(res, 400, { error: 'missing_url' });
    return;
  }

  const parsed = new URL(req.url, `http://${host}:${port}`);
  const pathname = parsed.pathname;

  if (pathname === '/healthz') {
    writeJson(res, 200, { ok: true });
    return;
  }

  if (pathname === '/api/devices') {
    writeJson(res, 200, { devices });
    return;
  }

  if (pathname.startsWith('/api/devices/')) {
    const key = decodeURIComponent(pathname.slice('/api/devices/'.length));
    const device = byKey.get(key);
    if (!device) {
      writeJson(res, 404, { error: 'not_found' });
      return;
    }
    writeJson(res, 200, device);
    return;
  }

  writeJson(res, 404, { error: `unhandled_path:${pathname}` });
});

server.listen(port, host, () => {
  // Keep output terse and machine-friendly for runner logs.
  process.stdout.write(`mock_api_ready host=${host} port=${port}\n`);
});
