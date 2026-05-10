import { Image, StyleSheet } from 'react-native';

import { PULSE_MARK_ICON_SOURCE } from '@/shared/ui/pulseMarkAsset';

export function PulseMark({ size = 42 }: { size?: number }) {
  return (
    <Image
      source={PULSE_MARK_ICON_SOURCE}
      resizeMode="contain"
      accessibilityIgnoresInvertColors
      style={[
        styles.icon,
        {
          width: size,
          height: size,
          borderRadius: Math.round(size * 0.24)
        }
      ]}
    />
  );
}

const styles = StyleSheet.create({
  icon: {
    overflow: 'hidden'
  }
});
