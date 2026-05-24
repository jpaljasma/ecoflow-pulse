export type SocSweepMode = 'charging' | 'discharging' | 'idle' | 'neutral';
export type SocSweepDirection = 'left-to-right' | 'right-to-left';
export type SocSweepConfig = {
  enabled: boolean;
  overlayColor: string;
  direction: SocSweepDirection;
};

export const SOC_SWEEP_PAUSE_MS = 5000;
export const SOC_SWEEP_DURATION_MS = 1660;
export const SOC_SWEEP_PERIOD_MS = SOC_SWEEP_DURATION_MS + SOC_SWEEP_PAUSE_MS;
export const SOC_SWEEP_WIDTH_RATIO = 0.2;
export const SOC_SWEEP_SKEW_DEG = -45;

export function getSocSweepConfig(mode: SocSweepMode): SocSweepConfig {
  if (mode === 'charging') {
    return { enabled: true, overlayColor: 'rgba(255,255,255,0.42)', direction: 'left-to-right' };
  }
  if (mode === 'discharging') {
    return { enabled: true, overlayColor: 'rgba(0,0,0,0.24)', direction: 'right-to-left' };
  }
  return { enabled: false, overlayColor: 'transparent', direction: 'left-to-right' };
}

export function getSocSweepTravelRange(
  trackWidth: number,
  sweepWidth: number,
  direction: SocSweepDirection
): [number, number] {
  if (direction === 'right-to-left') {
    return [trackWidth + sweepWidth, -sweepWidth];
  }
  return [-sweepWidth, trackWidth + sweepWidth];
}
