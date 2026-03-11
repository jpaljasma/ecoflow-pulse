#!/usr/bin/env node
/* global clearInterval, process, require, setInterval */

const { WebSocketServer } = require('ws');

const host = process.env.MAESTRO_MOCK_WS_HOST || '127.0.0.1';
const port = Number(process.env.MAESTRO_MOCK_WS_PORT || '8082');

const DEFAULT_DEVICE_IDS = [
  '11111111-1111-7111-8111-111111111111',
  '22222222-2222-7222-8222-222222222222'
];

function nowUnixMs() {
  return Date.now();
}

function telemetryMessage(deviceId, index) {
  const ts = nowUnixMs();
  const base = deviceId.endsWith('d30') ? 25 : 32;
  return {
    type: 'telemetry',
    deviceId,
    ts,
    metrics: {
      soc: Math.max(5, Math.min(100, base - (index % 5))),
      pvW: deviceId.endsWith('d30') ? 356 : 0,
      loadW: deviceId.endsWith('d30') ? 118 : 143,
      batteryW: deviceId.endsWith('d30') ? 238 : -11,
      tempC: deviceId.endsWith('d30') ? 24.1 : 20.2,
      acW: deviceId.endsWith('d30') ? 0 : 132,
      dcW: 0
    }
  };
}

function statusMessage(deviceId) {
  return {
    type: 'device_status',
    deviceId,
    ts: nowUnixMs(),
    online: true
  };
}

const wss = new WebSocketServer({
  host,
  port,
  path: '/ws'
});

wss.on('connection', (socket) => {
  let subscribedIds = [...DEFAULT_DEVICE_IDS];
  let tick = 0;

  const sendFrame = (frame) => {
    if (socket.readyState !== socket.OPEN) return;
    socket.send(JSON.stringify(frame));
  };

  const publish = () => {
    for (const deviceId of subscribedIds) {
      sendFrame(statusMessage(deviceId));
      sendFrame(telemetryMessage(deviceId, tick));
    }
    tick += 1;
  };

  publish();
  const interval = setInterval(publish, 1000);

  socket.on('message', (raw) => {
    try {
      const parsed = JSON.parse(String(raw));
      if (parsed?.type === 'subscribe' && Array.isArray(parsed.deviceIds)) {
        const validIds = parsed.deviceIds.filter((value) => typeof value === 'string');
        if (validIds.length > 0) {
          subscribedIds = validIds;
        }
      }
    } catch {
      // Ignore malformed client messages in mock server.
    }
  });

  socket.on('close', () => {
    clearInterval(interval);
  });
});

process.stdout.write(`mock_ws_ready host=${host} port=${port}\n`);
