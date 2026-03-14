import { useEffect } from 'react';
import { Platform } from 'react-native';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TamaguiProvider, Theme } from 'tamagui';
import {
  Roboto_400Regular,
  Roboto_500Medium,
  Roboto_700Bold,
  useFonts
} from '@expo-google-fonts/roboto';
import {
  Inter_400Regular,
  Inter_500Medium,
  Inter_700Bold,
  Inter_800ExtraBold
} from '@expo-google-fonts/inter';
import { SessionRefreshManager } from '@/features/auth/SessionRefreshManager';
import { TelemetryEngineProvider } from '@/features/telemetry/TelemetryEngineContext';
import { tamaguiConfig } from '@/shared/ui/theme';
import { useAppTheme } from '@/shared/theme/useAppTheme';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false
    }
  }
});

export function AppProvider({ children }: { children: React.ReactNode }) {
  const [fontsLoaded] = useFonts({
    Roboto_400Regular,
    Roboto_500Medium,
    Roboto_700Bold,
    Inter_400Regular,
    Inter_500Medium,
    Inter_700Bold,
    Inter_800ExtraBold
  });
  const { hydrated, variant, spec, isDark } = useAppTheme();
  const rootTheme = isDark ? 'dark' : 'light';

  useEffect(() => {
    // Ensure query cache starts warm and stable before heavy telemetry arrives.
    queryClient.resumePausedMutations().catch(() => undefined);
  }, []);

  useEffect(() => {
    if (Platform.OS !== 'web' || typeof document === 'undefined') {
      return;
    }

    const root = document.documentElement;
    const body = document.body;
    const appRoot = document.getElementById('root');

    root.style.backgroundColor = spec.colors.background;
    root.style.colorScheme = isDark ? 'dark' : 'light';
    body.style.backgroundColor = spec.colors.background;
    body.style.color = spec.colors.color;
    if (appRoot) {
      appRoot.style.backgroundColor = spec.colors.background;
      appRoot.style.color = spec.colors.color;
    }

    root.dataset.pulseTheme = variant;
    return () => {
      delete root.dataset.pulseTheme;
    };
  }, [isDark, spec, variant]);

  if (!fontsLoaded || !hydrated) return null;

  return (
    <TamaguiProvider config={tamaguiConfig} defaultTheme={rootTheme} themeClassNameOnRoot>
      <Theme name={variant} forceClassName>
        <QueryClientProvider client={queryClient}>
          <SessionRefreshManager />
          <TelemetryEngineProvider>{children}</TelemetryEngineProvider>
        </QueryClientProvider>
      </Theme>
    </TamaguiProvider>
  );
}
