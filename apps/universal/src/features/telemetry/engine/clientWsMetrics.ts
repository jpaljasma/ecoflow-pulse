import { env } from '@/shared/config/env';

export type ClientWsPhase = 'initial' | 'reconnect';
export type ClientWsConnectionOutcome =
  | 'connected'
  | 'auth_required'
  | 'connect_error'
  | 'closed_before_open';
export type ClientWsDisconnectReason =
  | 'unexpected_close'
  | 'socket_error'
  | 'stalled'
  | 'manual_disconnect'
  | 'auth_required';
export type ClientWsFreshnessState = 'stale' | 'fresh';

type ClientWsConnectionMetricEvent = {
  eventType: 'connection';
  phase: ClientWsPhase;
  outcome: ClientWsConnectionOutcome;
  durationMs: number;
};

type ClientWsDisconnectMetricEvent = {
  eventType: 'disconnect';
  reason: ClientWsDisconnectReason;
};

type ClientWsFreshnessTransitionMetricEvent = {
  eventType: 'freshness_transition';
  state: ClientWsFreshnessState;
};

type ClientWsStaleRecoveryMetricEvent = {
  eventType: 'stale_recovery';
  durationMs: number;
};

export type ClientWsMetricEvent =
  | ClientWsConnectionMetricEvent
  | ClientWsDisconnectMetricEvent
  | ClientWsFreshnessTransitionMetricEvent
  | ClientWsStaleRecoveryMetricEvent;

export async function reportClientWsMetric(event: ClientWsMetricEvent): Promise<void> {
  const url = `${env.apiUrl}/api/v1/client-metrics/ws`;
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
    // Best-effort client metric reporting should never affect live telemetry.
  }
}
