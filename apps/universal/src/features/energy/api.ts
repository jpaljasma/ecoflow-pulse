import { z } from 'zod';
import { requestJson } from '@/shared/api/restClient';

export const EnergyPresetSchema = z.enum([
  'today',
  'past24h',
  'yesterday',
  'last7d',
  'last30d',
  'thisWeek',
  'previousWeek',
  'thisMonth',
  'lastMonth',
  'last12m'
]);

const RollupMetricsSchema = z.object({
  socAvgPct: z.number(),
  socMinPct: z.number(),
  socMaxPct: z.number(),
  acInAvgW: z.number(),
  acInMaxW: z.number(),
  acOutputAvgW: z.number(),
  acOutputMaxW: z.number(),
  pvAvgW: z.number(),
  pvMaxW: z.number(),
  dcAvgW: z.number(),
  dcMaxW: z.number(),
  loadAvgW: z.number(),
  loadMaxW: z.number(),
  netAvgW: z.number(),
  netMinW: z.number(),
  netMaxW: z.number(),
  batteryAvgW: z.number(),
  batteryMinW: z.number(),
  batteryMaxW: z.number(),
  tempAvgC: z.number(),
  tempMinC: z.number(),
  tempMaxC: z.number(),
  solarGeneratedWh: z.number(),
  acInputEnergyWh: z.number(),
  acOutputEnergyWh: z.number(),
  dcOutputEnergyWh: z.number(),
  loadEnergyWh: z.number(),
  batteryChargeEnergyWh: z.number(),
  batteryDischargeEnergyWh: z.number()
});

const RollupPointSchema = z.object({
  bucketStartUnixMs: z.string(),
  bucketEndUnixMs: z.string(),
  sampleCount: z.number().int(),
  firstTsUnixMs: z.string(),
  lastTsUnixMs: z.string(),
  metrics: RollupMetricsSchema
});

const EnergyValueComparisonSchema = z.object({
  current: z.number(),
  previous: z.number(),
  delta: z.number(),
  deltaPct: z.number().nullable()
});

const EnergySummarySchema = z.object({
  solarGeneratedKwh: EnergyValueComparisonSchema,
  loadConsumedKwh: EnergyValueComparisonSchema,
  selfSufficiencyPct: EnergyValueComparisonSchema,
  batteryNetKwh: EnergyValueComparisonSchema,
  estimatedValue: EnergyValueComparisonSchema,
  estimatedAcInputCost: EnergyValueComparisonSchema,
  currency: z.string()
});

const BatterySummarySchema = z.object({
  chargeKwh: z.number(),
  dischargeKwh: z.number(),
  netKwh: z.number(),
  socStartPct: z.number(),
  socEndPct: z.number(),
  socMinPct: z.number(),
  socMaxPct: z.number()
});

const EnergyScopeSchema = z.object({
  mode: z.string(),
  deviceId: z.string(),
  resolvedDeviceIds: z.array(z.string())
});

const EnergyWindowSchema = z.object({
  preset: z.string(),
  timezone: z.string(),
  fromUnixMs: z.string(),
  toUnixMs: z.string(),
  previousFromUnixMs: z.string(),
  previousToUnixMs: z.string()
});

const EnergyPVPortHistorySchema = z.object({
  deviceId: z.string(),
  portId: z.string(),
  portLabel: z.string(),
  maxObservedVolts: z.number(),
  maxObservedAmps: z.number(),
  maxObservedWatts: z.number(),
  lastObservedVolts: z.number(),
  lastObservedAmps: z.number(),
  lastObservedWatts: z.number(),
  lastObservedUnixMs: z.string(),
  sampleCount: z.number().int()
});

const EnergyPVPortHistoryResponseSchema = z.object({
  pvPortHistory: z.array(EnergyPVPortHistorySchema)
});

const InsightEvidenceSchema = z.object({
  source: z.string(),
  summary: z.string(),
  metrics: z.record(z.string(), z.unknown()).optional()
});

