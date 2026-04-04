import { describe, expect, it } from 'vitest';

import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';

describe('deviceIcon', () => {
  it('maps DELTA Pro Ultra X models to the dedicated DPU-X asset family', () => {
    expect(getDeviceAssetMatch('DELTA Pro Ultra X')).toEqual({
      slug: 'delta_pro_ultra_x',
      glyph: {
        icon: 'home-battery-outline',
        label: 'DPU-X'
      }
    });
  });
});
