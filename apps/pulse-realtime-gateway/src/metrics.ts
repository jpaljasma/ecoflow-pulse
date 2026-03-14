import { Counter, Gauge, Registry } from 'prom-client';

const registry = new Registry();

const wsAuthTotal = new Counter({
  name: 'pulse_realtime_ws_auth_total',
  help: 'Realtime websocket authentication outcomes.',
  labelNames: ['outcome'] as const,
  registers: [registry]
});

const wsSessionsActive = new Gauge({
  name: 'pulse_realtime_ws_sessions_active',
  help: 'Currently active websocket sessions in the realtime gateway.',
  registers: [registry]
});

const wsSubscriptionsTotal = new Counter({
  name: 'pulse_realtime_ws_subscriptions_total',
  help: 'Realtime websocket subscription outcomes.',
  labelNames: ['outcome'] as const,
  registers: [registry]
});

export const realtimeMetrics = {
  recordAuthOutcome(outcome: 'accepted' | 'missing_bearer_token' | 'invalid_bearer_token' | 'anonymous_allowed') {
    wsAuthTotal.labels(outcome).inc();
  },
  sessionOpened() {
    wsSessionsActive.inc();
  },
  sessionClosed() {
    wsSessionsActive.dec();
  },
  recordSubscriptionOutcome(outcome: 'requested' | 'forbidden') {
    wsSubscriptionsTotal.labels(outcome).inc();
  },
  contentType() {
    return registry.contentType;
  },
  async render() {
    return await registry.metrics();
  },
  reset() {
    registry.resetMetrics();
  }
};
