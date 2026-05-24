import { describe, expect, it } from 'vitest';
import {
  getSocSweepConfig,
  SOC_SWEEP_DURATION_MS,
  SOC_SWEEP_PAUSE_MS,
  SOC_SWEEP_PERIOD_MS,
  SOC_SWEEP_SKEW_DEG,
  SOC_SWEEP_WIDTH_RATIO
} from '@/shared/ui/SocBarMotion';

describe('SocBarMotion', () => {
  it('sweeps left-to-right before pausing between passes', () => {
    expect(SOC_SWEEP_DURATION_MS).toBe(1660);
    expect(SOC_SWEEP_PAUSE_MS).toBe(5000);
    expect(SOC_SWEEP_PERIOD_MS).toBe(6660);
  });

  it('uses a 20 percent sweep band with 45-degree edges', () => {
    expect(SOC_SWEEP_WIDTH_RATIO).toBe(0.2);
    expect(SOC_SWEEP_SKEW_DEG).toBe(-45);
  });

  it('highlights charging gauges and darkens discharging gauges', () => {
    expect(getSocSweepConfig('charging')).toEqual({
      enabled: true,
      overlayColor: 'rgba(255,255,255,0.42)'
    });
    expect(getSocSweepConfig('discharging')).toEqual({
      enabled: true,
      overlayColor: 'rgba(0,0,0,0.24)'
    });
    expect(getSocSweepConfig('idle')).toEqual({ enabled: false, overlayColor: 'transparent' });
  });
});
