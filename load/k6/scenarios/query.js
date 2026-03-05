import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const querySuccess = new Counter('query_success_total');

export function historyQuery(data) {
  const toUnixMs = Date.now();
  const fromUnixMs = toUnixMs - data.config.queryWindowMs;
  const query = `${data.config.apiBaseUrl}/api/v1/devices/${encodeURIComponent(data.device.id)}/history?resolution=${encodeURIComponent(
    data.config.queryResolution
  )}&from=${fromUnixMs}&to=${toUnixMs}`;

  const response = http.get(query, {
    headers: data.config.headers,
    timeout: data.config.requestTimeout
  });

  const ok = check(response, {
    'history query status is 200': (res) => res.status === 200,
    'history query returns JSON': (res) => isJSONBody(res.body)
  });

  if (ok) {
    querySuccess.add(1);
  }
}

function isJSONBody(body) {
  try {
    JSON.parse(String(body || '{}'));
    return true;
  } catch {
    return false;
  }
}
