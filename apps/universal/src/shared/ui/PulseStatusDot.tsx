import { useEffect, useRef } from 'react';
import { Animated, Platform } from 'react-native';

export function PulseStatusDot({
  size = 16,
  color = 'rgba(34, 197, 94, 0.96)'
}: {
  size?: number;
  color?: string;
}) {
  const pulse = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    const animation = Animated.loop(
      Animated.sequence([
        Animated.timing(pulse, {
          toValue: 1,
          duration: 900,
          useNativeDriver: Platform.OS !== 'web'
        }),
        Animated.timing(pulse, {
          toValue: 0,
          duration: 900,
          useNativeDriver: Platform.OS !== 'web'
        })
      ])
    );
    animation.start();
    return () => {
      animation.stop();
    };
  }, [pulse]);

  const outerSize = size;
  const innerSize = Math.max(8, Math.round(size * 0.82));

  return (
    <Animated.View
      style={{
        width: outerSize,
        height: outerSize,
        borderRadius: outerSize / 2,
        alignItems: 'center',
        justifyContent: 'center',
        backgroundColor: `${color}2a`,
        transform: [
          {
            scale: pulse.interpolate({
              inputRange: [0, 1],
              outputRange: [1, 1.18]
            })
          }
        ],
        opacity: pulse.interpolate({
          inputRange: [0, 1],
          outputRange: [0.78, 1]
        })
      }}
    >
      <Animated.View
        style={{
          width: innerSize,
          height: innerSize,
          borderRadius: innerSize / 2,
          backgroundColor: color
        }}
      />
    </Animated.View>
  );
}
