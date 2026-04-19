import { describe, expect, it } from 'vitest';

import { deriveAuthSessionState } from '@/features/auth/authSessionState';

describe('deriveAuthSessionState', () => {
  it('partitions the auth cache key by connection profile', () => {
    const nowUnixMs = Date.now();
    const session = {
      issuerUrl: 'https://localhost/realms/pulse',
      clientId: 'pulse-local',
      accessToken: 'token',
      refreshToken: 'refresh',
      idToken: 'id',
      tokenType: 'Bearer',
      expiresAtUnixMs: nowUnixMs + 60_000,
      updatedAtUnixMs: 1234
    };
    const oidcConfig = {
      issuerUrl: 'https://localhost/realms/pulse',
      clientId: 'pulse-local',
      audience: '',
      scopes: ['openid', 'profile', 'email']
    };

    const local = deriveAuthSessionState({
      hydrated: true,
      refreshing: false,
      session,
      oidcConfig,
      profileId: 'local',
      nowUnixMs
    });
    const cloud = deriveAuthSessionState({
      hydrated: true,
      refreshing: false,
      session,
      oidcConfig,
      profileId: 'cloud',
      nowUnixMs
    });

    expect(local.sessionValid).toBe(true);
    expect(local.authKey).toBe('profile:local:session:1234');
    expect(cloud.authKey).toBe('profile:cloud:session:1234');
  });

  it('marks sessions invalid when the active profile issuer changes', () => {
    const nowUnixMs = Date.now();
    const result = deriveAuthSessionState({
      hydrated: true,
      refreshing: false,
      session: {
        issuerUrl: 'https://localhost/realms/pulse',
        clientId: 'pulse-local',
        accessToken: 'token',
        refreshToken: 'refresh',
        idToken: 'id',
        tokenType: 'Bearer',
        expiresAtUnixMs: nowUnixMs + 60_000,
        updatedAtUnixMs: 1234
      },
      oidcConfig: {
        issuerUrl: 'https://pulse.example.com/realms/pulse',
        clientId: 'pulse-cloud',
        audience: '',
        scopes: ['openid', 'profile', 'email']
      },
      profileId: 'cloud',
      nowUnixMs
    });

    expect(result.sessionValid).toBe(false);
    expect(result.token).toBeUndefined();
    expect(result.authKey).toBe('profile:cloud:auth-required');
  });
});
