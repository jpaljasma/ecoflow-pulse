import { describe, expect, it } from 'vitest';
import {
  canRefreshSession,
  isSessionExpiringWithin,
  shouldBlockOnSessionRecovery,
  shouldRefreshSession
} from '@/features/auth/sessionRefresh';
import type { StoredOidcSession } from '@/features/auth/store';

const now = 1_700_000_000_000;

const baseSession: StoredOidcSession = {
  issuerUrl: 'https://localhost/realms/pulse',
  clientId: 'pulse-universal-app',
  accessToken: 'access-token',
  refreshToken: 'refresh-token',
  idToken: 'id-token',
  tokenType: 'Bearer',
  expiresAtUnixMs: now + 5 * 60 * 1000,
  updatedAtUnixMs: now
};

describe('sessionRefresh', () => {
  it('recognizes refresh-capable sessions', () => {
    expect(canRefreshSession(baseSession)).toBe(true);
    expect(canRefreshSession({ ...baseSession, refreshToken: '' })).toBe(false);
  });

  it('detects when a session is nearing expiry', () => {
    expect(isSessionExpiringWithin(baseSession, now)).toBe(false);
    expect(isSessionExpiringWithin({ ...baseSession, expiresAtUnixMs: now + 30_000 }, now)).toBe(true);
  });

  it('refreshes sessions before they fully expire', () => {
    expect(shouldRefreshSession({ ...baseSession, expiresAtUnixMs: now + 45_000 }, now)).toBe(true);
    expect(shouldRefreshSession(baseSession, now)).toBe(false);
  });

  it('blocks auth redirects while an expired session is still recoverable', () => {
    expect(shouldBlockOnSessionRecovery({ ...baseSession, expiresAtUnixMs: now - 1 }, now)).toBe(true);
    expect(
      shouldBlockOnSessionRecovery({ ...baseSession, refreshToken: '', expiresAtUnixMs: now - 1 }, now)
    ).toBe(false);
  });
});
