import { describe, expect, it } from 'vitest';
import { isSessionExpired, type StoredOidcSession } from '@/features/auth/store';

const baseSession: StoredOidcSession = {
  issuerUrl: 'https://auth.example.com/realms/pulse',
  clientId: 'pulse-web',
  accessToken: 'at',
  refreshToken: 'rt',
  idToken: 'id',
  tokenType: 'Bearer',
  expiresAtUnixMs: 0,
  updatedAtUnixMs: 1700000000000
};

describe('isSessionExpired', () => {
  it('treats missing session as expired', () => {
    expect(isSessionExpired(null, Date.now())).toBe(true);
  });

  it('treats non-expiring sessions as active', () => {
    expect(isSessionExpired(baseSession, Date.now())).toBe(false);
  });

  it('treats elapsed expiring sessions as expired', () => {
    const now = 1700000000000;
    expect(
      isSessionExpired(
        {
          ...baseSession,
          expiresAtUnixMs: now - 1
        },
        now
      )
    ).toBe(true);
  });
});
