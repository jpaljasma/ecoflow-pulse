import { readOidcConfig } from '@/features/auth/oidcConfig';
import { shouldBlockOnSessionRecovery } from '@/features/auth/sessionRefresh';
import { isSessionExpired, useAuthStore } from '@/features/auth/store';

export function useAuthSession() {
  const hydrated = useAuthStore((state) => state.hydrated);
  const refreshing = useAuthStore((state) => state.refreshing);
  const session = useAuthStore((state) => state.session);
  const oidcConfig = readOidcConfig();
  const authConfigured = Boolean(oidcConfig);
  const sessionMatchesConfig =
    !oidcConfig ||
    (session?.issuerUrl === oidcConfig.issuerUrl && session?.clientId === oidcConfig.clientId);
  const sessionRecovering =
    hydrated && sessionMatchesConfig && shouldBlockOnSessionRecovery(session, Date.now());
  const sessionValid =
    sessionMatchesConfig && Boolean(session?.accessToken) && !isSessionExpired(session, Date.now());
  const token = sessionValid ? session?.accessToken : undefined;

  return {
    authConfigured,
    authReady: !authConfigured || (hydrated && !sessionRecovering),
    refreshing,
    sessionValid,
    token,
    authKey: sessionValid
      ? `session:${session?.updatedAtUnixMs ?? 0}`
      : authConfigured && sessionRecovering
        ? 'auth-refreshing'
        : authConfigured
        ? hydrated
          ? 'auth-required'
          : 'auth-pending'
        : 'anonymous'
  };
}
