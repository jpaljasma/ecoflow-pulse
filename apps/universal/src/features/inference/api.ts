import { z } from 'zod';

import { requestJson } from '@/shared/api/restClient';

export const InsightActionSchema = z.object({
  kind: z.enum(['internal_route', 'external_url', 'learn_more', 'dismiss', 'unspecified']),
  label: z.string(),
  target: z.string(),
  params: z.record(z.unknown()).optional()
});

export const InsightEvidenceSchema = z.object({
  source: z.enum([
    'live_snapshot',
    'rollup_history',
    'device_capabilities',
    'provider_metadata',
    'model_output',
    'rule_engine',
    'user_context',
    'unspecified'
  ]),
  summary: z.string(),
  metrics: z.record(z.unknown()).optional()
});

export const DeviceInsightSchema = z.object({
  id: z.string(),
  deviceId: z.string(),
  kind: z.enum([
    'unspecified',
    'battery_expansion',
    'solar_add_on',
    'solar_upgrade',
    'energy_shift',
    'maintenance'
  ]),
  title: z.string(),
  summary: z.string(),
  score: z.number(),
  rank: z.number().int(),
  modelKey: z.string(),
  modelVersion: z.string(),
  generatedAtUnixMs: z.string(),
  expiresAtUnixMs: z.string(),
  tags: z.array(z.string()),
  evidence: z.array(InsightEvidenceSchema),
  actions: z.array(InsightActionSchema),
  attributes: z.record(z.unknown()).optional()
});

export const DeviceInsightsSchema = z.object({
  deviceId: z.string(),
  status: z.enum(['pending', 'ready', 'stale', 'unavailable', 'unspecified']),
  statusDetail: z.string(),
  refreshedAtUnixMs: z.string(),
  insights: z.array(DeviceInsightSchema)
});

export type DeviceInsight = z.infer<typeof DeviceInsightSchema>;
export type DeviceInsights = z.infer<typeof DeviceInsightsSchema>;

type FetchDeviceInsightsOptions = {
  token?: string;
};

export async function fetchDeviceInsights(
  deviceId: string,
  { token }: FetchDeviceInsightsOptions = {}
): Promise<DeviceInsights> {
  const data = await requestJson<unknown>(
    `/api/v1/devices/${deviceId}/insights?kind=battery_expansion&maxItems=1`,
    { token }
  );
  return DeviceInsightsSchema.parse(data);
}
