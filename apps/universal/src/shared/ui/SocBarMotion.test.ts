import { describe, expect, it } from 'vitest';
import {
  getSocSweepConfig,
  SOC_SWEEP_DELAY_MS,
  SOC_SWEEP_DURATION_MS,
  SOC_SWEEP_PERIOD_MS,
  SOC_SWEEP_WIDTH_RATIO
} from '@/shared/ui/SocBarMotion';

describe('SocBarMotion', () => {
  it('uses a five-second tick with a short sweep after the idle interval', () => {
    expect(SOC_SWEEP_DELAY_MS).toBe(4250);
    expect(SOC_SWEEP_DURATION_MS).toBe(750);
    expect(SOC_SWEEP_PERIOD_MS).toBe(5000);
  });

  it('uses a 20 percent diagonal sweep width', () => {
    expect(SOC_SWEEP_WIDTH_RATIO).toBe(0.2);
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
