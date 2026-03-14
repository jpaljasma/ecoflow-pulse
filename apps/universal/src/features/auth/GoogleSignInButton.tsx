import { Image, Platform, Pressable } from 'react-native';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import googleSignInDark from '../../../assets/auth/google-signin-dark.png';
import googleSignInLight from '../../../assets/auth/google-signin-light.png';

const GOOGLE_BUTTON_ASPECT_RATIO = 350 / 80;

export function GoogleSignInButton({
  disabled = false,
  maxWidth = 340,
  onPress
}: {
  disabled?: boolean;
  maxWidth?: number;
  onPress: () => void;
}) {
  const { isDark } = useAppTheme();
  const source = isDark ? googleSignInDark : googleSignInLight;

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel="Sign in with Google"
      accessibilityState={{ disabled }}
      disabled={disabled}
      onPress={onPress}
      style={({ pressed }) => [
        {
          width: '100%',
          maxWidth,
          alignSelf: 'center',
          opacity: disabled ? 0.55 : pressed ? 0.92 : 1
        },
        Platform.OS === 'web'
          ? ({
              cursor: disabled ? 'not-allowed' : 'pointer'
            } as any)
          : null
      ]}
    >
      <Image
        source={source}
        resizeMode="contain"
        style={{
          width: '100%',
          aspectRatio: GOOGLE_BUTTON_ASPECT_RATIO
        }}
      />
    </Pressable>
  );
}
