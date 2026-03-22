import { env } from '@/shared/config/env';

export type ClientRestOutcome = 'success' | 'http_error' | 'network_error' | 'client_error';
export type ClientRestMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
export type ClientRestStatusClass = 'none' | '2xx' | '3xx' | '4xx' | '5xx';

export type ClientRestMetricEvent = {
  route: string;
  method: ClientRestMethod;
  outcome: ClientRestOutcome;
  statusClass: ClientRestStatusClass;
  durationMs: number;
  errorKind: string;
};

export function classifyClientRestPath(path: string): string | null {
  let pathname = path;
  try {
    pathname = new URL(path, env.apiUrl).pathname;
  } catch {
    pathname = path.split('?')[0] ?? path;
  }

  switch (pathname) {
    case '/api/devices':
      return pathname;
    case '/api/v1/devices':
    case '/api/v1/me':
    case '/api/v1/me/identity-refresh':
    case '/api/v1/energy/dashboard':
    case '/api/v1/energy/pv-history':
    case '/api/v1/energy/comparison-insight':
    case '/api/v1/solar/outlook':
    case '/api/v1/devices/available':
    case '/api/v1/devices/available/test-mqtt':
    case '/api/v1/devices/available/enable':
      return pathname;
    default:
      break;
  }

  if (/^\/api\/v1\/devices\/[^/]+\/history\/compare$/.test(pathname)) {
    return '/api/v1/devices/:deviceId/history/compare';
  }
  if (/^\/api\/v1\/devices\/[^/]+\/history\/solar$/.test(pathname)) {
    return '/api/v1/devices/:deviceId/history/solar';
  }
  if (/^\/api\/v1\/devices\/[^/]+\/history$/.test(pathname)) {
    return '/api/v1/devices/:deviceId/history';
  }
  if (/^\/api\/v1\/devices\/[^/]+\/insights$/.test(pathname)) {
    return '/api/v1/devices/:deviceId/insights';
  }
  if (/^\/api\/v1\/devices\/[^/]+$/.test(pathname)) {
    return '/api/v1/devices/:deviceId';
  }
  if (/^\/api\/devices\/[^/]+$/.test(pathname)) {
    return '/api/devices/:deviceId';
  }
  if (pathname.startsWith('/api/v1/auth/session-events')) {
    return null;
  }
  if (pathname.startsWith('/api/v1/client-metrics')) {
    return null;
  }
  if (pathname.startsWith('/api/')) {
    return '/api/other';
  }
  return null;
}

export function toStatusClass(status: number | null | undefined): ClientRestStatusClass {
  if (!status || status <= 0) {
    return 'none';
  }
  return `${Math.floor(status / 100)}xx` as ClientRestStatusClass;
}

export function toErrorKind(input: {
  outcome: ClientRestOutcome;
  status?: number;
}): string {
  if (input.outcome === 'success') {
    return 'none';
  }
  if (input.outcome === 'network_error') {
    return 'network_failure';
  }
  if (input.outcome === 'client_error') {
    return 'client_response_handling';
  }
  if ((input.status ?? 0) === 401) {
    return 'status_401';
  }
  if ((input.status ?? 0) === 403) {
    return 'status_403';
  }
  if ((input.status ?? 0) >= 500) {
    return 'status_5xx';
  }
  if ((input.status ?? 0) >= 400) {
    return 'status_4xx';
  }
  return 'unknown';
}

export async function reportClientRestMetric(event: ClientRestMetricEvent): Promise<void> {
  const url = `${env.apiUrl}/api/v1/client-metrics/rest`;
  const payload = JSON.stringify(event);
  try {
    if (
      env.isWeb &&
      typeof navigator !== 'undefined' &&
      typeof navigator.sendBeacon === 'function' &&
      typeof Blob !== 'undefined'
    ) {
      navigator.sendBeacon(url, new Blob([payload], { type: 'application/json' }));
      return;
    }
    await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json'
      },
      body: payload,
      keepalive: true
    });
  } catch {
    // Best-effort client metric reporting should never affect user traffic.
  }
}
