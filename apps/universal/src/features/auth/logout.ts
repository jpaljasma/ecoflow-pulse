import type { OidcConfig } from '@/features/auth/oidcConfig';
import type { StoredOidcSession } from '@/features/auth/store';

export async function performLogout(deps: {
  disconnectRealtime: () => void;
  resetTelemetry: () => void;
  clearSession: () => void;
  clearQueries: () => void;
  onComplete?: () => void;
  navigateHome: () => void;
  session?: StoredOidcSession | null;
  oidcConfig?: OidcConfig | null;
}) {
  deps.disconnectRealtime();
  deps.resetTelemetry();
  deps.clearSession();
  deps.clearQueries();
  deps.onComplete?.();
  const logoutUrl = buildOidcLogoutUrl(deps.oidcConfig ?? null, deps.session ?? null);
  if (logoutUrl && typeof window !== 'undefined') {
    window.location.assign(logoutUrl);
    return;
  }
  deps.navigateHome();
}

function buildOidcLogoutUrl(
  oidcConfig: OidcConfig | null,
  session: StoredOidcSession | null
): string | null {
  if (!oidcConfig || !session?.issuerUrl || oidcConfig.issuerUrl !== session.issuerUrl) {
    return null;
  }
  if (typeof window === 'undefined') {
    return null;
  }
  const endpoint = `${oidcConfig.issuerUrl.replace(/\/+$/, '')}/protocol/openid-connect/logout`;
  const params = new URLSearchParams({
    client_id: oidcConfig.clientId,
    post_logout_redirect_uri: `${window.location.origin}/`
  });
  if (session.idToken) {
    params.set('id_token_hint', session.idToken);
  }
  return `${endpoint}?${params.toString()}`;
}
