import { useEffect } from 'react';
import { useGlobalSearchParams, usePathname, useRouter } from 'expo-router';
import { buildReturnTo } from '@/features/auth/useReturnTo';
import { useAuthStore } from '@/features/auth/store';

export function SessionRecoveryRedirector() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useGlobalSearchParams();
  const reauthRequest = useAuthStore((state) => state.reauthRequest);
  const clearSession = useAuthStore((state) => state.clearSession);
  const clearReauthenticationRequest = useAuthStore((state) => state.clearReauthenticationRequest);

  useEffect(() => {
    if (!reauthRequest.reason) {
      return;
    }

    clearSession();
    const returnTo =
      pathname && pathname !== '/login' && pathname !== '/auth/callback'
        ? buildReturnTo(pathname, params)
        : undefined;
    router.replace({
      pathname: '/login',
      params: {
        ...(returnTo ? { returnTo } : {}),
        reason: reauthRequest.reason
      }
    });
    clearReauthenticationRequest();
  }, [
    clearReauthenticationRequest,
    clearSession,
    params,
    pathname,
    reauthRequest.reason,
    reauthRequest.nonce,
    router
  ]);

  return null;
}
