import type { ImageSourcePropType } from 'react-native';
import type { EcoFlowDeviceSlug } from '@/shared/assets/ecoflowAssets';

import d2m256 from '../../../public/ecoflow_delta_assets/processed/crop_256/delta_2_max_256.png';
import d2mPlus256 from '../../../public/ecoflow_delta_assets/processed/crop_256/delta_2_max_plus_battery_256.png';
import d3256 from '../../../public/ecoflow_delta_assets/processed/crop_256/delta_3_256.png';
import d3Max256 from '../../../public/ecoflow_delta_assets/processed/crop_256/delta_3_max_256.png';
import d3Plus256 from '../../../public/ecoflow_delta_assets/processed/crop_256/delta_3_plus_256.png';
import d3Ultra256 from '../../../public/ecoflow_delta_assets/processed/crop_256/delta_3_ultra_256.png';
import dp256 from '../../../public/ecoflow_delta_assets/processed/crop_256/delta_pro_256.png';
import dp3256 from '../../../public/ecoflow_delta_assets/processed/crop_256/delta_pro_3_256.png';
import dpu256 from '../../../public/ecoflow_delta_assets/processed/crop_256/delta_pro_ultra_256.png';
import dpuX256 from '../../../public/ecoflow_delta_assets/processed/crop_256/delta_pro_ultra_x_256.png';

import d2m512 from '../../../public/ecoflow_delta_assets/processed/crop_512/delta_2_max_512.png';
import d2mPlus512 from '../../../public/ecoflow_delta_assets/processed/crop_512/delta_2_max_plus_battery_512.png';
import d3512 from '../../../public/ecoflow_delta_assets/processed/crop_512/delta_3_512.png';
import d3Max512 from '../../../public/ecoflow_delta_assets/processed/crop_512/delta_3_max_512.png';
import d3Plus512 from '../../../public/ecoflow_delta_assets/processed/crop_512/delta_3_plus_512.png';
import d3Ultra512 from '../../../public/ecoflow_delta_assets/processed/crop_512/delta_3_ultra_512.png';
import dp512 from '../../../public/ecoflow_delta_assets/processed/crop_512/delta_pro_512.png';
import dp3512 from '../../../public/ecoflow_delta_assets/processed/crop_512/delta_pro_3_512.png';
import dpu512 from '../../../public/ecoflow_delta_assets/processed/crop_512/delta_pro_ultra_512.png';
import dpuX512 from '../../../public/ecoflow_delta_assets/processed/crop_512/delta_pro_ultra_x_512.png';

type FallbackSize = '256' | '512';

const FALLBACKS_256: Record<EcoFlowDeviceSlug, ImageSourcePropType> = {
  delta_2_max: d2m256,
  delta_2_max_plus_battery: d2mPlus256,
  delta_3: d3256,
  delta_3_max: d3Max256,
  delta_3_plus: d3Plus256,
  delta_3_ultra: d3Ultra256,
  delta_pro: dp256,
  delta_pro_3: dp3256,
  delta_pro_ultra: dpu256,
  delta_pro_ultra_x: dpuX256
};

const FALLBACKS_512: Record<EcoFlowDeviceSlug, ImageSourcePropType> = {
  delta_2_max: d2m512,
  delta_2_max_plus_battery: d2mPlus512,
  delta_3: d3512,
  delta_3_max: d3Max512,
  delta_3_plus: d3Plus512,
  delta_3_ultra: d3Ultra512,
  delta_pro: dp512,
  delta_pro_3: dp3512,
  delta_pro_ultra: dpu512,
  delta_pro_ultra_x: dpuX512
};

export function getBundledDeviceFallback(
  slug: EcoFlowDeviceSlug,
  size: FallbackSize = '256'
): ImageSourcePropType {
  return size === '512' ? FALLBACKS_512[slug] : FALLBACKS_256[slug];
}
