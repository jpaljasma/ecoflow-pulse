import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { SessionRecoveryRedirector } from '@/features/auth/SessionRecoveryRedirector';
import { AppProvider } from '@/shared/providers/AppProvider';
import { useAppTheme } from '@/shared/theme/useAppTheme';

export default function RootLayout() {
  const { isDark } = useAppTheme();

  return (
    <AppProvider>
      <StatusBar style={isDark ? 'light' : 'dark'} />
      <SessionRecoveryRedirector />
      <Stack screenOptions={{ headerShown: false }}>
        <Stack.Screen name="index" />
        <Stack.Screen name="auth/callback" />
        <Stack.Screen name="login" />
        <Stack.Screen name="onboarding" options={{ animation: 'slide_from_right' }} />
        <Stack.Screen name="(tabs)" />
        <Stack.Screen name="device/[deviceId]" options={{ animation: 'flip' }} />
        <Stack.Screen name="profile" options={{ animation: 'slide_from_right' }} />
      </Stack>
    </AppProvider>
  );
}
