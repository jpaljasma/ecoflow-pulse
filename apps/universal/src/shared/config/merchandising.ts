export const ecoFlowInviteCode = 'ATH7F3EF1P';

export type BatteryUpsellContext = {
  model?: string;
  serialNumber?: string;
  batteryCount?: number;
};

type BatteryUpsellRule = {
  key: string;
  matches: (ctx: BatteryUpsellContext) => boolean;
  url: string;
};

type ModelProfile = {
  key: 'delta-pro-ultra' | 'delta-2-max';
  match: RegExp;
  maxBatteryCount: number;
};

const MODEL_PROFILES: ModelProfile[] = [
  { key: 'delta-pro-ultra', match: /\bdelta\s*pro\s*ultra\b/i, maxBatteryCount: 5 },
  { key: 'delta-2-max', match: /\bdelta\s*2\s*max\b/i, maxBatteryCount: 3 }
];

const batteryUpsellRules: BatteryUpsellRule[] = [
  {
    key: 'delta-pro-ultra',
    matches: (ctx) =>
      (ctx.model ?? '').toLowerCase().includes('delta pro ultra') &&
      (ctx.batteryCount ?? 0) < 5,
    url:
      'https://us.ecoflow.com/products/delta-pro-ultra-battery?variant=41446274465865&inviteCode={inviteCode}'
  },
  {
    key: 'delta-2-max',
    matches: (ctx) =>
      (ctx.model ?? '').toLowerCase().includes('delta 2 max') &&
      (ctx.batteryCount ?? 0) < 3,
    url:
      'https://us.ecoflow.com/products/delta-2-max-smart-extra-battery-flash-sales?_pos=1&_sid=ed8ecff75&_ss=r&variant=40573812310089&inviteCode={inviteCode}'
  }
];

export function getBatteryUpsellUrl(ctx: BatteryUpsellContext): string | undefined {
  const match = batteryUpsellRules.find((rule) => rule.matches(ctx));
  if (!match) return undefined;
  return match.url.replace('{inviteCode}', encodeURIComponent(ecoFlowInviteCode));
}

function normalizeModel(model?: string): string {
  return (model ?? '').toLowerCase().replace(/[^a-z0-9]+/g, ' ').trim();
}

export function getMaxBatteryCount(model?: string): number {
  const normalized = normalizeModel(model);
  const profile = MODEL_PROFILES.find((p) => p.match.test(normalized));
  return profile?.maxBatteryCount || 0;
}