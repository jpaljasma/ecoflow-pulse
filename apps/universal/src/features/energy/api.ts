import { z } from 'zod';
import { requestJson } from '@/shared/api/restClient';

export const EnergyPresetSchema = z.enum([
  'today',
  'yesterday',
  'last7d',
  'thisWeek',
  'previousWeek',
  'thisMonth',
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

export async function fetchEnergyDashboard({
  scope,
  deviceId,
  preset,
  timezone,
  includeComparison = true,
  gridPricePerKwh,
  currency,
  token
}: {
  scope: 'device' | 'all';
  deviceId?: string;
  preset: EnergyPreset;
  timezone: string;
  includeComparison?: boolean;
  gridPricePerKwh?: number;
  currency?: string;
  token?: string;
}): Promise<EnergyDashboard> {
  const params = new URLSearchParams({
    scope,
    preset,
    timezone,
    includeComparison: includeComparison ? 'true' : 'false'
  });
  if (scope === 'device' && deviceId) {
    params.set('deviceId', deviceId);
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
