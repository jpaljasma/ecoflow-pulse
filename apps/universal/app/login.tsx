import { useEffect, useRef } from 'react';
import { Redirect, useLocalSearchParams, useRouter } from 'expo-router';
import { ScrollView } from 'react-native';
import { Button, Text, YStack } from 'tamagui';
import { LoginCard } from '@/features/auth/KeycloakPkceCard';
import { useAuthSession } from '@/features/auth/hooks';
import { buildLoginNotice, parseReauthReason } from '@/features/auth/loginNotice';
import { resolvePostLoginTarget, sanitizeReturnTo } from '@/features/auth/useReturnTo';
import { resolveUserDisplayName } from '@/features/profile/model';
import { useCurrentUser, useUpdateCurrentUser } from '@/features/profile/hooks';
import { detectCurrentTimeZone } from '@/features/profile/timezone';
import { ApiError } from '@/shared/api/restClient';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';
import { Card } from '@/shared/ui/Card';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { StatusBanner } from '@/shared/ui/StatusBanner';

export default function LoginScreen() {
  const router = useRouter();
  const params = useLocalSearchParams<{ returnTo?: string | string[]; reason?: string | string[] }>();
  const returnTo = sanitizeReturnTo(params.returnTo);
  const loginNotice = buildLoginNotice(parseReauthReason(params.reason));
  const timezone = detectCurrentTimeZone();
  const redirectedRef = useRef(false);
  const timezoneSyncRef = useRef(false);
  const bootstrapRetryRef = useRef(false);
  const { authConfigured, authReady, token, authKey, sessionValid } = useAuthSession();
  const tokenReady = authReady && sessionValid && Boolean(token);
  const currentUserQuery = useCurrentUser({
    token,
    authKey,
    enabled: tokenReady
  });
  const updateCurrentUser = useUpdateCurrentUser({
    token,
    authKey
  });
  const postLoginTarget = resolvePostLoginTarget(
    returnTo,
    currentUserQuery.data?.authorization.deviceCount
  );

  useEffect(() => {
    if (!tokenReady || !currentUserQuery.isError || bootstrapRetryRef.current) {
      return;
    }
    if (!(currentUserQuery.error instanceof ApiError) || currentUserQuery.error.status !== 401) {
      return;
    }
    if (typeof window === 'undefined') {
      return;
    }
    bootstrapRetryRef.current = true;
    const retry = window.setTimeout(() => {
      void currentUserQuery.refetch();
    }, 250);
    return () => {
      window.clearTimeout(retry);
    };
  }, [currentUserQuery, tokenReady]);

  useEffect(() => {
    const bootstrap = currentUserQuery.data;
    const user = bootstrap?.user;
    if (!sessionValid || !bootstrap || !user || redirectedRef.current || updateCurrentUser.isPending) {
      return;
    }
    if (!timezoneSyncRef.current && !user.timezone && timezone) {
      timezoneSyncRef.current = true;
      updateCurrentUser.mutate({
        displayName: resolveUserDisplayName(user) || 'Pulse User',
        timezone,
        weatherLocationEnabled: user.weatherLocationEnabled,
        weatherLocation: user.weatherLocation
      });
      return;
    }
    redirectedRef.current = true;
    router.replace(postLoginTarget);
  }, [currentUserQuery.data, postLoginTarget, router, sessionValid, timezone, updateCurrentUser]);

  if (tokenReady && (currentUserQuery.isLoading || updateCurrentUser.isPending)) {
    return <BrandedLoadingState minHeight={260} message="Finishing sign-in…" />;
  }

  if (sessionValid && redirectedRef.current) {
    return <Redirect href={postLoginTarget} />;
  }

  return (
    <YStack flex={1} backgroundColor="$background" padding="$4">
      <ScrollView contentContainerStyle={{ flexGrow: 1, justifyContent: 'center' }}>
        <YStack gap="$4" width="100%" maxWidth={520} alignSelf="center">
          <YStack alignItems="center">
            <BrandLogo />
          </YStack>
          {loginNotice ? (
            <StatusBanner
              iconText={loginNotice.iconText}
              headline={loginNotice.headline}
              detail={loginNotice.detail}
              statusLabel={loginNotice.statusLabel}
            />
          ) : null}
          <LoginCard />
          {!authConfigured ? (
            <Card gap="$2">
              <Text fontSize="$5" fontWeight="700">OIDC configuration missing</Text>
              <Text color="$colorMuted">
                Set the public OIDC environment variables before validating the login flow.
              </Text>
            </Card>
          ) : null}
          {currentUserQuery.isError &&
          (!(currentUserQuery.error instanceof ApiError) || currentUserQuery.error.status !== 401) ? (
            <Card gap="$2">
              <Text fontSize="$5" fontWeight="700">Profile bootstrap is temporarily unavailable</Text>
              <Text color="$colorMuted">
                Your session is still kept locally. Retry once the platform finishes recovering.
              </Text>
              <Button
                size="$4"
                alignSelf="flex-start"
                onPress={() => {
                  void currentUserQuery.refetch();
                }}
              >
                Retry bootstrap
              </Button>
            </Card>
          ) : null}
          {updateCurrentUser.isError ? (
            <Card gap="$2">
              <Text fontSize="$5" fontWeight="700">Failed to finalize timezone setup</Text>
              <Text color="$colorMuted">{String(updateCurrentUser.error)}</Text>
            </Card>
          ) : null}
        </YStack>
      </ScrollView>
    </YStack>
  );
}
