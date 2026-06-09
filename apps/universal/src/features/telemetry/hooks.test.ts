import { describe, expect, it, vi } from 'vitest';

vi.mock('@/features/telemetry/TelemetryEngineContext', () => ({
  useTelemetryEngine: vi.fn()
}));

import {
  normalizeTelemetryDeviceIds,
  resolveTelemetrySubscriptionDeviceIds
} from '@/features/telemetry/hooks';

describe('telemetry subscription hooks', () => {
  it('normalizes device ids for stable realtime subscriptions', () => {
    expect(normalizeTelemetryDeviceIds(['device-b', 'device-a', 'device-b'])).toEqual([
      'device-a',
      'device-b'
    ]);
  });

  it('returns an empty subscription while the owning route is inactive', () => {
    expect(
      resolveTelemetrySubscriptionDeviceIds(['device-b', 'device-a'], { active: false })
    ).toEqual([]);
  });

  it('keeps the stable subscription while the owning route is active', () => {
    expect(
      resolveTelemetrySubscriptionDeviceIds(['device-b', 'device-a'], { active: true })
    ).toEqual(['device-a', 'device-b']);
  });
});
