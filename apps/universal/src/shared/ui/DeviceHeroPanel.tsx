import { useEffect, useState } from 'react';
import type { ComponentProps, ReactNode } from 'react';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Image as ExpoImage } from 'expo-image';
import type { ImageSourcePropType, DimensionValue } from 'react-native';
import { XStack, YStack } from 'tamagui';
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
  iconFallback,
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
  iconFallback?: ComponentProps<typeof MaterialCommunityIcons>['name'];
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
            <MaterialCommunityIcons name={iconFallback ?? 'puzzle-outline'} size={42} color="rgba(28, 43, 45, 0.62)" />
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
