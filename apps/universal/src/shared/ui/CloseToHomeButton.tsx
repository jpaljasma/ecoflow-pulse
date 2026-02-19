import { useRouter } from 'expo-router';
import { Animated, Platform } from 'react-native';
import { useRef, useState } from 'react';
import { Button, Text } from 'tamagui';
import { env } from '@/shared/config/env';

export function CloseToHomeButton({
  onClose
}: {
  onClose?: () => void;
}) {
  const router = useRouter();
  const [closing, setClosing] = useState(false);
  const pulse = useRef(new Animated.Value(0)).current;
  const animationMode = (env.closeButtonAnimation ?? 'subtle').toLowerCase();

  return (
    <Animated.View
      style={{
        transform: [
          {
            scale: pulse.interpolate({
              inputRange: [0, 1],
              outputRange: [1, 0.94]
            })
          }
        ],
        opacity: pulse.interpolate({
          inputRange: [0, 1],
          outputRange: [1, 0.88]
        })
      }}
    >
      <Button
        width={46}
        height={46}
        borderRadius={23}
        borderWidth={1}
        borderColor="rgba(120,120,128,0.32)"
        backgroundColor="rgba(120,120,128,0.10)"
        alignItems="center"
        justifyContent="center"
        pressStyle={{ scale: 0.97, opacity: 0.9 }}
        onPress={() => {
          if (closing) return;
          if (animationMode === 'off' || animationMode === 'none') {
            if (onClose) {
              onClose();
            } else {
              router.replace('/(tabs)/devices');
            }
            return;
          }
          setClosing(true);
          Animated.sequence([
            Animated.timing(pulse, {
              toValue: 1,
              duration: 90,
              useNativeDriver: Platform.OS !== 'web'
            }),
            Animated.timing(pulse, {
              toValue: 0,
              duration: 90,
              useNativeDriver: Platform.OS !== 'web'
            })
          ]).start(() => {
            pulse.setValue(0);
            setClosing(false);
            if (onClose) {
              onClose();
            } else {
              router.replace('/(tabs)/devices');
            }
          });
        }}
        accessibilityRole="button"
        accessibilityLabel="Close and return to devices"
      >
        <Text fontSize="$7" lineHeight={24} fontWeight="700" marginTop={-1}>
          ×
        </Text>
      </Button>
    </Animated.View>
  );
}
