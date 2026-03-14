import { Counter, Histogram, Registry } from 'prom-client';

const registry = new Registry();

const httpRequestsTotal = new Counter({
  name: 'pulse_public_http_requests_total',
  help: 'Total public app HTTP requests by classified route, method, and status class.',
  labelNames: ['route', 'method', 'status_class'] as const,
  registers: [registry]
});

const httpRequestDurationSeconds = new Histogram({
  name: 'pulse_public_http_request_duration_seconds',
  help: 'Public app HTTP request duration by classified route and method.',
  labelNames: ['route', 'method'] as const,
  buckets: [0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5],
  registers: [registry]
});

const authDeniedTotal = new Counter({
  name: 'pulse_public_auth_denials_total',
  help: 'Public app authenticated authorization denials by classified route and status.',
  labelNames: ['route', 'status'] as const,
  registers: [registry]
});

export function classifyPublicPathname(pathname: string): string | null {
  switch (pathname) {
    case '/':
    case '/login':
    case '/onboarding':
    case '/profile':
    case '/devices':
    case '/energy':
    case '/settings':
      return pathname;
    case '/api/v1/me':
      return pathname;
    case '/api/v1/energy/dashboard':
    case '/api/v1/energy/pv-history':
    case '/api/v1/energy/comparison-insight':
      return pathname;
    case '/api/v1/devices':
      return pathname;
    default:
      break;
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
  if (/^\/device\/[^/]+$/.test(pathname)) {
    return '/device/:deviceId';
  }
  if (pathname.startsWith('/api/')) {
    return '/api/other';
  }
  if (pathname === '/metrics' || pathname === '/healthz') {
    return null;
  }
  return '/app/other';
}

export function observePublicRequest(input: {
  pathname: string;
  method: string;
  statusCode: number;
  durationSeconds: number;
}): void {
  const route = classifyPublicPathname(input.pathname);
  if (!route) {
    return;
  }
  const method = input.method.toUpperCase();
  const statusClass = `${Math.floor(input.statusCode / 100)}xx`;
  httpRequestsTotal.labels(route, method, statusClass).inc();
  httpRequestDurationSeconds.labels(route, method).observe(Math.max(0, input.durationSeconds));
  if (input.statusCode === 401 || input.statusCode === 403) {
    authDeniedTotal.labels(route, String(input.statusCode)).inc();
  }
}

export function resetPublicMetrics(): void {
  registry.resetMetrics();
}

export async function renderPublicMetrics(): Promise<string> {
  return await registry.metrics();
}

export function publicMetricsContentType(): string {
  return registry.contentType;
}
