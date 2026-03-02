import { z } from 'zod';
import { requestJson } from '@/shared/api/restClient';

export const RollupResolutionSchema = z.enum(['minute', 'hour', 'day']);

const RollupMetricsSchema = z.object({
  socAvgPct: z.number(),
  socMinPct: z.number(),
  socMaxPct: z.number(),
  acInAvgW: z.number(),
  acInMaxW: z.number(),
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
  solarGeneratedWh: z.number()
});

const RollupPointSchema = z.object({
  bucketStartUnixMs: z.string(),
  bucketEndUnixMs: z.string(),
  sampleCount: z.number().int(),
  firstTsUnixMs: z.string(),
  lastTsUnixMs: z.string(),
  metrics: RollupMetricsSchema
});

export const RollupSeriesSchema = z.object({
  deviceId: z.string(),
  resolution: RollupResolutionSchema,
  fromUnixMs: z.string(),
  toUnixMs: z.string(),
  points: z.array(RollupPointSchema)
});

export const CompareRollupSeriesSchema = z.object({
  current: RollupSeriesSchema,
  previous: RollupSeriesSchema
});

export type RollupResolution = z.infer<typeof RollupResolutionSchema>;
export type RollupPoint = z.infer<typeof RollupPointSchema>;
export type RollupSeries = z.infer<typeof RollupSeriesSchema>;
export type CompareRollupSeries = z.infer<typeof CompareRollupSeriesSchema>;

type HistoryRequest = {
  deviceId: string;
  resolution: RollupResolution;
  fromIso: string;
  toIso: string;
  token?: string;
};

export async function fetchDeviceHistory({
  deviceId,
  resolution,
  fromIso,
  toIso,
  token
}: HistoryRequest): Promise<RollupSeries> {
  const params = new URLSearchParams({
    resolution,
    from: fromIso,
    to: toIso
  });
  const data = await requestJson<unknown>(`/api/v1/devices/${deviceId}/history?${params.toString()}`, { token });
  return RollupSeriesSchema.parse(data);
}

export async function fetchCompareDeviceHistory({
  deviceId,
  resolution,
  fromIso,
  toIso,
  token
}: HistoryRequest): Promise<CompareRollupSeries> {
  const params = new URLSearchParams({
    resolution,
    from: fromIso,
    to: toIso
  });
  const data = await requestJson<unknown>(`/api/v1/devices/${deviceId}/history/compare?${params.toString()}`, {
    token
  });
  return CompareRollupSeriesSchema.parse(data);
}
