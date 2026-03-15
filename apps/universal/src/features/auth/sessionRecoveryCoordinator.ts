import * as AuthSession from 'expo-auth-session';
import { readOidcConfig } from '@/features/auth/oidcConfig';
import { reportAuthSessionRecovery } from '@/features/auth/sessionRecoveryMetrics';
import { canRefreshSession } from '@/features/auth/sessionRefresh';
import { isSessionExpired, useAuthStore } from '@/features/auth/store';

const TERMINAL_REFRESH_ERROR_PATTERNS = ['invalid_grant', 'invalid refresh token', 'refresh token'];

let refreshPromise: Promise<string | null> | null = null;

function getConfiguredSessionSnapshot() {
  const oidcConfig = readOidcConfig();
  const { session } = useAuthStore.getState();
  if (!oidcConfig || !session) {
    return null;
  }
  if (session.issuerUrl !== oidcConfig.issuerUrl || session.clientId !== oidcConfig.clientId) {
    return null;
  }
  return {
    oidcConfig,
    session
  };
}

function getLatestUsableAccessToken(staleToken?: string): string | null {
  const snapshot = getConfiguredSessionSnapshot();
  if (!snapshot) {
    return null;
  }
  if (!snapshot.session.accessToken || isSessionExpired(snapshot.session, Date.now())) {
    return null;
  }
  if (snapshot.session.accessToken === staleToken) {
    return null;
  }
  return snapshot.session.accessToken;
}

async function refreshAccessToken(): Promise<string | null> {
  if (refreshPromise) {
    return refreshPromise;
  }

  refreshPromise = (async () => {
    const snapshot = getConfiguredSessionSnapshot();
    if (!snapshot || !canRefreshSession(snapshot.session)) {
      return null;
    }

    const { setRefreshing, setSession } = useAuthStore.getState();
    setRefreshing(true);
    try {
      const discovery = await AuthSession.fetchDiscoveryAsync(snapshot.oidcConfig.issuerUrl);
      const token = await AuthSession.refreshAsync(
        {
          clientId: snapshot.oidcConfig.clientId,
          refreshToken: snapshot.session.refreshToken,
          scopes: snapshot.oidcConfig.scopes
        },
        discovery
      );
      setSession({
        issuerUrl: snapshot.oidcConfig.issuerUrl,
        clientId: snapshot.oidcConfig.clientId,
        token: {
          ...token,
          refreshToken: token.refreshToken ?? snapshot.session.refreshToken
        }
      });
      return token.accessToken ?? null;
    } catch (error) {
      const message = error instanceof Error ? error.message.toLowerCase() : String(error).toLowerCase();
      if (TERMINAL_REFRESH_ERROR_PATTERNS.some((pattern) => message.includes(pattern))) {
        return null;
      }
      return getLatestUsableAccessToken();
    } finally {
      useAuthStore.getState().setRefreshing(false);
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

export async function recoverSessionForUnauthorizedRequest(staleToken?: string): Promise<string | null> {
  const latestToken = getLatestUsableAccessToken(staleToken);
  if (latestToken) {
    void reportAuthSessionRecovery('recovered_in_memory');
    return latestToken;
  }
  const refreshedToken = await refreshAccessToken();
  if (refreshedToken) {
    void reportAuthSessionRecovery('recovered_refresh');
  }
  return refreshedToken;
}

export function triggerSessionExpiredRedirect() {
  const state = useAuthStore.getState();
  if (state.reauthRequest.reason) {
    return;
  }
  void reportAuthSessionRecovery('reauth_redirect');
  state.requestReauthentication('session_expired');
}
