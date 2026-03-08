import { describe, expect, it } from 'vitest';
import { maskSerialNumber } from '@/features/telemetry/format';

describe('maskSerialNumber', () => {
  it('keeps the first two and last four characters visible', () => {
    expect(maskSerialNumber('R351ZABAPH331057')).toBe('R3••••••1057');
  });

  it('returns short serials unchanged', () => {
    expect(maskSerialNumber('AB1234')).toBe('AB1234');
  });

  it('returns an em dash for missing values', () => {
    expect(maskSerialNumber(undefined)).toBe('—');
    expect(maskSerialNumber('   ')).toBe('—');
  });
});
