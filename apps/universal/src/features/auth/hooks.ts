import { readOidcConfig } from '@/features/auth/oidcConfig';
import { deriveAuthSessionState } from '@/features/auth/authSessionState';
import { useAuthStore } from '@/features/auth/store';
import { useConnectionProfileStore } from '@/shared/config/connectionProfileStore';

export function useAuthSession() {
  const hydrated = useAuthStore((state) => state.hydrated);
  const refreshing = useAuthStore((state) => state.refreshing);
  const session = useAuthStore((state) => state.session);
  const profileId = useConnectionProfileStore((state) => state.profileId);
  const oidcConfig = readOidcConfig();

  return deriveAuthSessionState({
    hydrated,
    refreshing,
    session,
    oidcConfig,
    profileId,
    nowUnixMs: Date.now()
  });
}
