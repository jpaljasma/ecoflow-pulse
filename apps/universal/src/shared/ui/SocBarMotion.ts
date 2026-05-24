export type SocSweepMode = 'charging' | 'discharging' | 'idle' | 'neutral';
export type SocSweepDirection = 'left-to-right' | 'right-to-left';
export type SocSweepEasing = 'out-cubic';
export type SocSweepConfig = {
  enabled: boolean;
  overlayColor: string;
  direction: SocSweepDirection;
};

export const SOC_SWEEP_PAUSE_MS = 5000;
export const SOC_SWEEP_DURATION_MS = 2200;
export const SOC_SWEEP_PERIOD_MS = SOC_SWEEP_DURATION_MS + SOC_SWEEP_PAUSE_MS;
export const SOC_SWEEP_EASING: SocSweepEasing = 'out-cubic';
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

export function getSocSweepEasingPoint(progress: number): number {
  const t = Math.max(0, Math.min(1, progress));
  return 1 - Math.pow(1 - t, 3);
}
