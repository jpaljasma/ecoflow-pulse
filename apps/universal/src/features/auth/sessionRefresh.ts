import type { StoredOidcSession } from '@/features/auth/store';

export const SESSION_REFRESH_LEEWAY_MS = 60_000;

export function isSessionExpiringWithin(
  session: StoredOidcSession | null,
  nowUnixMs: number,
  withinMs: number = SESSION_REFRESH_LEEWAY_MS
): boolean {
  if (!session || session.expiresAtUnixMs <= 0) {
    return false;
  }
  return session.expiresAtUnixMs - nowUnixMs <= withinMs;
}

export function canRefreshSession(session: StoredOidcSession | null): boolean {
  return Boolean(session?.refreshToken);
}

export function shouldRefreshSession(session: StoredOidcSession | null, nowUnixMs: number): boolean {
  return canRefreshSession(session) && isSessionExpiringWithin(session, nowUnixMs);
}

export function shouldBlockOnSessionRecovery(session: StoredOidcSession | null, nowUnixMs: number): boolean {
  if (!session || !canRefreshSession(session) || session.expiresAtUnixMs <= 0) {
    return false;
  }
  return nowUnixMs >= session.expiresAtUnixMs;
}
