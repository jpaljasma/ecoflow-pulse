import { describe, expect, it } from 'vitest';

import { getStatusIconName } from '@/shared/ui/statusGlyph';

describe('status glyphs', () => {
  it('uses a clear offline icon for device telemetry cutoffs', () => {
    expect(getStatusIconName('offline')).toBe('cloud-off-outline');
  });
});
