import { describe, expect, it } from 'vitest';
import {
  getSocSweepConfig,
  getSocSweepEasingPoint,
  getSocSweepTravelRange,
  SOC_SWEEP_DURATION_MS,
  SOC_SWEEP_EASING,
  SOC_SWEEP_PAUSE_MS,
  SOC_SWEEP_PERIOD_MS,
  SOC_SWEEP_SKEW_DEG,
  SOC_SWEEP_WIDTH_RATIO
} from '@/shared/ui/SocBarMotion';

describe('SocBarMotion', () => {
  it('uses a slower ease-out sweep before pausing between passes', () => {
    expect(SOC_SWEEP_DURATION_MS).toBe(2200);
    expect(SOC_SWEEP_PAUSE_MS).toBe(5000);
    expect(SOC_SWEEP_PERIOD_MS).toBe(7200);
    expect(SOC_SWEEP_EASING).toBe('out-cubic');
  });

  it('uses a 20 percent sweep band with 45-degree edges', () => {
    expect(SOC_SWEEP_WIDTH_RATIO).toBe(0.2);
    expect(SOC_SWEEP_SKEW_DEG).toBe(-45);
  });

  it('travels across the full gauge track and reverses for discharging', () => {
    expect(getSocSweepTravelRange(400, 80, 'left-to-right')).toEqual([-80, 480]);
    expect(getSocSweepTravelRange(400, 80, 'right-to-left')).toEqual([480, -80]);
  });

  it('uses an ease-out curve that slows toward the end of the sweep', () => {
    const firstQuarterDistance = getSocSweepEasingPoint(0.25) - getSocSweepEasingPoint(0);
    const lastQuarterDistance = getSocSweepEasingPoint(1) - getSocSweepEasingPoint(0.75);

    expect(getSocSweepEasingPoint(0)).toBe(0);
    expect(getSocSweepEasingPoint(0.5)).toBeCloseTo(0.875);
    expect(getSocSweepEasingPoint(1)).toBe(1);
    expect(firstQuarterDistance).toBeGreaterThan(lastQuarterDistance);
  });

  it('highlights charging gauges and darkens discharging gauges', () => {
    expect(getSocSweepConfig('charging')).toEqual({
      enabled: true,
      overlayColor: 'rgba(255,255,255,0.42)',
      direction: 'left-to-right'
    });
    expect(getSocSweepConfig('discharging')).toEqual({
      enabled: true,
      overlayColor: 'rgba(0,0,0,0.24)',
      direction: 'right-to-left'
    });
    expect(getSocSweepConfig('idle')).toEqual({
      enabled: false,
      overlayColor: 'transparent',
      direction: 'left-to-right'
    });
  });
});
