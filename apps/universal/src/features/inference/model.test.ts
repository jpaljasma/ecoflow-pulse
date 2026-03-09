import { describe, expect, it } from 'vitest';

import { buildBatteryUpsellView } from '@/features/inference/model';
import type { DeviceInsights } from '@/features/inference/api';

const readyInsights: DeviceInsights = {
  deviceId: 'dev-1',
  status: 'ready',
  statusDetail: 'ok',
  refreshedAtUnixMs: '1772197190000',
  insights: [
    {
      id: 'ins-1',
      deviceId: 'dev-1',
      kind: 'battery_expansion',
      title: 'Add extra battery capacity',
      summary: 'Your DELTA Pro Ultra is using 2 of 5 supported battery packs.',
      score: 0.9,
      rank: 1,
      modelKey: 'battery-expansion-rule',
      modelVersion: 'v1',
      generatedAtUnixMs: '1772197190000',
      expiresAtUnixMs: '1772218790000',
      tags: ['battery', 'upsell'],
      evidence: [],
      actions: [
        {
          kind: 'external_url',
          label: 'Get More Batteries (3)',
          target:
            'https://us.ecoflow.com/products/delta-pro-ultra-battery?variant=41446274465865&inviteCode=ATH7F3EF1P'
        }
      ],
      attributes: {
        recommended_additional_packs: 3,
        max_battery_packs: 5
      }
    }
  ]
};

describe('battery upsell inference model', () => {
  it('prefers ready inference-backed upsell content', () => {
    const view = buildBatteryUpsellView({
      insights: readyInsights,
      model: 'DELTA Pro Ultra',
      batteryCount: 2,
      allowFallback: true
    });

    expect(view).toEqual(
      expect.objectContaining({
        title: 'Add extra battery capacity',
        summary: 'Your DELTA Pro Ultra is using 2 of 5 supported battery packs.',
        href:
          'https://us.ecoflow.com/products/delta-pro-ultra-battery?variant=41446274465865&inviteCode=ATH7F3EF1P',
        ctaLabel: 'Get More Batteries (3)',
        recommendedAdditionalBatteries: 3,
        maxBatteries: 5
      })
    );
  });

  it('falls back to static merchandising when inference is unavailable', () => {
    const view = buildBatteryUpsellView({
      insights: {
        ...readyInsights,
        status: 'pending',
        insights: []
      },
      model: 'DELTA 2 Max',
      batteryCount: 1,
      allowFallback: true
    });

    expect(view).toEqual(
      expect.objectContaining({
        href:
          'https://us.ecoflow.com/products/delta-2-max-smart-extra-battery-flash-sales?_pos=1&_sid=ed8ecff75&_ss=r&variant=40573812310089&inviteCode=ATH7F3EF1P',
        ctaLabel: 'Get More Batteries (2)',
        recommendedAdditionalBatteries: 2,
        maxBatteries: 3
      })
    );
  });

  it('suppresses the fallback when inference is ready but has no battery insight', () => {
    const view = buildBatteryUpsellView({
      insights: {
        ...readyInsights,
        insights: []
      },
      model: 'DELTA Pro Ultra',
      batteryCount: 2,
      allowFallback: true
    });

    expect(view).toBeNull();
  });
});
