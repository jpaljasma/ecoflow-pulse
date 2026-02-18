import { useEffect } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TamaguiProvider, Theme } from 'tamagui';
import { useColorScheme } from 'react-native';
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
import { TelemetryEngineProvider } from '@/features/telemetry/TelemetryEngineContext';
import { tamaguiConfig } from '@/shared/ui/theme';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false
    }
  }
});

export function AppProvider({ children }: { children: React.ReactNode }) {
  const scheme = useColorScheme();
  const [fontsLoaded] = useFonts({
    Roboto_400Regular,
    Roboto_500Medium,
    Roboto_700Bold,
    Inter_400Regular,
    Inter_500Medium,
    Inter_700Bold,
    Inter_800ExtraBold
  });

  useEffect(() => {
    // Ensure query cache starts warm and stable before heavy telemetry arrives.
    queryClient.resumePausedMutations().catch(() => undefined);
  }, []);

  if (!fontsLoaded) return null;

  return (
    <TamaguiProvider config={tamaguiConfig} defaultTheme={scheme === 'dark' ? 'dark' : 'light'}>
      <Theme name={scheme === 'dark' ? 'dark' : 'light'}>
        <QueryClientProvider client={queryClient}>
          <TelemetryEngineProvider>{children}</TelemetryEngineProvider>
        </QueryClientProvider>
      </Theme>
    </TamaguiProvider>
  );
}
