import { shouldBlockOnSessionRecovery } from '@/features/auth/sessionRefresh';
import { isSessionExpired, type StoredOidcSession } from '@/features/auth/store';
import type { OidcConfig } from '@/features/auth/oidcConfig';
import type { ConnectionProfileId } from '@/shared/config/env';

export function deriveAuthSessionState(input: {
  hydrated: boolean;
  refreshing: boolean;
  session: StoredOidcSession | null;
  oidcConfig: OidcConfig | null;
  profileId: ConnectionProfileId;
  nowUnixMs: number;
}) {
  const { hydrated, refreshing, session, oidcConfig, profileId, nowUnixMs } = input;
  const authConfigured = Boolean(oidcConfig);
  const sessionMatchesConfig =
    !oidcConfig ||
    (session?.issuerUrl === oidcConfig.issuerUrl && session?.clientId === oidcConfig.clientId);
  const sessionRecovering =
    hydrated && sessionMatchesConfig && shouldBlockOnSessionRecovery(session, nowUnixMs);
  const sessionValid =
    sessionMatchesConfig && Boolean(session?.accessToken) && !isSessionExpired(session, nowUnixMs);
  const token = sessionValid ? session?.accessToken : undefined;

  return {
    authConfigured,
    authReady: !authConfigured || (hydrated && !sessionRecovering),
    refreshing,
    sessionValid,
    token,
    authKey: sessionValid
      ? `profile:${profileId}:session:${session?.updatedAtUnixMs ?? 0}`
      : authConfigured && sessionRecovering
        ? `profile:${profileId}:auth-refreshing`
        : authConfigured
          ? hydrated
            ? `profile:${profileId}:auth-required`
            : `profile:${profileId}:auth-pending`
          : `profile:${profileId}:anonymous`
  };
}
