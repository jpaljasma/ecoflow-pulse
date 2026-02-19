export type UiTone = 'neutral' | 'success' | 'warning' | 'danger' | 'info';
export type MetricTone = 'default' | 'muted' | 'cold';

export function isMutedMetric(value: number | undefined | null): boolean {
  if (value === null || value === undefined || Number.isNaN(value)) return false;
  return value >= -0.5 && value <= 0.5;
}

export function toneFromState(state: string | undefined): UiTone {
  const value = (state ?? '').toLowerCase();
  if (value.includes('charging') || value.includes('online') || value.includes('on')) return 'success';
  if (value.includes('locked') || value.includes('idle')) return 'warning';
  if (value.includes('error') || value.includes('fault') || value.includes('off')) return 'danger';
  return 'neutral';
}

export function signalTone(on: boolean | undefined): UiTone {
  if (on === true) return 'success';
  if (on === false) return 'neutral';
  return 'warning';
}

export function formatSolarState(state: string | undefined): string {
  if (!state) return 'unknown';
  return state.replaceAll('_', ' ');
}

export function isInactivePvPort(volts: number | undefined): boolean {
  return Number.isFinite(volts as number) && Math.abs((volts as number) ?? 0) <= 0.5;
}

export function toPctOfMax(
  value: number | undefined,
  max: number | undefined
): number | null {
  if (!Number.isFinite(value as number) || !Number.isFinite(max as number) || (max as number) <= 0) {
    return null;
  }
  return ((value as number) / (max as number)) * 100;
}

export function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, value));
}

export function pvLoadColor(percent: number): string {
  const pct = clampPercent(percent) / 100;
  const start = { r: 255, g: 232, b: 199 };
  const end = { r: 255, g: 159, b: 10 };
  const r = Math.round(start.r + (end.r - start.r) * pct);
  const g = Math.round(start.g + (end.g - start.g) * pct);
  const b = Math.round(start.b + (end.b - start.b) * pct);
  return `rgb(${r},${g},${b})`;
}

