export type DeviceGlyph = {
  emoji: string;
  label: string;
};

export function getDeviceGlyph(model: string): DeviceGlyph {
  const m = model.toLowerCase();

  if (m.includes('delta pro ultra')) {
    return { emoji: '🏛️', label: 'DPU' };
  }

  if (m.includes('delta 2 max')) {
    return { emoji: '📦', label: 'D2M' };
  }

  if (m.includes('delta pro')) {
    return { emoji: '🔋', label: 'DP' };
  }

  if (m.includes('delta')) {
    return { emoji: '🔋', label: 'Delta' };
  }

  if (m.includes('river')) {
    return { emoji: '🌊', label: 'River' };
  }

  return { emoji: '🧩', label: 'EcoFlow' };
}
