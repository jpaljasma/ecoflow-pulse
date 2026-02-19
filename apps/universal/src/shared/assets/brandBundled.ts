import type { ImageSourcePropType } from 'react-native';

import logoWordmarkBlack from '../../../public/ecoflow_brand_assets/ecoflow_logo_wordmark.png';
import logoWordmarkWhite from '../../../public/ecoflow_brand_assets/ecoflow_logo_wordmark_white.png';
import logoMarkBlack from '../../../public/ecoflow_brand_assets/ecoflow_f_logo_256.png';
import logoMarkWhite from '../../../public/ecoflow_brand_assets/ecoflow_f_logo_256_white.png';

export function getBundledBrandWordmark(theme: 'light' | 'dark'): ImageSourcePropType {
  return theme === 'dark' ? logoWordmarkWhite : logoWordmarkBlack;
}

export function getBundledBrandMark(theme: 'light' | 'dark'): ImageSourcePropType {
  return theme === 'dark' ? logoMarkWhite : logoMarkBlack;
}
