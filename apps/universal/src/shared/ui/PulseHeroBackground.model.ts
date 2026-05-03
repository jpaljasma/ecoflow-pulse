const PULSE_HERO_BACKGROUND_WIDTHS = [1280, 2048, 4096] as const;

const PULSE_HERO_BACKGROUND_FORMATS = [
  { extension: 'avif', type: 'image/avif' },
  { extension: 'webp', type: 'image/webp' }
] as const;

export function buildPulseHeroBackgroundImageProps(sizes = '100vw') {
  return {
    src: buildPulseHeroBackgroundUrl('webp', PULSE_HERO_BACKGROUND_WIDTHS[0]),
    srcSet: buildPulseHeroBackgroundSrcSet('webp'),
    sizes
  };
}

export function buildPulseHeroBackgroundPictureSources(sizes = '100vw') {
  return PULSE_HERO_BACKGROUND_FORMATS.map((format) => ({
    srcSet: buildPulseHeroBackgroundSrcSet(format.extension),
    sizes,
    type: format.type
  }));
}

function buildPulseHeroBackgroundSrcSet(extension: 'avif' | 'webp'): string {
  return PULSE_HERO_BACKGROUND_WIDTHS.map((width) => `${buildPulseHeroBackgroundUrl(extension, width)} ${width}w`).join(', ');
}

function buildPulseHeroBackgroundUrl(extension: 'avif' | 'webp', width: number): string {
  return `/pulse-backgrounds/pulse-hero-lines-${width}.${extension}`;
}
