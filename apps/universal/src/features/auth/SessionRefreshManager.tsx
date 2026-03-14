import * as AuthSession from 'expo-auth-session';
import { useEffect, useRef } from 'react';
import { AppState, type AppStateStatus, Platform } from 'react-native';
import { readOidcConfig } from '@/features/auth/oidcConfig';
import { canRefreshSession, shouldRefreshSession } from '@/features/auth/sessionRefresh';
import { useAuthStore } from '@/features/auth/store';

const REFRESH_POLL_MS = 15_000;
const TERMINAL_REFRESH_ERROR_PATTERNS = ['invalid_grant', 'invalid refresh token', 'refresh token'];

export function SessionRefreshManager() {
  const hydrated = useAuthStore((state) => state.hydrated);
  const refreshing = useAuthStore((state) => state.refreshing);
  const session = useAuthStore((state) => state.session);
  const setSession = useAuthStore((state) => state.setSession);
  const clearSession = useAuthStore((state) => state.clearSession);
  const setRefreshing = useAuthStore((state) => state.setRefreshing);
  const refreshPromiseRef = useRef<Promise<void> | null>(null);

  useEffect(() => {
    if (!hydrated) {
      return;
    }

    const tryRefresh = async () => {
      const cfg = readOidcConfig();
      const latest = useAuthStore.getState().session;
      if (!cfg || !latest) {
        return;
      }
      if (latest.issuerUrl !== cfg.issuerUrl || latest.clientId !== cfg.clientId) {
        return;
      }
      if (!shouldRefreshSession(latest, Date.now()) || refreshPromiseRef.current) {
        return;
      }

      const refresh = (async () => {
        setRefreshing(true);
        try {
          const discovery = await AuthSession.fetchDiscoveryAsync(cfg.issuerUrl);
          const token = await AuthSession.refreshAsync(
            {
              clientId: cfg.clientId,
              refreshToken: latest.refreshToken,
              scopes: cfg.scopes
            },
            discovery
          );
          setSession({
            issuerUrl: cfg.issuerUrl,
            clientId: cfg.clientId,
            token: {
              ...token,
              refreshToken: token.refreshToken ?? latest.refreshToken
            }
          });
        } catch (error) {
          const message = error instanceof Error ? error.message.toLowerCase() : String(error).toLowerCase();
          if (TERMINAL_REFRESH_ERROR_PATTERNS.some((pattern) => message.includes(pattern))) {
            clearSession();
            return;
          }
          setRefreshing(false);
        } finally {
          refreshPromiseRef.current = null;
        }
      })();

      refreshPromiseRef.current = refresh;
      await refresh;
    };

    const interval = setInterval(() => {
      void tryRefresh();
    }, REFRESH_POLL_MS);

    const onAppStateChange = (state: AppStateStatus) => {
      if (state === 'active') {
        void tryRefresh();
      }
    };

    const appStateSubscription = AppState.addEventListener('change', onAppStateChange);

    let removeWebListeners = () => undefined;
    if (Platform.OS === 'web' && typeof window !== 'undefined') {
      const onWindowFocus = () => {
        void tryRefresh();
      };
      const onVisibilityChange = () => {
        if (document.visibilityState === 'visible') {
          void tryRefresh();
        }
      };
      window.addEventListener('focus', onWindowFocus);
      document.addEventListener('visibilitychange', onVisibilityChange);
      removeWebListeners = () => {
        window.removeEventListener('focus', onWindowFocus);
        document.removeEventListener('visibilitychange', onVisibilityChange);
      };
    }

    if (canRefreshSession(session) && !refreshing) {
      void tryRefresh();
    }

    return () => {
      clearInterval(interval);
      appStateSubscription.remove();
      removeWebListeners();
    };
  }, [clearSession, hydrated, refreshing, session, setRefreshing, setSession]);

  return null;
}
