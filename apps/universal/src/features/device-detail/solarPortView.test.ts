import { describe, expect, it } from 'vitest';

import { solarPortView } from '@/features/device-detail/solarPort';

describe('solarPortView', () => {
  it('treats positive solar watts as active even if stale volts and state still read inactive', () => {
    const port = solarPortView({
      id: 'pv-low',
      name: 'PV Low',
      state: 'inactive',
      volts: 0,
      amps: 0,
      watts: 121,
      maxVolts: 150,
      maxAmps: 15,
      maxWatts: 1600
    });

    expect(port.inactive).toBe(false);
    expect(port.stateLabel).toBe('active');
    expect(port.stateTone).toBe('neutral');
  });

  it('keeps charging when live volts amps and watts all indicate active solar flow', () => {
    const port = solarPortView({
      id: 'pv-low',
      name: 'PV Low',
      state: 'charging',
      volts: 68.59,
      amps: 1.93,
      watts: 132,
      maxVolts: 150,
      maxAmps: 15,
      maxWatts: 1600
    });

    expect(port.inactive).toBe(false);
    expect(port.stateLabel).toBe('charging');
    expect(port.stateTone).toBe('success');
  });
});
