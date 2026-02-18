export type DeviceGlyph = {
  emoji: string;
  label: string;
};

export type DeviceAssetMatch = {
  slug?: 'delta_2_max' | 'delta_3' | 'delta_3_plus' | 'delta_3_max' | 'delta_3_ultra' | 'delta_pro' | 'delta_pro_3' | 'delta_pro_ultra' | 'delta_pro_ultra_x';
  glyph: DeviceGlyph;
};

export function getDeviceGlyph(model: string): DeviceGlyph {
  return getDeviceAssetMatch(model).glyph;
}

export function getDeviceAssetMatch(model: string): DeviceAssetMatch {
  const m = model.toLowerCase();

  if (m.includes('delta pro ultra x')) {
    return { slug: 'delta_pro_ultra_x', glyph: { emoji: '🏛️', label: 'DPU-X' } };
  }

  if (m.includes('delta pro ultra')) {
    return { slug: 'delta_pro_ultra', glyph: { emoji: '🏛️', label: 'DPU' } };
  }

  if (m.includes('delta pro 3')) {
    return { slug: 'delta_pro_3', glyph: { emoji: '🔋', label: 'DP3' } };
  }

  if (m.includes('delta 2 max')) {
    return { slug: 'delta_2_max', glyph: { emoji: '📦', label: 'D2M' } };
  }

  if (m.includes('delta 3 ultra')) {
    return { slug: 'delta_3_ultra', glyph: { emoji: '⚡', label: 'D3U' } };
  }

  if (m.includes('delta 3 max')) {
    return { slug: 'delta_3_max', glyph: { emoji: '⚡', label: 'D3M' } };
  }

  if (m.includes('delta 3 plus')) {
    return { slug: 'delta_3_plus', glyph: { emoji: '⚡', label: 'D3+' } };
  }

  if (m.includes('delta 3')) {
    return { slug: 'delta_3', glyph: { emoji: '⚡', label: 'D3' } };
  }

  if (m.includes('delta pro')) {
    return { slug: 'delta_pro', glyph: { emoji: '🔋', label: 'DP' } };
  }

  if (m.includes('delta')) {
    return { glyph: { emoji: '🔋', label: 'Delta' } };
  }

  if (m.includes('river')) {
    return { glyph: { emoji: '🌊', label: 'River' } };
  }

  return { glyph: { emoji: '🧩', label: 'EcoFlow' } };
}
