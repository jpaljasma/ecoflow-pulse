import { useCallback, useMemo, useRef } from 'react';
import { Animated, Platform } from 'react-native';
import type { Router } from 'expo-router';
import { env } from '@/shared/config/env';

type CloseTransitionMode = 'off' | 'subtle' | 'flip';

function resolveMode(): CloseTransitionMode {
  const raw = (env.closePageTransition ?? 'subtle').toLowerCase();
  if (raw === 'off' || raw === 'none') return 'off';
  if (raw === 'flip') return 'flip';
  return 'subtle';
}

export function useCloseToHomeTransition(router: Router) {
  const progress = useRef(new Animated.Value(0)).current;
  const mode = resolveMode();
  const duration = Math.max(120, Number(env.closePageTransitionMs) || 220);

  const containerStyle = useMemo(() => {
    if (mode === 'off') {
      return { flex: 1 } as const;
    }

    if (mode === 'flip') {
      return {
        flex: 1,
        transform: [
          { perspective: 1200 },
          {
            rotateY: progress.interpolate({
              inputRange: [0, 1],
              outputRange: ['0deg', '72deg']
            })
          }
        ],
        opacity: progress.interpolate({
          inputRange: [0, 1],
          outputRange: [1, 0.2]
        })
      };
    }

    return {
      flex: 1,
      transform: [
        {
          translateY: progress.interpolate({
            inputRange: [0, 1],
            outputRange: [0, 8]
          })
        },
        {
          scale: progress.interpolate({
            inputRange: [0, 1],
            outputRange: [1, 0.985]
          })
        }
      ],
      opacity: progress.interpolate({
        inputRange: [0, 1],
        outputRange: [1, 0.88]
      })
    };
  }, [mode, progress]);

  const closeToHome = useCallback(() => {
    if (mode === 'off') {
      router.replace('/(tabs)/devices');
      return;
    }

    Animated.timing(progress, {
      toValue: 1,
      duration,
      useNativeDriver: Platform.OS !== 'web'
    }).start(() => {
      progress.setValue(0);
      router.replace('/(tabs)/devices');
    });
  }, [duration, mode, progress, router]);

  return { containerStyle, closeToHome };
}
