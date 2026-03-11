import { describe, expect, it } from 'vitest';
import { maskSerialNumber } from '@/features/telemetry/format';

describe('maskSerialNumber', () => {
  it('keeps the first two and last four characters visible', () => {
    expect(maskSerialNumber('DEMOD2M00001057')).toBe('DE••••••1057');
  });

  it('returns short serials unchanged', () => {
    expect(maskSerialNumber('AB1234')).toBe('AB1234');
  });

  it('returns an em dash for missing values', () => {
    expect(maskSerialNumber(undefined)).toBe('—');
    expect(maskSerialNumber('   ')).toBe('—');
  });
});
