import { check, sleep } from 'k6';
import ws from 'k6/ws';
import { Counter, Rate, Trend } from 'k6/metrics';

const wsSessionSuccess = new Rate('ws_session_success');
const wsTelemetryMessages = new Counter('ws_telemetry_messages_total');
const wsFirstTelemetryMs = new Trend('ws_first_telemetry_ms', true);

export function wsFanout(data) {
  const startedAt = Date.now();
  let recordedOutcome = false;

  const response = ws.connect(
    data.config.wsUrl,
    {
      headers: buildWSHeaders(data.config.headers),
      tags: { device_id: data.device.id }
    },
    (socket) => {
      let sawTelemetry = false;

      socket.on('open', () => {
        socket.send(
          JSON.stringify({
            type: 'subscribe',
            deviceIds: [data.device.id]
          })
        );
      });

      socket.on('message', (raw) => {
        const message = parseMessage(raw);
        if (!message || message.type !== 'telemetry' || message.deviceId !== data.device.id) {
          return;
        }
        sawTelemetry = true;
        wsTelemetryMessages.add(1);
        wsFirstTelemetryMs.add(Date.now() - startedAt);
        recordOutcome(true);
        socket.setTimeout(() => {
          socket.close();
        }, data.config.wsPostTelemetryHoldMs);
      });

      socket.on('error', () => {
        recordOutcome(false);
      });

      socket.setTimeout(() => {
        if (!sawTelemetry) {
          recordOutcome(false);
          socket.close();
        }
      }, data.config.wsSessionTimeoutMs);

      socket.on('close', () => {
        if (!recordedOutcome) {
          recordOutcome(sawTelemetry);
        }
      });
    }
  );

  const handshakeOK = check(response, {
    'ws handshake status is 101': (res) => res && res.status === 101
  });
  if (!handshakeOK) {
    recordOutcome(false);
  }
  sleep(data.config.wsThinkTimeMs / 1000);

  function recordOutcome(ok) {
    if (recordedOutcome) {
      return;
    }
    wsSessionSuccess.add(ok ? 1 : 0);
    recordedOutcome = true;
  }
}

function parseMessage(raw) {
  try {
    return JSON.parse(String(raw || ''));
  } catch {
    return null;
  }
}

function buildWSHeaders(headers) {
  const out = {};
  for (const [key, value] of Object.entries(headers)) {
    if (key.toLowerCase() === 'content-type') {
      continue;
    }
    out[key] = value;
  }
  return out;
}
