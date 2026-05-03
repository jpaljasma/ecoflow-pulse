import { createElement } from 'react';
import type { CSSProperties } from 'react';
import { Platform } from 'react-native';
import { YStack } from 'tamagui';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import {
  buildPulseHeroBackgroundImageProps,
  buildPulseHeroBackgroundPictureSources
} from '@/shared/ui/PulseHeroBackground.model';

const imageLayerStyle: CSSProperties = {
  position: 'absolute',
  top: 0,
  right: 0,
  bottom: 0,
  left: 0,
  width: '100%',
  height: '100%',
  objectFit: 'cover',
  objectPosition: 'right bottom',
  pointerEvents: 'none',
  userSelect: 'none'
};

const pictureLayerStyle: CSSProperties = {
  position: 'absolute',
  top: 0,
  right: 0,
  bottom: 0,
  left: 0,
  zIndex: 0,
  pointerEvents: 'none',
  overflow: 'hidden'
};

const fallbackBackground = {
  dark: '#0d1622',
  light: '#e6f5ff'
} as const;

export function PulseHeroBackground({
  sizes = '100vw',
  variant = 'fleet'
}: {
  sizes?: string;
  variant?: 'fleet' | 'device';
}) {
  const { isDark } = useAppTheme();

  if (Platform.OS !== 'web') {
    return null;
  }

  const imageProps = buildPulseHeroBackgroundImageProps(sizes);
  const pictureSources = buildPulseHeroBackgroundPictureSources(sizes);

  return (
    <>
      <YStack
        pointerEvents="none"
        position="absolute"
        top={0}
        right={0}
        bottom={0}
        left={0}
        zIndex={0}
        style={{ backgroundColor: isDark ? fallbackBackground.dark : fallbackBackground.light }}
      />
      {createElement(
        'picture',
        {
          'aria-hidden': true,
          style: pictureLayerStyle
        },
        ...pictureSources.map((source) => createElement('source', { key: source.type, ...source })),
        createElement('img', {
          ...imageProps,
          alt: '',
          decoding: 'async',
          draggable: false,
          loading: 'eager',
          style: imageLayerStyle
        })
      )}
      <YStack
        pointerEvents="none"
        position="absolute"
        top={0}
        right={0}
        bottom={0}
        left={0}
        zIndex={0}
        style={{ backgroundImage: resolvePulseHeroOverlay(variant, isDark) }}
      />
    </>
  );
}

function resolvePulseHeroOverlay(variant: 'fleet' | 'device', isDark: boolean): string {
  if (isDark) {
    const rightAlpha = variant === 'fleet' ? 0.42 : 0.5;
    return [
      `linear-gradient(90deg, rgba(13, 22, 34, 0.94) 0%, rgba(13, 22, 34, 0.86) 38%, rgba(13, 22, 34, 0.66) 68%, rgba(13, 22, 34, ${rightAlpha}) 100%)`,
      'radial-gradient(circle at 78% 70%, rgba(57, 189, 248, 0.18) 0%, rgba(57, 189, 248, 0) 44%)'
    ].join(', ');
  }

  const centerAlpha = variant === 'fleet' ? 0.56 : 0.62;
  const rightAlpha = variant === 'fleet' ? 0.3 : 0.38;
  return [
    `linear-gradient(90deg, rgba(239, 250, 255, 0.92) 0%, rgba(239, 250, 255, 0.8) 36%, rgba(239, 250, 255, ${centerAlpha}) 68%, rgba(239, 250, 255, ${rightAlpha}) 100%)`,
    'radial-gradient(circle at 78% 70%, rgba(255, 255, 255, 0.24) 0%, rgba(255, 255, 255, 0) 42%)'
  ].join(', ');
}
