import type { ImageSourcePropType } from 'react-native';
import type { DeviceSummary } from '@/features/devices/api';
import { getDeviceAssetMatch, type DeviceAssetMatch } from '@/features/devices/deviceIcon';
import { getEcoFlowAsset, getEcoFlowDefaultSize, type EcoFlowDeviceSlug } from '@/shared/assets/ecoflowAssets';
import { getBundledDeviceFallback } from '@/shared/assets/deviceFallbacks';
import { getPecronAsset, getPecronDefaultSize, type PecronDeviceSlug } from '@/shared/assets/pecronAssets';

type DeviceImageContext = Parameters<typeof getEcoFlowDefaultSize>[0];

export type DeviceVisualAssets = {
  match: DeviceAssetMatch;
  imageUri?: string;
  fallbackSource?: ImageSourcePropType;
};

export function getDeviceBatteryCount(device: DeviceSummary): number {
  return (
    device.details?.bpCount ??
    ((device.capabilities as { batteryPacks?: number } | undefined)?.batteryPacks ?? 1)
  );
}

export function resolveDeviceVisualAssets(
  device: DeviceSummary,
  {
    useRemoteImage,
    imageContext = 'card'
  }: {
    useRemoteImage: boolean;
    imageContext?: DeviceImageContext;
  }
): DeviceVisualAssets {
  const match = getDeviceAssetMatch(device.model, {
    batteryCount: getDeviceBatteryCount(device)
  });

  return {
    match,
    imageUri: useRemoteImage ? getDeviceVisualImageUri(match, imageContext) : undefined,
    fallbackSource: getDeviceVisualFallbackSource(match, '256')
  };
}

export function getDeviceVisualImageUri(
  match: DeviceAssetMatch,
  imageContext: DeviceImageContext = 'card'
): string | undefined {
  if (!match.slug || !match.assetFamily) {
    return undefined;
  }
  switch (match.assetFamily) {
    case 'pecron':
      return getPecronAsset(match.slug as PecronDeviceSlug, getPecronDefaultSize(imageContext));
    case 'ecoflow':
      return getEcoFlowAsset(match.slug as EcoFlowDeviceSlug, getEcoFlowDefaultSize(imageContext));
    default:
      return undefined;
  }
}

export function getDeviceVisualFallbackSource(
  match: DeviceAssetMatch,
  size: '256' | '512' = '256'
) {
  if (!match.slug) {
    return undefined;
  }
  return getBundledDeviceFallback(match.slug, size);
}
