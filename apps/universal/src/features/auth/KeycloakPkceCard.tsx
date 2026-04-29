import * as AuthSession from 'expo-auth-session';
import * as WebBrowser from 'expo-web-browser';
import { useEffect, useMemo, useState } from 'react';
import { Button, Spinner, Text, XStack, YStack } from 'tamagui';
import { readOidcConfig, type OidcConfig } from '@/features/auth/oidcConfig';
import { isSessionExpired, useAuthStore, type StoredOidcSession } from '@/features/auth/store';
import {
  beginFullPageWebAuthRedirect,
  shouldUseFullPageWebAuthRedirect
} from '@/features/auth/webPkceRedirect';
import { Card } from '@/shared/ui/Card';

WebBrowser.maybeCompleteAuthSession();

type AuthActions = Pick<ReturnType<typeof useAuthStore.getState>, 'setSession' | 'clearSession'>;

export function KeycloakPkceCard() {
  return <KeycloakPkceCardWithVariant variant="settings" />;
}

export function LoginCard() {
  return <KeycloakPkceCardWithVariant variant="login" />;
}

function KeycloakPkceCardWithVariant({ variant }: { variant: 'settings' | 'login' }) {
  const cfg = readOidcConfig();
  const session = useAuthStore((state) => state.session);
  const setSession = useAuthStore((state) => state.setSession);
  const clearSession = useAuthStore((state) => state.clearSession);
  const loginVariant = variant === 'login';

  if (!cfg) {
    return (
      <Card gap="$2">
        <Text fontSize="$5" fontWeight="700">
          {loginVariant ? 'Sign in is unavailable' : 'Authentication (PKCE)'}
        </Text>
        <Text opacity={0.75}>
          Set the OIDC issuer and client ID for the selected connection profile to enable Keycloak login.
        </Text>
      </Card>
    );
  }

  return (
    <ConfiguredPkceCard
      variant={variant}
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
  variant: 'settings' | 'login';
  cfg: OidcConfig;
  session: StoredOidcSession | null;
  actions: AuthActions;
};

function ConfiguredPkceCard({ variant, cfg, session, actions }: ConfiguredPkceCardProps) {
  const [busy, setBusy] = useState(false);
  const [statusText, setStatusText] = useState('');
  const [errorText, setErrorText] = useState('');
  const loginVariant = variant === 'login';

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
    actions.clearSession();
    setErrorText('');
    setStatusText('');
    if (shouldUseFullPageWebAuthRedirect()) {
      if (!request || !discovery) return;
      setStatusText('Opening secure sign-in…');
      try {
        await beginFullPageWebAuthRedirect({
          cfg,
          discovery,
          redirectUri,
          request
        });
      } catch (err) {
        setStatusText('');
        setErrorText(err instanceof Error ? err.message : 'Unable to start sign-in');
      }
      return;
    }
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
    <Card gap={loginVariant ? '$4' : '$2'} alignItems={loginVariant ? 'center' : undefined}>
      <Text
        fontSize={loginVariant ? '$7' : '$5'}
        fontWeight="700"
        textAlign={loginVariant ? 'center' : undefined}
      >
        {loginVariant ? 'Sign in to Pulse' : 'Authentication (PKCE)'}
      </Text>
      <Text
        opacity={0.75}
        textAlign={loginVariant ? 'center' : undefined}
        maxWidth={loginVariant ? 420 : undefined}
      >
        {loginVariant
          ? 'Use your Google account to access your devices securely.'
          : 'Keycloak Authorization Code + PKCE for Expo Web/iOS/Android.'}
      </Text>
      {loginVariant ? null : <Text opacity={0.75}>Issuer: {cfg.issuerUrl}</Text>}
      {loginVariant ? null : <Text opacity={0.75}>Client: {cfg.clientId}</Text>}
      {loginVariant ? null : (
        <Text style={{ color: authenticated ? '#15803d' : '#6b7280' }}>
          {authenticated ? 'Session: signed in' : 'Session: signed out'}
        </Text>
      )}
      {statusText ? (
        <Text
          style={{ color: '#6b7280' }}
          fontSize="$2"
          textAlign={loginVariant ? 'center' : undefined}
        >
          {statusText}
        </Text>
      ) : null}
      {errorText ? (
        <Text
          style={{ color: '#dc2626' }}
          fontSize="$2"
          textAlign={loginVariant ? 'center' : undefined}
        >
          {errorText}
        </Text>
      ) : null}
      {loginVariant ? (
        <YStack gap="$2" width="100%" maxWidth={220}>
          <Button
            size="$5"
            backgroundColor="$accentColor"
            color="white"
            borderColor="$accentColor"
            fontWeight="800"
            paddingVertical="$3"
            paddingHorizontal="$4"
            minHeight={56}
            borderRadius={18}
            style={{
              backgroundColor: '#0f766e',
              borderColor: '#0f766e'
            }}
            pressStyle={{
              backgroundColor: '#115e59'
            }}
            onPress={() => {
              void onSignIn();
            }}
            disabled={busy || !request || !discovery}
          >
            Sign In
          </Button>
          {busy ? (
            <YStack alignItems="center" paddingTop="$1">
              <Spinner size="small" color="#f59e0b" />
            </YStack>
          ) : null}
        </YStack>
      ) : (
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
      )}
    </Card>
  );
}
