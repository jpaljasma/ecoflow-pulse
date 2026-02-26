import * as AuthSession from 'expo-auth-session';
import * as WebBrowser from 'expo-web-browser';
import { useEffect, useMemo, useState } from 'react';
import { Button, Spinner, Text, XStack, YStack } from 'tamagui';
import { readOidcConfig, type OidcConfig } from '@/features/auth/oidcConfig';
import { isSessionExpired, useAuthStore, type StoredOidcSession } from '@/features/auth/store';
import { Card } from '@/shared/ui/Card';

WebBrowser.maybeCompleteAuthSession();

type AuthActions = Pick<ReturnType<typeof useAuthStore.getState>, 'setSession' | 'clearSession'>;

export function KeycloakPkceCard() {
  const cfg = readOidcConfig();
  const session = useAuthStore((state) => state.session);
  const setSession = useAuthStore((state) => state.setSession);
  const clearSession = useAuthStore((state) => state.clearSession);

  if (!cfg) {
    return (
      <Card gap="$2">
        <Text fontSize="$5" fontWeight="700">
          Authentication (PKCE)
        </Text>
        <Text opacity={0.75}>
          Set `EXPO_PUBLIC_OIDC_ISSUER_URL` and `EXPO_PUBLIC_OIDC_CLIENT_ID` to enable Keycloak login.
        </Text>
      </Card>
    );
  }

  return (
    <ConfiguredPkceCard
      cfg={cfg}
      session={session}
      actions={{
        setSession,
        clearSession
      }}
    />
  );
}

type ConfiguredPkceCardProps = {
  cfg: OidcConfig;
  session: StoredOidcSession | null;
  actions: AuthActions;
};

function ConfiguredPkceCard({ cfg, session, actions }: ConfiguredPkceCardProps) {
  const [busy, setBusy] = useState(false);
  const [statusText, setStatusText] = useState('');
  const [errorText, setErrorText] = useState('');

  const redirectUri = useMemo(
    () =>
      AuthSession.makeRedirectUri({
        scheme: 'ecoflowpulse',
        path: 'auth/callback'
      }),
    []
  );

  const discovery = AuthSession.useAutoDiscovery(cfg.issuerUrl);
  const [request, response, promptAsync] = AuthSession.useAuthRequest(
    {
      clientId: cfg.clientId,
      scopes: cfg.scopes,
      redirectUri,
      usePKCE: true,
      responseType: AuthSession.ResponseType.Code,
      extraParams: cfg.audience ? { audience: cfg.audience } : undefined
    },
    discovery
  );

  useEffect(() => {
    let cancelled = false;
    const run = async () => {
      if (!response) return;
      if (response.type === 'dismiss') return;
      if (response.type === 'error') {
        setErrorText(response.error?.message ?? 'Login failed');
        return;
      }
      if (response.type !== 'success') return;
      if (!discovery?.tokenEndpoint) {
        setErrorText('Missing OIDC token endpoint');
        return;
      }
      if (!request?.codeVerifier) {
        setErrorText('Missing PKCE verifier');
        return;
      }
      const code = response.params.code;
      if (!code) {
        setErrorText('Missing auth code');
        return;
      }

      setBusy(true);
      setErrorText('');
      setStatusText('Exchanging code…');
      try {
        const token = await AuthSession.exchangeCodeAsync(
          {
            clientId: cfg.clientId,
            code,
            redirectUri,
            extraParams: {
              code_verifier: request.codeVerifier
            }
          },
          discovery
        );
        if (cancelled) return;
        actions.setSession({
          issuerUrl: cfg.issuerUrl,
          clientId: cfg.clientId,
          token
        });
        setStatusText('Signed in');
      } catch (err) {
        if (cancelled) return;
        setErrorText(err instanceof Error ? err.message : 'Token exchange failed');
      } finally {
        if (!cancelled) {
          setBusy(false);
        }
      }
    };
    void run();
    return () => {
      cancelled = true;
    };
  }, [actions, cfg.clientId, cfg.issuerUrl, discovery, redirectUri, request, response]);

  const authenticated =
    session?.issuerUrl === cfg.issuerUrl &&
    session?.clientId === cfg.clientId &&
    !isSessionExpired(session, Date.now()) &&
    !!session.accessToken;
  const allowRefresh = !!session?.refreshToken && !!discovery?.tokenEndpoint;

  const onSignIn = async () => {
    setErrorText('');
    setStatusText('');
    await promptAsync();
  };

  const onRefresh = async () => {
    if (!allowRefresh || !session || !discovery) return;
    setBusy(true);
    setErrorText('');
    setStatusText('Refreshing token…');
    try {
      const token = await AuthSession.refreshAsync(
        {
          clientId: cfg.clientId,
          refreshToken: session.refreshToken,
          scopes: cfg.scopes
        },
        discovery
      );
      actions.setSession({
        issuerUrl: cfg.issuerUrl,
        clientId: cfg.clientId,
        token: {
          ...token,
          refreshToken: token.refreshToken ?? session.refreshToken
        }
      });
      setStatusText('Token refreshed');
    } catch (err) {
      setErrorText(err instanceof Error ? err.message : 'Token refresh failed');
    } finally {
      setBusy(false);
    }
  };

  const onSignOut = () => {
    actions.clearSession();
    setStatusText('Signed out');
    setErrorText('');
  };

  return (
    <Card gap="$2">
      <Text fontSize="$5" fontWeight="700">
        Authentication (PKCE)
      </Text>
      <Text opacity={0.75}>Keycloak Authorization Code + PKCE for Expo Web/iOS/Android.</Text>
      <Text opacity={0.75}>Issuer: {cfg.issuerUrl}</Text>
      <Text opacity={0.75}>Client: {cfg.clientId}</Text>
      <Text style={{ color: authenticated ? '#15803d' : '#6b7280' }}>
        {authenticated ? 'Session: signed in' : 'Session: signed out'}
      </Text>
      {statusText ? (
        <Text style={{ color: '#6b7280' }} fontSize="$2">
          {statusText}
        </Text>
      ) : null}
      {errorText ? (
        <Text style={{ color: '#dc2626' }} fontSize="$2">
          {errorText}
        </Text>
      ) : null}
      <XStack gap="$2" flexWrap="wrap" alignItems="center">
        <Button
          size="$3"
          onPress={() => {
            void onSignIn();
          }}
          disabled={busy || !request || !discovery}
        >
          Sign in
        </Button>
        <Button
          size="$3"
          onPress={() => {
            void onRefresh();
          }}
          disabled={busy || !allowRefresh}
        >
          Refresh token
        </Button>
        <Button size="$3" onPress={onSignOut} disabled={busy || !session}>
          Sign out
        </Button>
        {busy ? (
          <YStack paddingLeft="$2">
            <Spinner size="small" color="#f59e0b" />
          </YStack>
        ) : null}
      </XStack>
    </Card>
  );
}
