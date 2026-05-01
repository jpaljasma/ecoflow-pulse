import type { ImageSourcePropType } from 'react-native';
import type { DeviceSummary } from '@/features/devices/api';
import { getDeviceAssetMatch, type DeviceAssetMatch } from '@/features/devices/deviceIcon';
import { getEcoFlowAsset, getEcoFlowDefaultSize } from '@/shared/assets/ecoflowAssets';
import { getBundledDeviceFallback } from '@/shared/assets/deviceFallbacks';

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
    imageUri:
      useRemoteImage && match.slug
        ? getEcoFlowAsset(match.slug, getEcoFlowDefaultSize(imageContext))
        : undefined,
    fallbackSource: match.slug ? getBundledDeviceFallback(match.slug, '256') : undefined
  };
}
