import { describe, expect, it } from 'vitest';

import {
  buildPulseHeroBackgroundImageProps,
  buildPulseHeroBackgroundPictureSources
} from '@/shared/ui/PulseHeroBackground.model';

describe('PulseHeroBackground', () => {
  it('builds responsive WebP image props for hero surfaces', () => {
    expect(buildPulseHeroBackgroundImageProps('(min-width: 1280px) 1180px, 100vw')).toEqual({
      src: '/pulse-backgrounds/pulse-hero-lines-1280.webp',
      srcSet:
        '/pulse-backgrounds/pulse-hero-lines-1280.webp 1280w, /pulse-backgrounds/pulse-hero-lines-2048.webp 2048w, /pulse-backgrounds/pulse-hero-lines-4096.webp 4096w',
      sizes: '(min-width: 1280px) 1180px, 100vw'
    });
  });

  it('prefers responsive AVIF sources with WebP fallback', () => {
    expect(buildPulseHeroBackgroundPictureSources('100vw')).toEqual([
      {
        srcSet:
          '/pulse-backgrounds/pulse-hero-lines-1280.avif 1280w, /pulse-backgrounds/pulse-hero-lines-2048.avif 2048w, /pulse-backgrounds/pulse-hero-lines-4096.avif 4096w',
        sizes: '100vw',
        type: 'image/avif'
      },
      {
        srcSet:
          '/pulse-backgrounds/pulse-hero-lines-1280.webp 1280w, /pulse-backgrounds/pulse-hero-lines-2048.webp 2048w, /pulse-backgrounds/pulse-hero-lines-4096.webp 4096w',
        sizes: '100vw',
        type: 'image/webp'
      }
    ]);
  });
});
