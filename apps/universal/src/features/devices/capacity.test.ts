import { describe, expect, it } from 'vitest';

import { getCapacityKWh } from '@/features/devices/capacity';

describe('device capacity', () => {
  it('uses explicit capability capacity when present', () => {
    expect(
      getCapacityKWh({
        model: 'DELTA Pro Ultra X',
        capabilities: {
          batteryCapacityKWh: 24.576
        }
      })
    ).toBe(24.576);
  });

  it('falls back to the Ultra X default base capacity', () => {
    expect(
      getCapacityKWh({
        model: 'DELTA Pro Ultra X',
        capabilities: {}
      })
    ).toBe(12.288);
  });
});