const EnergyComparisonCardSchema = z.object({
  category: z.string(),
  title: z.string(),
  summary: z.string(),
  recommendation: z.string(),
  score: z.number(),
  confidence: z.number(),
  evidence: z.array(InsightEvidenceSchema),
  attributes: z.record(z.string(), z.unknown()).optional()
});

const EnergyComparisonInsightSchema = z.object({
  id: z.string(),
  scope: z.object({
    mode: z.string(),
    deviceId: z.string(),
    resolvedDeviceIds: z.array(z.string())
  }),
  preset: z.string(),
  timezone: z.string(),
  verdictClass: z.string(),
  headline: z.string(),
  summary: z.string(),
  score: z.number(),
  confidence: z.number(),
  modelKey: z.string(),
  modelVersion: z.string(),
  generatedAtUnixMs: z.string(),
  expiresAtUnixMs: z.string(),
  tags: z.array(z.string()),
  cards: z.array(EnergyComparisonCardSchema),
  evidence: z.array(InsightEvidenceSchema),
  attributes: z.record(z.string(), z.unknown()).optional()
});

const EnergyComparisonInsightResponseSchema = z.object({
  status: z.string(),
  statusDetail: z.string(),
  insight: EnergyComparisonInsightSchema.optional()
});

const EnergyCalendarSelectedMonthTotalsSchema = z.object({
  solarGeneratedKwh: z.number(),
  estimatedValue: z.number(),
  currency: z.string()
});

const EnergyCalendarSelectedMonthSchema = z.object({
  year: z.number().int(),
  month: z.number().int(),
  totals: EnergyCalendarSelectedMonthTotalsSchema
});

const EnergyCalendarVisibleDaySchema = z.object({
  dateIso: z.string(),
  year: z.number().int(),
  month: z.number().int(),
  day: z.number().int(),
  solarGeneratedKwh: z.number(),
  estimatedValue: z.number(),
  currency: z.string(),
  isCurrentMonth: z.boolean(),
  hasData: z.boolean(),
  isToday: z.boolean().optional().default(false),
  isFuture: z.boolean()
});

const EnergyCalendarSchema = z.object({
  scope: EnergyScopeSchema,
  selectedMonth: EnergyCalendarSelectedMonthSchema,
  visibleDays: z.array(EnergyCalendarVisibleDaySchema)
});

export const EnergyDashboardSchema = z.object({
  scope: EnergyScopeSchema,
  window: EnergyWindowSchema,
  summary: EnergySummarySchema,
  battery: BatterySummarySchema,
  currentEnergyPoints: z.array(RollupPointSchema),
  previousEnergyPoints: z.array(RollupPointSchema),
  currentPowerPoints: z.array(RollupPointSchema),
  previousPowerPoints: z.array(RollupPointSchema),
  pvPortHistory: z.array(EnergyPVPortHistorySchema)
});

export type EnergyPreset = z.infer<typeof EnergyPresetSchema>;
export type EnergyValueComparison = z.infer<typeof EnergyValueComparisonSchema>;
export type EnergyDashboard = z.infer<typeof EnergyDashboardSchema>;
export type EnergyRollupPoint = z.infer<typeof RollupPointSchema>;
export type EnergyPVPortHistory = z.infer<typeof EnergyPVPortHistorySchema>;
export type EnergyComparisonInsight = z.infer<typeof EnergyComparisonInsightSchema>;
export type EnergyComparisonInsightResponse = z.infer<typeof EnergyComparisonInsightResponseSchema>;
export type EnergyCalendarSelectedMonthTotals = z.infer<typeof EnergyCalendarSelectedMonthTotalsSchema>;
export type EnergyCalendarSelectedMonth = z.infer<typeof EnergyCalendarSelectedMonthSchema>;
export type EnergyCalendarVisibleDay = z.infer<typeof EnergyCalendarVisibleDaySchema>;
export type EnergyCalendar = z.infer<typeof EnergyCalendarSchema>;

