import type { ComponentProps } from 'react';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import type { EcoFlowDeviceSlug } from '@/shared/assets/ecoflowAssets';
import type { PecronDeviceSlug } from '@/shared/assets/pecronAssets';

export type DeviceGlyph = {
  icon: ComponentProps<typeof MaterialCommunityIcons>['name'];
  label: string;
};

export type DeviceAssetMatch = {
  assetFamily?: 'ecoflow' | 'pecron';
  slug?: EcoFlowDeviceSlug | PecronDeviceSlug;
  glyph: DeviceGlyph;
};

export function getDeviceGlyph(model: string): DeviceGlyph {
  return getDeviceAssetMatch(model).glyph;
}

export function getDeviceAssetMatch(
  model: string,
  options?: { batteryCount?: number }
): DeviceAssetMatch {
  const m = model.toLowerCase();
  const compact = m.replace(/[^a-z0-9]/g, '');

  const pecron = getPecronAssetMatch(compact, m);
  if (pecron) {
    return pecron;
  }

  if (m.includes('delta pro ultra x')) {
    return {
      assetFamily: 'ecoflow',
      slug: 'delta_pro_ultra_x',
      glyph: { icon: 'home-battery-outline', label: 'DPU-X' }
    };
  }

  if (m.includes('delta pro ultra')) {
    return {
      assetFamily: 'ecoflow',
      slug: 'delta_pro_ultra',
      glyph: { icon: 'home-battery-outline', label: 'DPU' }
    };
  }

  if (m.includes('delta pro 3')) {
    return { assetFamily: 'ecoflow', slug: 'delta_pro_3', glyph: { icon: 'battery-high', label: 'DP3' } };
  }

  if (m.includes('delta 2 max')) {
    return {
      assetFamily: 'ecoflow',
      slug: (options?.batteryCount ?? 0) > 1 ? 'delta_2_max_plus_battery' : 'delta_2_max',
      glyph: { icon: 'package-variant-closed', label: 'D2M' }
    };
  }

  if (m.includes('delta 3 ultra')) {
    return { assetFamily: 'ecoflow', slug: 'delta_3_ultra', glyph: { icon: 'lightning-bolt', label: 'D3U' } };
  }

  if (m.includes('delta 3 max')) {
    return { assetFamily: 'ecoflow', slug: 'delta_3_max', glyph: { icon: 'lightning-bolt', label: 'D3M' } };
  }

  if (m.includes('delta 3 plus')) {
    return { assetFamily: 'ecoflow', slug: 'delta_3_plus', glyph: { icon: 'lightning-bolt', label: 'D3+' } };
  }

  if (m.includes('delta 3')) {
    return { assetFamily: 'ecoflow', slug: 'delta_3', glyph: { icon: 'lightning-bolt', label: 'D3' } };
  }

  if (m.includes('delta pro')) {
    return { assetFamily: 'ecoflow', slug: 'delta_pro', glyph: { icon: 'battery-high', label: 'DP' } };
  }

  if (m.includes('delta')) {
    return { glyph: { icon: 'battery-high', label: 'Delta' } };
  }

  if (m.includes('river')) {
    return { glyph: { icon: 'waves', label: 'River' } };
  }

  return { glyph: { icon: 'puzzle-outline', label: 'Device' } };
}

function getPecronAssetMatch(compact: string, normalized: string): DeviceAssetMatch | null {
  const slugByModel: Array<[string, PecronDeviceSlug]> = [
    ['e1000lfp', 'pecron_e1000lfp'],
    ['e1500lfp', 'pecron_e1500lfp'],
    ['e2000lfp', 'pecron_e2000lfp'],
    ['e2400lfp', 'pecron_e2400lfp'],
    ['e300lfp', 'pecron_e300lfp'],
    ['e3600lfp', 'pecron_e3600lfp'],
    ['e3800lfp', 'pecron_e3800lfp'],
    ['e500lfp', 'pecron_e500lfp'],
    ['e600lfp', 'pecron_e600lfp'],
    ['f1000lfp', 'pecron_f1000lfp'],
    ['f3000lfp', 'pecron_f3000lfp'],
    ['f5000lfp', 'pecron_f5000lfp']
  ];
  for (const [needle, slug] of slugByModel) {
    if (compact.includes(needle)) {
      return {
        assetFamily: 'pecron',
        slug,
        glyph: { icon: 'battery-charging-wireless-outline', label: 'Pecron' }
      };
    }
  }
  if (normalized.includes('pecron')) {
    return { glyph: { icon: 'battery-charging-wireless-outline', label: 'Pecron' } };
  }
  return null;
}
