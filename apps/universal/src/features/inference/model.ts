import type { DeviceInsight, DeviceInsights } from '@/features/inference/api';
import { getBatteryUpsellUrl, getMaxBatteryCount } from '@/shared/config/merchandising';

export type BatteryUpsellView = {
  title: string;
  summary: string;
  href?: string;
  ctaLabel: string;
  recommendedAdditionalBatteries: number;
  maxBatteries: number;
};

type BuildBatteryUpsellOptions = {
  insights?: DeviceInsights;
  model?: string;
  batteryCount: number;
  allowFallback: boolean;
};

export function buildBatteryUpsellView({
  insights,
  model,
  batteryCount,
  allowFallback
}: BuildBatteryUpsellOptions): BatteryUpsellView | null {
  const maxBatteries = getMaxBatteryCount(model);
  const fallbackHref = getBatteryUpsellUrl({ model, batteryCount });
  const fallbackAdditional = Math.max(0, maxBatteries - batteryCount);

  if (insights?.status === 'ready') {
    const insight = selectBatteryExpansionInsight(insights);
    if (!insight) {
      return null;
    }
    const recommendedAdditionalBatteries = maxInt(
      1,
      readPositiveInt(insight.attributes?.recommended_additional_packs) ?? fallbackAdditional
    );
    const inferredMax = readPositiveInt(insight.attributes?.max_battery_packs) ?? maxBatteries;
    const action = insight.actions.find((entry) => entry.kind === 'external_url' && entry.target);
    return {
      title: insight.title || `Your ${model ?? 'device'} supports more batteries`,
      summary: insight.summary || `Your ${model ?? 'device'} supports more batteries.`,
      href: action?.target || fallbackHref,
      ctaLabel: action?.label || `Get More Batteries (${recommendedAdditionalBatteries})`,
      recommendedAdditionalBatteries,
      maxBatteries: inferredMax
    };
  }

  if (!allowFallback || !fallbackHref || fallbackAdditional < 1) {
    return null;
  }

  return {
    title: `Your ${model ?? 'device'} supports more batteries!`,
    summary: `Add extra battery capacity to expand runtime and reserve power.`,
    href: fallbackHref,
    ctaLabel: `Get More Batteries (${fallbackAdditional})`,
    recommendedAdditionalBatteries: fallbackAdditional,
    maxBatteries
  };
}

export function selectBatteryExpansionInsight(insights: DeviceInsights | undefined): DeviceInsight | undefined {
  return insights?.insights.find((entry) => entry.kind === 'battery_expansion');
}

function readPositiveInt(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? Math.round(value) : undefined;
}

function maxInt(a: number, b: number): number {
  return a > b ? a : b;
}
