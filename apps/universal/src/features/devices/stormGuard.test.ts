import { describe, expect, it } from 'vitest';

import { buildStormGuardLabel } from '@/features/devices/stormGuard';

describe('buildStormGuardLabel', () => {
  it('returns null when Storm Guard is inactive', () => {
    expect(buildStormGuardLabel({ stormGuardActive: false })).toBeNull();
  });

  it('formats the remaining active window when an end time exists', () => {
    expect(
      buildStormGuardLabel(
        {
          stormGuardActive: true,
          stormGuardEndsAtUnixMs: 5 * 60 * 60 * 1000
        },
        3 * 60 * 60 * 1000
      )
    ).toBe('Storm Guard ~2h more');
  });

  it('falls back to a generic active label without a future end time', () => {
    expect(buildStormGuardLabel({ stormGuardActive: true })).toBe('Storm Guard active');
  });
});
