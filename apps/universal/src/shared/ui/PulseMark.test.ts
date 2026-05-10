import { describe, expect, it } from 'vitest';

import canonicalAppIcon from '../../../assets/icon.png';
import { PULSE_MARK_ICON_SOURCE } from './pulseMarkAsset';

describe('PulseMark', () => {
  it('uses the bundled app icon asset as the shared product mark', () => {
    expect(PULSE_MARK_ICON_SOURCE).toBe(canonicalAppIcon);
  });
});
