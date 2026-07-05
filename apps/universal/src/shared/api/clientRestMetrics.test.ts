import { describe, expect, it, vi } from 'vitest';

vi.mock('@/shared/config/env', () => ({
  env: {
    apiUrl: 'http://localhost:18081'
  }
}));

import { classifyClientRestPath } from '@/shared/api/clientRestMetrics';

describe('classifyClientRestPath', () => {
  it('classifies EcoFlow BLE auth and edge collector routes', () => {
    expect(
      classifyClientRestPath('/api/v1/integrations/ecoflow-ble-auth')
    ).toBe('/api/v1/integrations/ecoflow-ble-auth');
    expect(
      classifyClientRestPath('/api/v1/integrations/ecoflow-ble-auth/connect')
    ).toBe('/api/v1/integrations/ecoflow-ble-auth/connect');
    expect(
      classifyClientRestPath('/api/v1/integrations/ecoflow-ble-auth/manual')
    ).toBe('/api/v1/integrations/ecoflow-ble-auth/manual');
    expect(
      classifyClientRestPath('/api/v1/edge/device-sources?status=pending')
    ).toBe('/api/v1/edge/device-sources');
    expect(
      classifyClientRestPath('/api/v1/edge/device-sources/source-123/approve')
    ).toBe('/api/v1/edge/device-sources/:sourceId/approve');
    expect(
      classifyClientRestPath(
        '/api/v1/edge/collectors/collector-123/revoke-setup-token'
      )
    ).toBe('/api/v1/edge/collectors/:collectorId/revoke-setup-token');
  });
});
