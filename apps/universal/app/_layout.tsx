import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { AppProvider } from '@/shared/providers/AppProvider';
import { useAppTheme } from '@/shared/theme/useAppTheme';

export default function RootLayout() {
  const { isDark } = useAppTheme();

  return (
    <AppProvider>
      <StatusBar style={isDark ? 'light' : 'dark'} />
      <Stack screenOptions={{ headerShown: false }}>
        <Stack.Screen name="(tabs)" />
        <Stack.Screen name="device/[deviceId]" options={{ animation: 'flip' }} />
      </Stack>
    </AppProvider>
  );
}
