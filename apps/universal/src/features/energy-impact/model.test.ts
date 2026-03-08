import { describe, expect, it } from 'vitest';
import {
  buildPastTwelveMonthsBounds,
  computeEnergyImpactFromSolarWh,
  energyImpactPeriodLabel,
  formatImpactMassFromGrams,
  formatTreeYears
} from '@/features/energy-impact/model';

describe('energy impact model', () => {
  it('matches the NYUP 10,000 kWh/year worked example', () => {
    const impact = computeEnergyImpactFromSolarWh(10_000_000, 'NYUP');

    expect(impact.co2eMetricTons).toBeCloseTo(4.13585523, 8);
    expect(impact.noxGrams).toBeCloseTo(1814.36948, 5);
    expect(impact.so2Grams).toBeCloseTo(467.200141, 5);
  });

  it('matches the U.S. average 10,000 kWh/year worked example', () => {
    const impact = computeEnergyImpactFromSolarWh(10_000_000, 'US_AVG');

    expect(impact.co2eMetricTons).toBeCloseTo(6.25594597, 8);
    expect(impact.noxGrams).toBeCloseTo(4082.33133, 5);
    expect(impact.so2Grams).toBeCloseTo(3102.57181, 5);
  });

  it('formats grams and kilograms for UI display', () => {
    expect(formatImpactMassFromGrams(1814.36948)).toBe('1.81 kg');
    expect(formatImpactMassFromGrams(467.200141)).toBe('467 g');
    expect(formatImpactMassFromGrams(6.617)).toBe('6.6 g');
    expect(formatImpactMassFromGrams(0.0029)).toBe('0.003 g');
  });

  it('computes tree-years from lifecycle CO2 benchmark', () => {
    const impact = computeEnergyImpactFromSolarWh(10_000_000, 'NYUP');

    expect(impact.solarLifecycleCo2eKg).toBeCloseTo(450, 6);
    expect(impact.treeYearsEquivalent).toBeCloseTo(20.64220183, 6);
  });

  it('formats very small tree-year values for today-so-far UI', () => {
    expect(formatTreeYears(20.64220183)).toBe('20.6 tree-years');
    expect(formatTreeYears(0.0001197)).toBe('0.00012 tree-years');
  });

  it('builds a rolling trailing 12-month range', () => {
    const now = new Date('2026-03-08T09:30:00-05:00');
    const { from, to } = buildPastTwelveMonthsBounds(now);

    expect(from.toISOString()).toBe('2025-03-08T15:30:00.000Z');
    expect(to.toISOString()).toBe('2026-03-08T14:30:00.000Z');
  });

  it('formats period labels for UI copy', () => {
    expect(energyImpactPeriodLabel('today')).toBe('today so far');
    expect(energyImpactPeriodLabel('past12Months')).toBe('the past 12 months');
  });

  it('clamps negative or missing solar values to zero', () => {
    expect(computeEnergyImpactFromSolarWh(undefined).co2eGrams).toBe(0);
    expect(computeEnergyImpactFromSolarWh(-50).noxGrams).toBe(0);
  });
});
