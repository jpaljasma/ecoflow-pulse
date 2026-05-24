import { useEffect, useRef, useState } from 'react';
import { Animated, Easing, Platform, useWindowDimensions } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import {
  getSocSweepConfig,
  SOC_SWEEP_DELAY_MS,
  SOC_SWEEP_DURATION_MS,
  SOC_SWEEP_WIDTH_RATIO,
  type SocSweepMode
} from '@/shared/ui/SocBarMotion';
import { usePrefersReducedMotion } from '@/shared/ui/usePrefersReducedMotion';

function clamp(value: number): number {
  if (Number.isNaN(value)) return 0;
  return Math.max(0, Math.min(100, value));
}

export function SocBar({
  value,
  fullWidth = false,
  sweepMode = 'idle'
}: {
  value: number | null | undefined;
  fullWidth?: boolean;
  sweepMode?: SocSweepMode;
}) {
  const { width } = useWindowDimensions();
  const isTabletUp = width >= 768;
  const pct = clamp(value ?? 0);
  const barWidth = fullWidth ? '100%' : isTabletUp ? '50%' : '100%';
  const minBarWidth = fullWidth ? 0 : isTabletUp ? 220 : 0;
  const prefersReducedMotion = usePrefersReducedMotion();
  const sweepConfig = getSocSweepConfig(sweepMode);
  const sweepProgress = useRef(new Animated.Value(0)).current;
  const [fillWidth, setFillWidth] = useState(0);
  const sweepWidth = Math.max(12, fillWidth * SOC_SWEEP_WIDTH_RATIO);
  const sweepEnabled = sweepConfig.enabled && pct > 0 && fillWidth > 0 && !prefersReducedMotion;

  useEffect(() => {
    if (!sweepEnabled) {
      sweepProgress.stopAnimation();
      sweepProgress.setValue(0);
      return;
    }

    const animation = Animated.loop(
      Animated.sequence([
        Animated.delay(SOC_SWEEP_DELAY_MS),
        Animated.timing(sweepProgress, {
          toValue: 1,
          duration: SOC_SWEEP_DURATION_MS,
          easing: Easing.inOut(Easing.cubic),
          useNativeDriver: Platform.OS !== 'web'
        })
      ]),
      { resetBeforeIteration: true }
    );
    animation.start();
    return () => animation.stop();
  }, [sweepEnabled, sweepProgress]);

  return (
    <YStack gap="$2" width={barWidth} minWidth={minBarWidth}>
      <XStack alignItems="center" justifyContent="space-between">
        <Text fontFamily="$body" fontSize="$3" opacity={0.78} fontWeight="500">
          SOC
        </Text>
        <Text fontFamily="$body" fontSize="$3" fontWeight="700">
          {Number.isFinite(value as number) ? `${pct.toFixed(1)}%` : '—'}
        </Text>
      </XStack>
      <XStack
        height={10}
        borderRadius="$5"
        overflow="hidden"
        backgroundColor="rgba(120,120,128,0.20)"
      >
        <XStack
          position="relative"
          height="100%"
          width={`${pct}%` as `${number}%`}
          backgroundColor={pct >= 60 ? '#30d158' : pct >= 30 ? '#ff9f0a' : '#ff453a'}
          overflow="hidden"
          onLayout={(event) => setFillWidth(event.nativeEvent.layout.width)}
        >
          {sweepEnabled ? (
            <Animated.View
              pointerEvents="none"
              style={{
                position: 'absolute',
                top: -8,
                bottom: -8,
                left: 0,
                width: sweepWidth,
                backgroundColor: sweepConfig.overlayColor,
                transform: [
                  { translateX: sweepProgress.interpolate({ inputRange: [0, 1], outputRange: [-sweepWidth, fillWidth + sweepWidth] }) },
                  { rotate: '45deg' }
                ]
              }}
            />
          ) : null}
        </XStack>
      </XStack>
    </YStack>
  );
}
