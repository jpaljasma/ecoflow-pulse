import { describe, expect, it } from 'vitest';

import { formatAvailableDeviceActionError } from '@/features/devices/actionMessages';

describe('formatAvailableDeviceActionError', () => {
  it('surfaces upstream API messages for device enable failures', () => {
    const error = {
      message: 'Request failed (412) for POST /api/v1/devices/available/enable',
      body: {
        message: 'successful mqtt probe required before enablement: timeout'
      }
    };

    expect(formatAvailableDeviceActionError('Enable device', error)).toBe(
      'Enable device failed: successful mqtt probe required before enablement: timeout'
    );
  });

  it('falls back to validation issue text when the response has zod issues', () => {
    const error = {
      message: 'Request failed (400) for POST /api/v1/devices/available/enable',
      body: {
        issues: [{ message: 'providerDeviceId is required' }]
      }
    };

    expect(formatAvailableDeviceActionError('Enable device', error)).toBe(
      'Enable device failed: providerDeviceId is required'
    );
  });
});
