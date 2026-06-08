import { env } from '@/shared/config/env';
import { buildApiRequestUrl } from '@/shared/api/url';

export type AuthSessionRecoveryOutcome =
  | 'recovered_in_memory'
  | 'recovered_refresh'
  | 'reauth_redirect';

export async function reportAuthSessionRecovery(outcome: AuthSessionRecoveryOutcome): Promise<void> {
  const url = buildApiRequestUrl(env.apiUrl, '/api/v1/auth/session-events');
  try {
    await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json'
      },
      body: JSON.stringify({ outcome }),
      keepalive: true
    });
  } catch {
    // Best-effort client metric reporting should never block auth recovery UX.
  }
}
