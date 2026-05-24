import { describe, expect, it } from 'vitest';
import {
  getRollingMetricDirection,
  getRollingMetricDigitTiming,
  parseRollingMetricNumber,
  ROLLING_METRIC_BASE_DURATION_MS,
  tokenizeRollingMetricValue
} from '@/shared/ui/RollingMetricValueModel';

describe('RollingMetricValue model', () => {
  it('tokenizes formatted power values into rolling digits and static suffix text', () => {
    expect(tokenizeRollingMetricValue('415 W')).toEqual([
      { kind: 'digit', value: '4' },
      { kind: 'digit', value: '1' },
      { kind: 'digit', value: '5' },
      { kind: 'static', value: ' W' }
    ]);
  });

  it('keeps decimals, signs, units, and placeholders static', () => {
    expect(tokenizeRollingMetricValue('-11.74 kWh')).toEqual([
      { kind: 'static', value: '-' },
      { kind: 'digit', value: '1' },
      { kind: 'digit', value: '1' },
      { kind: 'static', value: '.' },
      { kind: 'digit', value: '7' },
      { kind: 'digit', value: '4' },
      { kind: 'static', value: ' kWh' }
    ]);
    expect(tokenizeRollingMetricValue('—')).toEqual([{ kind: 'static', value: '—' }]);
  });

  it('parses the displayed numeric portion for directional rolling', () => {
    expect(parseRollingMetricNumber('11.74 kWh')).toBe(11.74);
    expect(parseRollingMetricNumber('-119 W')).toBe(-119);
    expect(parseRollingMetricNumber('—')).toBeNull();
  });

  it('rolls digits up or down from the numeric delta', () => {
    expect(getRollingMetricDirection('415 W', '534 W')).toBe('up');
    expect(getRollingMetricDirection('53.5%', '21.5%')).toBe('down');
    expect(getRollingMetricDirection('—', '0 W')).toBe('none');
    expect(getRollingMetricDirection('11.74 kWh', '11.74 kWh')).toBe('none');
  });

  it('slows rolling digits progressively from left to right with overlap', () => {
    expect(ROLLING_METRIC_BASE_DURATION_MS).toBe(528);
    expect(getRollingMetricDigitTiming(0)).toEqual({ delayMs: 0, durationMs: 528 });
    expect(getRollingMetricDigitTiming(1)).toEqual({ delayMs: 42, durationMs: 621 });
    expect(getRollingMetricDigitTiming(2)).toEqual({ delayMs: 84, durationMs: 731 });
  });
});
