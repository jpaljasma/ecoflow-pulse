import { readOidcConfig } from '@/features/auth/oidcConfig';
import { isSessionExpired, useAuthStore } from '@/features/auth/store';

export function useAuthSession() {
  const hydrated = useAuthStore((state) => state.hydrated);
  const session = useAuthStore((state) => state.session);
  const authConfigured = Boolean(readOidcConfig());
  const sessionValid =
    Boolean(session?.accessToken) && !isSessionExpired(session, Date.now());
  const token = sessionValid ? session?.accessToken : undefined;

  return {
    authConfigured,
    authReady: !authConfigured || hydrated,
    sessionValid,
    token,
    authKey: sessionValid
      ? `session:${session?.updatedAtUnixMs ?? 0}`
      : authConfigured
        ? hydrated
          ? 'auth-required'
          : 'auth-pending'
        : 'anonymous'
  };
}
