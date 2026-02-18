import { useEffect } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TamaguiProvider, Theme } from 'tamagui';
import { useColorScheme } from 'react-native';
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

  useEffect(() => {
    // Ensure query cache starts warm and stable before heavy telemetry arrives.
    queryClient.resumePausedMutations().catch(() => undefined);
  }, []);

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
