import { useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { Button } from 'tamagui';
import { readOidcConfig } from '@/features/auth/oidcConfig';
import { performLogout } from '@/features/auth/logout';
import { useAuthStore } from '@/features/auth/store';
import { useTelemetryEngine } from '@/features/telemetry/TelemetryEngineContext';
import { useTelemetryStore } from '@/features/telemetry/store';

export function LogoutButton({ onComplete }: { onComplete?: () => void } = {}) {
  const clearSession = useAuthStore((state) => state.clearSession);
  const session = useAuthStore((state) => state.session);
  const queryClient = useQueryClient();
  const telemetryEngine = useTelemetryEngine();
  const resetTelemetry = useTelemetryStore((state) => state.reset);

  return (
    <Button
      size="$5"
      justifyContent="flex-start"
      onPress={() => {
        void performLogout({
          disconnectRealtime: () => telemetryEngine.disconnect(),
          resetTelemetry,
          clearSession,
          clearQueries: () => queryClient.clear(),
          onComplete,
          navigateHome: () => router.replace('/'),
          session,
          oidcConfig: readOidcConfig()
        });
      }}
    >
      Log out
    </Button>
  );
}
