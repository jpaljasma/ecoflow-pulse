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

const authSessionRecoveryTotal = new Counter({
  name: 'pulse_public_auth_session_recovery_total',
  help: 'Client-reported auth session recovery outcomes from the universal app.',
  labelNames: ['outcome'] as const,
  registers: [registry]
});

const clientRestRequestsTotal = new Counter({
  name: 'pulse_public_client_rest_requests_total',
  help: 'Client-observed REST request outcomes from the universal app by canonical route, method, outcome, and status class.',
  labelNames: ['route', 'method', 'outcome', 'status_class'] as const,
  registers: [registry]
});

const clientRestRequestDurationSeconds = new Histogram({
  name: 'pulse_public_client_rest_request_duration_seconds',
  help: 'Client-observed REST request duration from the universal app by canonical route, method, and outcome.',
  labelNames: ['route', 'method', 'outcome'] as const,
  buckets: [0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10],
  registers: [registry]
});

const clientRestErrorsTotal = new Counter({
  name: 'pulse_public_client_rest_errors_total',
  help: 'Client-observed REST request errors from the universal app by canonical route, method, and error kind.',
  labelNames: ['route', 'method', 'error_kind'] as const,
  registers: [registry]
});

const clientWsConnectionsTotal = new Counter({
  name: 'pulse_public_client_ws_connections_total',
  help: 'Client-observed websocket connection outcomes from the universal app by phase and outcome.',
  labelNames: ['phase', 'outcome'] as const,
  registers: [registry]
});

const clientWsConnectDurationSeconds = new Histogram({
  name: 'pulse_public_client_ws_connect_duration_seconds',
  help: 'Client-observed websocket connect and reconnect duration from the universal app by phase and outcome.',
  labelNames: ['phase', 'outcome'] as const,
  buckets: [0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30],
  registers: [registry]
});

const clientWsDisconnectsTotal = new Counter({
  name: 'pulse_public_client_ws_disconnects_total',
  help: 'Client-observed websocket disconnect reasons from the universal app.',
  labelNames: ['reason'] as const,
  registers: [registry]
});

const clientWsFreshnessTransitionsTotal = new Counter({
  name: 'pulse_public_client_ws_freshness_transitions_total',
  help: 'Client-observed websocket freshness-state transitions from the universal app.',
  labelNames: ['state'] as const,
  registers: [registry]
});

const clientWsStaleRecoveryDurationSeconds = new Histogram({
  name: 'pulse_public_client_ws_stale_recovery_duration_seconds',
  help: 'Client-observed websocket stale-recovery duration from the universal app.',
  buckets: [0.1, 0.5, 1, 2, 5, 10, 30, 60, 120],
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
    case '/energy-calendar':
    case '/settings':
    case '/settings/integrations':
      return pathname;
    case '/api/v1/me':
    case '/api/v1/integrations':
    case '/api/v1/solar/outlook':
      return pathname;
    case '/api/v1/energy/dashboard':
    case '/api/v1/energy/calendar':
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
  if (/^\/api\/v1\/integrations\/[^/]+\/active$/.test(pathname)) {
    return '/api/v1/integrations/:credentialId/active';
  }
  if (/^\/api\/v1\/integrations\/[^/]+$/.test(pathname)) {
    return '/api/v1/integrations/:credentialId';
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

export function observePublicRequest(input: { pathname: string; method: string; statusCode: number; durationSeconds: number }): void {
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

export type AuthSessionRecoveryOutcome = 'recovered_in_memory' | 'recovered_refresh' | 'reauth_redirect';

export function observeAuthSessionRecovery(outcome: AuthSessionRecoveryOutcome): void {
  authSessionRecoveryTotal.labels(outcome).inc();
}

export type ClientRestOutcome = 'success' | 'http_error' | 'network_error' | 'client_error';

export function observeClientRestRequest(input: {
  route: string;
  method: string;
  outcome: ClientRestOutcome;
  statusClass: 'none' | '2xx' | '3xx' | '4xx' | '5xx';
  durationSeconds: number;
  errorKind: string;
}): void {
  const method = input.method.toUpperCase();
  clientRestRequestsTotal.labels(input.route, method, input.outcome, input.statusClass).inc();
  clientRestRequestDurationSeconds.labels(input.route, method, input.outcome).observe(Math.max(0, input.durationSeconds));
  if (input.errorKind !== 'none') {
    clientRestErrorsTotal.labels(input.route, method, input.errorKind).inc();
  }
}

export function observeClientWsConnection(input: {
  phase: 'initial' | 'reconnect';
  outcome: 'connected' | 'auth_required' | 'connect_error' | 'closed_before_open';
  durationSeconds: number;
}): void {
  clientWsConnectionsTotal.labels(input.phase, input.outcome).inc();
  clientWsConnectDurationSeconds.labels(input.phase, input.outcome).observe(Math.max(0, input.durationSeconds));
}

export function observeClientWsDisconnect(
  reason: 'unexpected_close' | 'socket_error' | 'stalled' | 'manual_disconnect' | 'auth_required'
): void {
  clientWsDisconnectsTotal.labels(reason).inc();
}

export function observeClientWsFreshnessTransition(state: 'stale' | 'fresh'): void {
  clientWsFreshnessTransitionsTotal.labels(state).inc();
}

export function observeClientWsStaleRecoveryDuration(durationSeconds: number): void {
  clientWsStaleRecoveryDurationSeconds.observe(Math.max(0, durationSeconds));
}

export async function renderPublicMetrics(): Promise<string> {
  return await registry.metrics();
}

export function publicMetricsContentType(): string {
  return registry.contentType;
}
