export const AVOIDED_EMISSIONS_FACTOR_VERSION = 'egrid2023_rev2';
export const DEFAULT_AVOIDED_EMISSIONS_FACTOR_KEY = 'NYUP';
export const ENERGY_IMPACT_REFERENCE_DOC_URL =
  'https://github.com/jpaljasma/ecoflow-pulse/blob/main/docs/reference/solar-avoided-emissions.md';
export const TREE_EQUIVALENT_REFERENCE_DOC_URL =
  'https://github.com/jpaljasma/ecoflow-pulse/blob/main/docs/reference/tree-equivalent.md';
export const TREE_EQUIVALENT_FACTOR_VERSION = 'tree_eq_v1';
export const PV_LIFECYCLE_CO2E_KG_PER_KWH = 0.045;
export const GENERIC_TREE_CO2_REMOVED_KG_PER_YEAR = 21.8;
export const GENERIC_TREE_KWH_PER_YEAR = GENERIC_TREE_CO2_REMOVED_KG_PER_YEAR / PV_LIFECYCLE_CO2E_KG_PER_KWH;
export const ENERGY_IMPACT_HISTORY_STALE_MS = Number.POSITIVE_INFINITY;
export const ENERGY_IMPACT_HISTORY_GC_MS = 24 * 60 * 60_000;

export type EnergyImpactPeriod = 'today' | 'past12Months';

export const AVOIDED_EMISSIONS_FACTORS = {
  NYUP: {
    key: 'NYUP',
    label: 'NPCC Upstate NY',
    source: 'EPA eGRID2023 non-baseload output emission rates',
    co2eMetricTonsPerKWh: 0.000413585523,
    co2eKgPerKWh: 0.413585523,
    noxGramsPerKWh: 0.181436948,
    so2GramsPerKWh: 0.046720014
  },
  US_AVG: {
    key: 'US_AVG',
    label: 'U.S. average',
    source: 'EPA eGRID2023 non-baseload output emission rates',
    co2eMetricTonsPerKWh: 0.000625594597,
    co2eKgPerKWh: 0.625594597,
    noxGramsPerKWh: 0.408233133,
    so2GramsPerKWh: 0.310257181
  }
} as const;

export type AvoidedEmissionsFactorKey = keyof typeof AVOIDED_EMISSIONS_FACTORS;
export type AvoidedEmissionsFactor = (typeof AVOIDED_EMISSIONS_FACTORS)[AvoidedEmissionsFactorKey];

export type EnergyImpactMetric = {
  key: 'co2e' | 'nox' | 'so2' | 'trees';
  badge: 'CO2e' | 'NOx' | 'SO2' | 'Trees';
  label: string;
  detail: string;
  grams?: number;
  treeYears?: number;
  displayAmount: string;
};

export type EnergyImpactResult = {
  solarWh: number;
  solarKWh: number;
  factorKey: AvoidedEmissionsFactorKey;
  factor: AvoidedEmissionsFactor;
  co2eMetricTons: number;
  co2eKg: number;
  co2eGrams: number;
  noxGrams: number;
  so2Grams: number;
  treeYearsEquivalent: number;
  solarLifecycleCo2eKg: number;
  metrics: EnergyImpactMetric[];
};

export function buildPastTwelveMonthsBounds(now = new Date()): { from: Date; to: Date } {
  const to = new Date(now);
  const from = new Date(now);
  from.setFullYear(from.getFullYear() - 1);
  return { from, to };
}

export function energyImpactPeriodLabel(period: EnergyImpactPeriod): string {
  return period === 'today' ? 'today so far' : 'the past 12 months';
}

export function formatTreeYears(value: number): string {
  const positive = Math.max(0, value);
  if (positive >= 100) {
    return `${Math.round(positive)} tree-years`;
  }
  if (positive >= 10) {
    return `${positive.toFixed(1)} tree-years`;
  }
  if (positive >= 1) {
    return `${positive.toFixed(2)} tree-years`;
  }
  if (positive >= 0.01) {
    return `${positive.toFixed(3)} tree-years`;
  }
  return `${positive.toFixed(5)} tree-years`;
}

export function formatImpactMassFromGrams(grams: number): string {
  const positive = Math.max(0, grams);
  if (positive >= 1000) {
    return `${(positive / 1000).toFixed(2)} kg`;
  }
  if (positive >= 10) {
    return `${Math.round(positive)} g`;
  }
  if (positive >= 1) {
    return `${positive.toFixed(1)} g`;
  }
  return `${positive.toFixed(3)} g`;
}

export function computeEnergyImpactFromSolarWh(
  solarWh: number | undefined,
  factorKey: AvoidedEmissionsFactorKey = DEFAULT_AVOIDED_EMISSIONS_FACTOR_KEY
): EnergyImpactResult {
  const normalizedSolarWh = Math.max(0, solarWh ?? 0);
  const solarKWh = normalizedSolarWh / 1000;
  const factor = AVOIDED_EMISSIONS_FACTORS[factorKey];
  const co2eMetricTons = solarKWh * factor.co2eMetricTonsPerKWh;
  const co2eKg = solarKWh * factor.co2eKgPerKWh;
  const co2eGrams = co2eKg * 1000;
  const noxGrams = solarKWh * factor.noxGramsPerKWh;
  const so2Grams = solarKWh * factor.so2GramsPerKWh;
  const solarLifecycleCo2eKg = solarKWh * PV_LIFECYCLE_CO2E_KG_PER_KWH;
  const treeYearsEquivalent = solarLifecycleCo2eKg / GENERIC_TREE_CO2_REMOVED_KG_PER_YEAR;

  return {
    solarWh: normalizedSolarWh,
    solarKWh,
    factorKey,
    factor,
    co2eMetricTons,
    co2eKg,
    co2eGrams,
    noxGrams,
    so2Grams,
    treeYearsEquivalent,
    solarLifecycleCo2eKg,
    metrics: [
      {
        key: 'co2e',
        badge: 'CO2e',
        label: `${formatImpactMassFromGrams(co2eGrams)} CO2e avoided`,
        detail: 'Climate impact displaced by your solar generation for the selected period.',
        grams: co2eGrams,
        displayAmount: formatImpactMassFromGrams(co2eGrams)
      },
      {
        key: 'nox',
        badge: 'NOx',
        label: `${formatImpactMassFromGrams(noxGrams)} NOx avoided`,
        detail: 'Smog-forming nitrogen oxides displaced for the selected period.',
        grams: noxGrams,
        displayAmount: formatImpactMassFromGrams(noxGrams)
      },
      {
        key: 'so2',
        badge: 'SO2',
        label: `${formatImpactMassFromGrams(so2Grams)} SO2 avoided`,
        detail: 'Sulfur pollution displaced by solar generation for the selected period.',
        grams: so2Grams,
        displayAmount: formatImpactMassFromGrams(so2Grams)
      },
      {
        key: 'trees',
        badge: 'Trees',
        label: `${formatTreeYears(treeYearsEquivalent).replace(' tree-years', '')} mature tree-years`,
        detail: `Conservative lifecycle comparison only. 1 mature tree-year ≈ ${Math.round(GENERIC_TREE_KWH_PER_YEAR)} kWh.`,
        treeYears: treeYearsEquivalent,
        displayAmount: formatTreeYears(treeYearsEquivalent)
      }
    ]
  };
}
