import { env } from '@/shared/config/env';
import { splitScopes } from '@/features/auth/scopes';

export type OidcConfig = {
  issuerUrl: string;
  clientId: string;
  audience: string;
  scopes: string[];
};

export function readOidcConfig(): OidcConfig | null {
  const issuerUrl = env.oidcIssuerUrl.trim();
  const clientId = env.oidcClientId.trim();
  if (!issuerUrl || !clientId) {
    return null;
  }
  const scopes = splitScopes(env.oidcScopes);
  return {
    issuerUrl,
    clientId,
    audience: env.oidcAudience.trim(),
    scopes: scopes.length > 0 ? scopes : ['openid', 'profile', 'email', 'offline_access']
  };
}