export async function fetchEnergyDashboard({
  scope,
  deviceId,
  preset,
  includeComparison = true,
  date,
  gridPricePerKwh,
  currency,
  token
}: {
  scope: 'device' | 'all';
  deviceId?: string;
  preset: EnergyPreset;
  includeComparison?: boolean;
  date?: string;
  gridPricePerKwh?: number;
  currency?: string;
  token?: string;
}): Promise<EnergyDashboard> {
  const params = new URLSearchParams({
    scope,
    preset,
    includeComparison: includeComparison ? 'true' : 'false'
  });
  if (scope === 'device' && deviceId) {
    params.set('deviceId', deviceId);
  }
  if (date) {
    params.set('date', date);
  }
  if (gridPricePerKwh !== undefined && Number.isFinite(gridPricePerKwh)) {
    params.set('gridPricePerKwh', String(gridPricePerKwh));
  }
  if (currency) {
    params.set('currency', currency);
  }
  const data = await requestJson<unknown>(`/api/v1/energy/dashboard?${params.toString()}`, { token });
  return EnergyDashboardSchema.parse(data);
}

export async function fetchEnergyPvPortHistory({
  scope,
  deviceId,
  preset,
  date,
  token
}: {
  scope: 'device' | 'all';
  deviceId?: string;
  preset: EnergyPreset;
  date?: string;
  token?: string;
}): Promise<EnergyPVPortHistory[]> {
  const params = new URLSearchParams({
    scope,
    preset
  });
  if (scope === 'device' && deviceId) {
    params.set('deviceId', deviceId);
  }
  if (date) {
    params.set('date', date);
  }
  const data = await requestJson<unknown>(`/api/v1/energy/pv-history?${params.toString()}`, { token });
  return EnergyPVPortHistoryResponseSchema.parse(data).pvPortHistory;
}

export async function fetchEnergyComparisonInsight({
  scope,
  deviceId,
  preset,
  date,
  gridPricePerKwh,
  currency,
  token
}: {
  scope: 'device' | 'all';
  deviceId?: string;
  preset: EnergyPreset;
  date?: string;
  gridPricePerKwh?: number;
  currency?: string;
  token?: string;
}): Promise<EnergyComparisonInsightResponse> {
  const params = new URLSearchParams({
    scope,
    preset
  });
  if (scope === 'device' && deviceId) {
    params.set('deviceId', deviceId);
  }
  if (date) {
    params.set('date', date);
  }
  if (gridPricePerKwh !== undefined && Number.isFinite(gridPricePerKwh)) {
    params.set('gridPricePerKwh', String(gridPricePerKwh));
  }
  if (currency) {
    params.set('currency', currency);
  }
  const data = await requestJson<unknown>(`/api/v1/energy/comparison-insight?${params.toString()}`, { token });
  return EnergyComparisonInsightResponseSchema.parse(data);
}

export async function fetchEnergyCalendar({
  scope,
  deviceId,
  year,
  month,
  gridPricePerKwh,
  currency,
  token
}: {
  scope: 'device' | 'all';
  deviceId?: string;
  year: number;
  month: number;
  gridPricePerKwh?: number;
  currency?: string;
  token?: string;
}): Promise<EnergyCalendar> {
  const params = new URLSearchParams();
  params.set('scope', scope);
  if (scope === 'device' && deviceId) {
    params.set('deviceId', deviceId);
  }
  params.set('year', String(year));
  params.set('month', String(month));
  if (gridPricePerKwh !== undefined && Number.isFinite(gridPricePerKwh)) {
    params.set('gridPricePerKwh', String(gridPricePerKwh));
  }
  if (currency) {
    params.set('currency', currency);
  }
  const data = await requestJson<unknown>(`/api/v1/energy/calendar?${params.toString()}`, { token });
  return EnergyCalendarSchema.parse(data);
}
