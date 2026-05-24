export type SocSweepMode = 'charging' | 'discharging' | 'idle' | 'neutral';

export const SOC_SWEEP_PAUSE_MS = 5000;
export const SOC_SWEEP_DURATION_MS = 1660;
export const SOC_SWEEP_PERIOD_MS = SOC_SWEEP_DURATION_MS + SOC_SWEEP_PAUSE_MS;
export const SOC_SWEEP_WIDTH_RATIO = 0.2;
export const SOC_SWEEP_SKEW_DEG = -45;

export function getSocSweepConfig(mode: SocSweepMode): { enabled: boolean; overlayColor: string } {
  if (mode === 'charging') {
    return { enabled: true, overlayColor: 'rgba(255,255,255,0.42)' };
  }
  if (mode === 'discharging') {
    return { enabled: true, overlayColor: 'rgba(0,0,0,0.24)' };
  }
  return { enabled: false, overlayColor: 'transparent' };
}
