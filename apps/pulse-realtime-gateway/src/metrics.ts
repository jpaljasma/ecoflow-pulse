import { Counter, Gauge, Registry } from 'prom-client';

const registry = new Registry();

type WsLogSubscriptionOutcome =
  | 'requested'
  | 'source_unavailable'
  | 'authorized'
  | 'subscribed'
  | 'replay_done'
  | 'source_error'
  | 'status_replay'
  | 'status_live'
  | 'status_forbidden'
  | 'status_error'
  | 'status_closed';

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

const wsLogSubscriptionsTotal = new Counter({
  name: 'pulse_realtime_ws_log_subscriptions_total',
  help: 'Realtime websocket admin log subscription lifecycle outcomes.',
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
  recordLogSubscriptionOutcome(outcome: WsLogSubscriptionOutcome) {
    wsLogSubscriptionsTotal.labels(outcome).inc();
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
