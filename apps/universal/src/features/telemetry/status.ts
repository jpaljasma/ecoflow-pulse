import type { TelemetryEngineStatus } from '@/features/telemetry/engine/types';

export function formatConnectionStatus(status: TelemetryEngineStatus): string {
  switch (status) {
    case 'auth_required':
      return 'Sign in required for live telemetry';
    case 'connecting':
      return 'Connecting to realtime telemetry';
    case 'reconnecting':
      return 'Reconnecting to realtime telemetry';
    case 'connected':
      return 'Realtime telemetry live';
    case 'disconnected':
      return 'Realtime telemetry offline';
    case 'idle':
    default:
      return 'Waiting for realtime telemetry';
  }
}
