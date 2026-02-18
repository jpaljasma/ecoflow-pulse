export type EcoFlowLogoVariant = 'black' | 'white';
export type EcoFlowLogoSize = 'mark' | 'wordmark' | '1024' | '512' | '256';

export const ECOFLOW_BRAND = {
  black: {
    mark: '/ecoflow_brand_assets/ecoflow_f_logo_256.png',
    wordmark: '/ecoflow_brand_assets/ecoflow_logo_wordmark.png',
    '1024': '/ecoflow_brand_assets/ecoflow_logo_1024.png',
    '512': '/ecoflow_brand_assets/ecoflow_logo_512.png',
    '256': '/ecoflow_brand_assets/ecoflow_logo_256.png'
  },
  white: {
    mark: '/ecoflow_brand_assets/ecoflow_f_logo_256_white.png',
    wordmark: '/ecoflow_brand_assets/ecoflow_logo_wordmark_white.png',
    '1024': '/ecoflow_brand_assets/ecoflow_logo_1024_white.png',
    '512': '/ecoflow_brand_assets/ecoflow_logo_512_white.png',
    '256': '/ecoflow_brand_assets/ecoflow_logo_256_white.png'
  }
} as const;

export function getEcoFlowLogo(
  size: EcoFlowLogoSize = '512',
  variant: EcoFlowLogoVariant = 'black'
): string {
  const entry = ECOFLOW_BRAND[variant];
  return entry[size] ?? entry['512'];
}

export function getEcoFlowLogoForTheme(
  theme: 'light' | 'dark',
  size: EcoFlowLogoSize = '512'
): string {
  return getEcoFlowLogo(size, theme === 'dark' ? 'white' : 'black');
}
