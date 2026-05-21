import { Platform } from 'react-native';
import { env } from '@/shared/config/env';

/**
 * Pecron portable power station product image assets (official Pecron product media).
 */

export type PecronDeviceSlug =
  | 'pecron_e1000lfp'
  | 'pecron_e1500lfp'
  | 'pecron_e2000lfp'
  | 'pecron_e2400lfp'
  | 'pecron_e300lfp'
  | 'pecron_e3600lfp'
  | 'pecron_e3800lfp'
  | 'pecron_e500lfp'
  | 'pecron_e600lfp'
  | 'pecron_f1000lfp'
  | 'pecron_f3000lfp'
  | 'pecron_f5000lfp';

export type PecronImageSize = 'original' | '1024' | '512' | '256';

export type PecronAssetEntry = {
  original: string;
  '1024': string;
  '512': string;
  '256': string;
};

export const PECRON_ASSETS: Record<PecronDeviceSlug, PecronAssetEntry> = {
  pecron_e1000lfp: {
    original: 'pecron_assets/processed/original_png/pecron_e1000lfp.png',
    '1024': 'pecron_assets/processed/crop_1024/pecron_e1000lfp_1024.png',
    '512': 'pecron_assets/processed/crop_512/pecron_e1000lfp_512.png',
    '256': 'pecron_assets/processed/crop_256/pecron_e1000lfp_256.png'
  },
  pecron_e1500lfp: {
    original: 'pecron_assets/processed/original_png/pecron_e1500lfp.png',
    '1024': 'pecron_assets/processed/crop_1024/pecron_e1500lfp_1024.png',
    '512': 'pecron_assets/processed/crop_512/pecron_e1500lfp_512.png',
    '256': 'pecron_assets/processed/crop_256/pecron_e1500lfp_256.png'
  },
  pecron_e2000lfp: {
    original: 'pecron_assets/processed/original_png/pecron_e2000lfp.png',
    '1024': 'pecron_assets/processed/crop_1024/pecron_e2000lfp_1024.png',
    '512': 'pecron_assets/processed/crop_512/pecron_e2000lfp_512.png',
    '256': 'pecron_assets/processed/crop_256/pecron_e2000lfp_256.png'
  },
  pecron_e2400lfp: {
    original: 'pecron_assets/processed/original_png/pecron_e2400lfp.png',
    '1024': 'pecron_assets/processed/crop_1024/pecron_e2400lfp_1024.png',
    '512': 'pecron_assets/processed/crop_512/pecron_e2400lfp_512.png',
    '256': 'pecron_assets/processed/crop_256/pecron_e2400lfp_256.png'
  },
  pecron_e300lfp: {
    original: 'pecron_assets/processed/original_png/pecron_e300lfp.png',
    '1024': 'pecron_assets/processed/crop_1024/pecron_e300lfp_1024.png',
    '512': 'pecron_assets/processed/crop_512/pecron_e300lfp_512.png',
    '256': 'pecron_assets/processed/crop_256/pecron_e300lfp_256.png'
  },
  pecron_e3600lfp: {
    original: 'pecron_assets/processed/original_png/pecron_e3600lfp.png',
    '1024': 'pecron_assets/processed/crop_1024/pecron_e3600lfp_1024.png',
    '512': 'pecron_assets/processed/crop_512/pecron_e3600lfp_512.png',
    '256': 'pecron_assets/processed/crop_256/pecron_e3600lfp_256.png'
  },
  pecron_e3800lfp: {
    original: 'pecron_assets/processed/original_png/pecron_e3800lfp.png',
    '1024': 'pecron_assets/processed/crop_1024/pecron_e3800lfp_1024.png',
    '512': 'pecron_assets/processed/crop_512/pecron_e3800lfp_512.png',
    '256': 'pecron_assets/processed/crop_256/pecron_e3800lfp_256.png'
  },
  pecron_e500lfp: {
    original: 'pecron_assets/processed/original_png/pecron_e500lfp.png',
    '1024': 'pecron_assets/processed/crop_1024/pecron_e500lfp_1024.png',
    '512': 'pecron_assets/processed/crop_512/pecron_e500lfp_512.png',
    '256': 'pecron_assets/processed/crop_256/pecron_e500lfp_256.png'
  },
  pecron_e600lfp: {
    original: 'pecron_assets/processed/original_png/pecron_e600lfp.png',
    '1024': 'pecron_assets/processed/crop_1024/pecron_e600lfp_1024.png',
    '512': 'pecron_assets/processed/crop_512/pecron_e600lfp_512.png',
    '256': 'pecron_assets/processed/crop_256/pecron_e600lfp_256.png'
  },
  pecron_f1000lfp: {
    original: 'pecron_assets/processed/original_png/pecron_f1000lfp.png',
    '1024': 'pecron_assets/processed/crop_1024/pecron_f1000lfp_1024.png',
    '512': 'pecron_assets/processed/crop_512/pecron_f1000lfp_512.png',
    '256': 'pecron_assets/processed/crop_256/pecron_f1000lfp_256.png'
  },
  pecron_f3000lfp: {
    original: 'pecron_assets/processed/original_png/pecron_f3000lfp.png',
    '1024': 'pecron_assets/processed/crop_1024/pecron_f3000lfp_1024.png',
    '512': 'pecron_assets/processed/crop_512/pecron_f3000lfp_512.png',
    '256': 'pecron_assets/processed/crop_256/pecron_f3000lfp_256.png'
  },
  pecron_f5000lfp: {
    original: 'pecron_assets/processed/original_png/pecron_f5000lfp.png',
    '1024': 'pecron_assets/processed/crop_1024/pecron_f5000lfp_1024.png',
    '512': 'pecron_assets/processed/crop_512/pecron_f5000lfp_512.png',
    '256': 'pecron_assets/processed/crop_256/pecron_f5000lfp_256.png'
  }
} as const;

export function getPecronAsset(slug: PecronDeviceSlug, size: PecronImageSize = '512'): string {
  const entry = PECRON_ASSETS[slug];
  const path = entry[size] ?? entry['512'];
  if (Platform.OS === 'web' && !env.assetBaseUrl) {
    return `/${path}`;
  }
  const base =
    env.assetBaseUrl ||
    'https://raw.githubusercontent.com/jpaljasma/ecoflow-pulse/main/apps/universal/public';
  return `${base.replace(/\/+$/, '')}/${path.replace(/^\/+/, '')}`;
}

export function getPecronDefaultSize(
  context: 'list' | 'card' | 'detail' = 'card'
): PecronImageSize {
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
