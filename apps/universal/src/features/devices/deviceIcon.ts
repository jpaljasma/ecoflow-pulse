import type { ComponentProps } from 'react';
import { MaterialCommunityIcons } from '@expo/vector-icons';

export type DeviceGlyph = {
  icon: ComponentProps<typeof MaterialCommunityIcons>['name'];
  label: string;
};

export type DeviceAssetMatch = {
  slug?:
    | 'delta_2_max'
    | 'delta_2_max_plus_battery'
    | 'delta_3'
    | 'delta_3_plus'
    | 'delta_3_max'
    | 'delta_3_ultra'
    | 'delta_pro'
    | 'delta_pro_3'
    | 'delta_pro_ultra'
    | 'delta_pro_ultra_x';
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

  if (m.includes('delta pro ultra x')) {
    return { slug: 'delta_pro_ultra_x', glyph: { icon: 'home-battery-outline', label: 'DPU-X' } };
  }

  if (m.includes('delta pro ultra')) {
    return { slug: 'delta_pro_ultra', glyph: { icon: 'home-battery-outline', label: 'DPU' } };
  }

  if (m.includes('delta pro 3')) {
    return { slug: 'delta_pro_3', glyph: { icon: 'battery-high', label: 'DP3' } };
  }

  if (m.includes('delta 2 max')) {
    return {
      slug: (options?.batteryCount ?? 0) > 1 ? 'delta_2_max_plus_battery' : 'delta_2_max',
      glyph: { icon: 'package-variant-closed', label: 'D2M' }
    };
  }

  if (m.includes('delta 3 ultra')) {
    return { slug: 'delta_3_ultra', glyph: { icon: 'lightning-bolt', label: 'D3U' } };
  }

  if (m.includes('delta 3 max')) {
    return { slug: 'delta_3_max', glyph: { icon: 'lightning-bolt', label: 'D3M' } };
  }

  if (m.includes('delta 3 plus')) {
    return { slug: 'delta_3_plus', glyph: { icon: 'lightning-bolt', label: 'D3+' } };
  }

  if (m.includes('delta 3')) {
    return { slug: 'delta_3', glyph: { icon: 'lightning-bolt', label: 'D3' } };
  }

  if (m.includes('delta pro')) {
    return { slug: 'delta_pro', glyph: { icon: 'battery-high', label: 'DP' } };
  }

  if (m.includes('delta')) {
    return { glyph: { icon: 'battery-high', label: 'Delta' } };
  }

  if (m.includes('river')) {
    return { glyph: { icon: 'waves', label: 'River' } };
  }

  return { glyph: { icon: 'puzzle-outline', label: 'EcoFlow' } };
}
