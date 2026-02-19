import { Platform } from 'react-native';
import { env } from '@/shared/config/env';

/**
 * EcoFlow DELTA-family product image assets (official EcoFlow sources).
 *
 * Generated from ecoflow_delta_assets/manifest.json and adapted for app use.
 */

export type EcoFlowDeviceSlug =
  | 'delta_2_max'
  | 'delta_2_max_plus_battery'
  | 'delta_3'
  | 'delta_3_max'
  | 'delta_3_plus'
  | 'delta_3_ultra'
  | 'delta_pro'
  | 'delta_pro_3'
  | 'delta_pro_ultra'
  | 'delta_pro_ultra_x';

export type EcoFlowImageSize = 'original' | '1024' | '512' | '256' | 'original_webp';

export type EcoFlowAssetEntry = {
  original: string;
  original_webp?: string;
  '1024': string;
  '512': string;
  '256': string;
};

export const ECOFLOW_ASSETS: Record<EcoFlowDeviceSlug, EcoFlowAssetEntry> = {
  delta_2_max: {
    original: 'ecoflow_delta_assets/processed/original_png/delta_2_max.png',
    original_webp: 'ecoflow_delta_assets/originals/delta_2_max.webp',
    '1024': 'ecoflow_delta_assets/processed/crop_1024/delta_2_max_1024.png',
    '512': 'ecoflow_delta_assets/processed/crop_512/delta_2_max_512.png',
    '256': 'ecoflow_delta_assets/processed/crop_256/delta_2_max_256.png'
  },
  delta_2_max_plus_battery: {
    original: 'ecoflow_delta_assets/processed/original_png/delta_2_max_plus_battery.png',
    original_webp: 'ecoflow_delta_assets/originals/delta_2_max_plus_battery.webp',
    '1024': 'ecoflow_delta_assets/processed/crop_1024/delta_2_max_plus_battery_1024.png',
    '512': 'ecoflow_delta_assets/processed/crop_512/delta_2_max_plus_battery_512.png',
    '256': 'ecoflow_delta_assets/processed/crop_256/delta_2_max_plus_battery_256.png'
  },
  delta_3: {
    original: 'ecoflow_delta_assets/processed/original_png/delta_3.png',
    original_webp: 'ecoflow_delta_assets/originals/delta_3.webp',
    '1024': 'ecoflow_delta_assets/processed/crop_1024/delta_3_1024.png',
    '512': 'ecoflow_delta_assets/processed/crop_512/delta_3_512.png',
    '256': 'ecoflow_delta_assets/processed/crop_256/delta_3_256.png'
  },
  delta_3_max: {
    original: 'ecoflow_delta_assets/processed/original_png/delta_3_max.png',
    original_webp: 'ecoflow_delta_assets/originals/delta_3_max.webp',
    '1024': 'ecoflow_delta_assets/processed/crop_1024/delta_3_max_1024.png',
    '512': 'ecoflow_delta_assets/processed/crop_512/delta_3_max_512.png',
    '256': 'ecoflow_delta_assets/processed/crop_256/delta_3_max_256.png'
  },
  delta_3_plus: {
    original: 'ecoflow_delta_assets/processed/original_png/delta_3_plus.png',
    original_webp: 'ecoflow_delta_assets/originals/delta_3_plus.webp',
    '1024': 'ecoflow_delta_assets/processed/crop_1024/delta_3_plus_1024.png',
    '512': 'ecoflow_delta_assets/processed/crop_512/delta_3_plus_512.png',
    '256': 'ecoflow_delta_assets/processed/crop_256/delta_3_plus_256.png'
  },
  delta_3_ultra: {
    original: 'ecoflow_delta_assets/processed/original_png/delta_3_ultra.png',
    original_webp: 'ecoflow_delta_assets/originals/delta_3_ultra.webp',
    '1024': 'ecoflow_delta_assets/processed/crop_1024/delta_3_ultra_1024.png',
    '512': 'ecoflow_delta_assets/processed/crop_512/delta_3_ultra_512.png',
    '256': 'ecoflow_delta_assets/processed/crop_256/delta_3_ultra_256.png'
  },
  delta_pro: {
    original: 'ecoflow_delta_assets/processed/original_png/delta_pro.png',
    original_webp: 'ecoflow_delta_assets/originals/delta_pro.webp',
    '1024': 'ecoflow_delta_assets/processed/crop_1024/delta_pro_1024.png',
    '512': 'ecoflow_delta_assets/processed/crop_512/delta_pro_512.png',
    '256': 'ecoflow_delta_assets/processed/crop_256/delta_pro_256.png'
  },
  delta_pro_3: {
    original: 'ecoflow_delta_assets/processed/original_png/delta_pro_3.png',
    original_webp: 'ecoflow_delta_assets/originals/delta_pro_3.webp',
    '1024': 'ecoflow_delta_assets/processed/crop_1024/delta_pro_3_1024.png',
    '512': 'ecoflow_delta_assets/processed/crop_512/delta_pro_3_512.png',
    '256': 'ecoflow_delta_assets/processed/crop_256/delta_pro_3_256.png'
  },
  delta_pro_ultra: {
    original: 'ecoflow_delta_assets/processed/original_png/delta_pro_ultra.png',
    original_webp: 'ecoflow_delta_assets/originals/delta_pro_ultra.webp',
    '1024': 'ecoflow_delta_assets/processed/crop_1024/delta_pro_ultra_1024.png',
    '512': 'ecoflow_delta_assets/processed/crop_512/delta_pro_ultra_512.png',
    '256': 'ecoflow_delta_assets/processed/crop_256/delta_pro_ultra_256.png'
  },
  delta_pro_ultra_x: {
    original: 'ecoflow_delta_assets/processed/original_png/delta_pro_ultra_x.png',
    original_webp: 'ecoflow_delta_assets/originals/delta_pro_ultra_x.webp',
    '1024': 'ecoflow_delta_assets/processed/crop_1024/delta_pro_ultra_x_1024.png',
    '512': 'ecoflow_delta_assets/processed/crop_512/delta_pro_ultra_x_512.png',
    '256': 'ecoflow_delta_assets/processed/crop_256/delta_pro_ultra_x_256.png'
  }
} as const;

export function getEcoFlowAsset(
  slug: EcoFlowDeviceSlug,
  size: EcoFlowImageSize = '512'
): string {
  const entry = ECOFLOW_ASSETS[slug];
  const path = entry[size] ?? entry['512'];
  if (Platform.OS === 'web' && !env.assetBaseUrl) {
    return `/${path}`;
  }
  const base =
    env.assetBaseUrl ||
    'https://raw.githubusercontent.com/jpaljasma/ecoflow-pulse/main/apps/universal/public';
  return `${base.replace(/\/+$/, '')}/${path.replace(/^\/+/, '')}`;
}

export function getEcoFlowDefaultSize(
  context: 'list' | 'card' | 'detail' = 'card'
): EcoFlowImageSize {
  switch (context) {
    case 'list':
      return '256';
    case 'detail':
      return '1024';
    case 'card':
    default:
      return '512';
  }
}
