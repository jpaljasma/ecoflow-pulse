import { describe, expect, it } from 'vitest';

import { normalizeEdgeDeviceSourceStatus } from '../src/grpc/edgeClient.js';

describe('edge gRPC client normalization', () => {
  it('preserves unknown device source statuses without reporting pending', () => {
    expect(normalizeEdgeDeviceSourceStatus('quarantined')).toEqual({
      status: 'unknown',
      rawStatus: 'quarantined'
    });
  });

  it('normalizes empty device source statuses to unknown without a raw status', () => {
    expect(normalizeEdgeDeviceSourceStatus('')).toEqual({ status: 'unknown' });
    expect(normalizeEdgeDeviceSourceStatus(undefined)).toEqual({ status: 'unknown' });
  });
});
