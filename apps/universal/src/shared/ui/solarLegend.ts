import { formatWhAndKWh } from '@/features/telemetry/format';

export const MIN_MEANINGFUL_SOLAR_COMPARISON_BASELINE_WH = 24;

export function formatSolarLegendDelta(
  todayWh: number | null | undefined,
  yesterdayWh: number | null | undefined,
  deltaPct: number | null | undefined
): string {
  const safeTodayWh = Number.isFinite(todayWh) ? Math.max(0, todayWh ?? 0) : 0;
  const safeYesterdayWh = Number.isFinite(yesterdayWh) ? Math.max(0, yesterdayWh ?? 0) : 0;
  if (deltaPct === null || deltaPct === undefined || !Number.isFinite(deltaPct)) {
    return '';
  }
  if (safeYesterdayWh <= 0) {
    return safeTodayWh > 0 ? ' (new activity today)' : '';
  }
  if (safeYesterdayWh < MIN_MEANINGFUL_SOLAR_COMPARISON_BASELINE_WH) {
    const absoluteDeltaWh = Math.max(0, safeTodayWh - safeYesterdayWh);
    return absoluteDeltaWh > 0 ? ` (+${formatWhAndKWh(absoluteDeltaWh)})` : '';
  }
  const rounded = Math.round(deltaPct);
  const sign = rounded > 0 ? '+' : '';
  return ` (${sign}${rounded}%)`;
}
