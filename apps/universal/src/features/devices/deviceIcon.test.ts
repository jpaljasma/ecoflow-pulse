import { describe, expect, it } from 'vitest';

import { getDeviceAssetMatch } from '@/features/devices/deviceIcon';

describe('deviceIcon', () => {
  it('maps DELTA Pro Ultra X models to the dedicated DPU-X asset family', () => {
    expect(getDeviceAssetMatch('DELTA Pro Ultra X')).toEqual({
      assetFamily: 'ecoflow',
      slug: 'delta_pro_ultra_x',
      glyph: {
        icon: 'home-battery-outline',
        label: 'DPU-X'
      }
    });
  });

  it('maps Pecron E1000LFP models to Pecron visual assets', () => {
    expect(getDeviceAssetMatch('E1000LFP')).toEqual({
      assetFamily: 'pecron',
      slug: 'pecron_e1000lfp',
      glyph: {
        icon: 'battery-charging-wireless-outline',
        label: 'Pecron'
      }
    });
  });

  it('does not label unknown devices as EcoFlow by default', () => {
    expect(getDeviceAssetMatch('Mystery Portable Power Station')).toEqual({
      glyph: {
        icon: 'puzzle-outline',
        label: 'Device'
      }
    });
  });
});
