export const AVOIDED_EMISSIONS_FACTOR_VERSION = 'egrid2023_rev2';
export const DEFAULT_AVOIDED_EMISSIONS_FACTOR_KEY = 'NYUP';
export const ENERGY_IMPACT_REFERENCE_DOC_URL =
  'https://github.com/jpaljasma/ecoflow-pulse/blob/main/docs/reference/solar-avoided-emissions.md';
export const TREE_EQUIVALENT_REFERENCE_DOC_URL =
  'https://github.com/jpaljasma/ecoflow-pulse/blob/main/docs/reference/tree-equivalent.md';
export const EV_RANGE_REFERENCE_DOC_URL =
  'https://github.com/jpaljasma/ecoflow-pulse/blob/main/docs/reference/ev-us-europe-database-report.md';
export const TREE_EQUIVALENT_FACTOR_VERSION = 'tree_eq_v1';
export const EV_MILES_FACTOR_VERSION = 'premium_ev_median_v1';
export const PV_LIFECYCLE_CO2E_KG_PER_KWH = 0.045;
export const GENERIC_TREE_CO2_REMOVED_KG_PER_YEAR = 21.8;
export const GENERIC_TREE_KWH_PER_YEAR = GENERIC_TREE_CO2_REMOVED_KG_PER_YEAR / PV_LIFECYCLE_CO2E_KG_PER_KWH;
export const PREMIUM_EV_MEDIAN_CONSUMPTION_KWH_PER_100MI = 35.8185;
export const PREMIUM_EV_MILES_PER_KWH = 100 / PREMIUM_EV_MEDIAN_CONSUMPTION_KWH_PER_100MI;
export const PREMIUM_EV_MEDIAN_SAMPLE_COUNT = 567;
export const PREMIUM_EV_REFERENCE_SCOPE =
  'combined U.S.+Europe premium-brand BEV rows from the stored EV reference dataset';
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
  evMilesEquivalent: number;
};

export type EnergyImpactUiRow = {
  key: 'co2e' | 'air' | 'solar' | 'evMiles' | 'trees';
  badge: 'CO2e' | 'Air' | 'Solar' | 'EV' | 'Trees';
  headline: string;
  detail: string;
};

function describeImpactWindow(period: EnergyImpactPeriod, labelOverride?: string): {
  contextPhrase: string;
  generatedPhrase: string;
} {
  if (labelOverride && labelOverride.trim() !== '') {
    return {
      contextPhrase: `over ${labelOverride}`,
      generatedPhrase: ` generated over ${labelOverride}.`
    };
  }
  return period === 'today'
    ? {
        contextPhrase: 'today',
        generatedPhrase: ' generated with your own solar today.'
      }
    : {
        contextPhrase: 'over the past 12 months',
        generatedPhrase: ' generated with your own solar.'
      };
}

export function buildPastTwelveMonthsBounds(now = new Date()): { from: Date; to: Date } {
  const to = new Date(now);
  const from = new Date(now);
  from.setFullYear(from.getFullYear() - 1);
  return { from, to };
}

export function energyImpactPeriodLabel(period: EnergyImpactPeriod): string {
  return period === 'today' ? 'today so far' : 'the past 12 months';
}

export function formatMiles(value: number): string {
  const positive = Math.max(0, value);
  if (positive >= 100) {
    return `${Math.round(positive)}`;
  }
  if (positive >= 10) {
    return `${positive.toFixed(1)}`;
  }
  if (positive >= 1) {
    return `${positive.toFixed(1)}`;
  }
  return `${positive.toFixed(2)}`;
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

export function formatPollutantMass(grams: number): string {
  const positive = Math.max(0, grams);
  if (positive >= 1000) {
    return `${(positive / 1000).toFixed(2)} kg`;
  }
  if (positive >= 1) {
    return `${Math.round(positive)} g`;
  }
  return `${Math.round(positive * 1000)} mg`;
}

export function buildEnergyImpactRows(
  impact: EnergyImpactResult,
  period: EnergyImpactPeriod,
  labelOverride?: string
): EnergyImpactUiRow[] {
  const windowCopy = describeImpactWindow(period, labelOverride);
  return [
    {
      key: 'co2e',
      badge: 'CO2e',
      headline: `You made cleaner power ${windowCopy.contextPhrase}`,
      detail: `${formatImpactMassFromGrams(impact.co2eGrams)} CO2e displaced from the grid mix.`
    },
    {
      key: 'air',
      badge: 'Air',
      headline: `You helped keep air cleaner ${windowCopy.contextPhrase}`,
      detail: `${formatPollutantMass(impact.noxGrams)} NOx and ${formatPollutantMass(impact.so2Grams)} SO2 displaced.`
    },
    {
      key: 'solar',
      badge: 'Solar',
      headline: `You relied less on the grid ${windowCopy.contextPhrase}`,
      detail: `${formatWhAndKWhLocal(impact.solarWh)}${windowCopy.generatedPhrase}`
    },
    {
      key: 'evMiles',
      badge: 'EV',
      headline: 'Sunlight turned into something real',
      detail: `About ${formatMiles(impact.evMilesEquivalent)} EV miles of driving energy.`
    },
    {
      key: 'trees',
      badge: 'Trees',
      headline: 'A carbon benchmark you can picture',
      detail: `About ${formatTreeYears(impact.treeYearsEquivalent).replace(' tree-years', '')} mature tree-years on a conservative lifecycle basis.`
    }
  ];
}

function formatWhAndKWhLocal(valueWh: number): string {
  const positive = Math.max(0, valueWh);
  if (positive < 1000) {
    return `${Math.round(positive)} Wh`;
  }
  return `${(positive / 1000).toFixed(2)} kWh`;
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
  const evMilesEquivalent = solarKWh * PREMIUM_EV_MILES_PER_KWH;

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
    evMilesEquivalent
  };
}
