import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { Image as ExpoImage } from 'expo-image';
import type { ImageSourcePropType, DimensionValue } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import { CachedImage } from '@/shared/ui/CachedImage';

export function DeviceHeroPanel({
  stacked = false,
  leftWidth,
  imageWidth,
  imageHeight,
  imageScale = 1.08,
  imageOffsetY = 0,
  imageUri,
  fallbackSource,
  emojiFallback,
  leftMeta,
  leftFooter,
  right
}: {
  stacked?: boolean;
  leftWidth: DimensionValue;
  imageWidth: number;
  imageHeight: number;
  imageScale?: number;
  imageOffsetY?: number;
  imageUri?: string;
  fallbackSource?: ImageSourcePropType;
  emojiFallback?: string;
  leftMeta?: ReactNode;
  leftFooter?: ReactNode;
  right: ReactNode;
}) {
  const [imageFailed, setImageFailed] = useState(false);

  useEffect(() => {
    setImageFailed(false);
  }, [imageUri, fallbackSource]);

  return (
    <XStack gap="$3" alignItems="stretch" flexDirection={stacked ? 'column' : 'row'}>
      <YStack style={{ width: leftWidth }} flexShrink={0} alignItems="center" gap="$3">
        <YStack
          width={imageWidth}
          height={imageHeight}
          marginTop={2}
          alignItems="center"
          justifyContent="center"
          borderRadius="$4"
          backgroundColor="rgba(120,120,128,0.12)"
          overflow="hidden"
        >
          {imageUri && !imageFailed ? (
            <CachedImage
              uri={imageUri}
              style={{
                width: imageWidth * imageScale,
                height: imageHeight * imageScale,
                transform: imageOffsetY ? [{ translateY: imageOffsetY }] : undefined
              }}
              contentFit="cover"
              onError={() => setImageFailed(true)}
            />
          ) : fallbackSource ? (
            <ExpoImage
              source={fallbackSource}
              style={{
                width: imageWidth * imageScale,
                height: imageHeight * imageScale,
                transform: imageOffsetY ? [{ translateY: imageOffsetY }] : undefined
              }}
              contentFit="cover"
            />
          ) : (
            <Text fontSize="$10">{emojiFallback ?? '🧩'}</Text>
          )}
        </YStack>
        {leftMeta}
        {leftFooter}
      </YStack>
      <YStack flex={1} minWidth={0}>
        {right}
      </YStack>
    </XStack>
  );
}
