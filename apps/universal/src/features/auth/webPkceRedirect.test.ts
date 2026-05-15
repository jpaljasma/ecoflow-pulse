import { beforeEach, describe, expect, it, vi } from 'vitest';

const { exchangeCodeAsync, fetchDiscoveryAsync } = vi.hoisted(() => ({
  exchangeCodeAsync: vi.fn(),
  fetchDiscoveryAsync: vi.fn()
}));

vi.mock('expo-auth-session', () => ({
  exchangeCodeAsync,
  fetchDiscoveryAsync
}));

import {
  beginFullPageWebAuthRedirect,
  completeFullPageWebAuthRedirect,
  shouldUseFullPageWebAuthRedirect
} from '@/features/auth/webPkceRedirect';

class MemoryStorage {
  private readonly values = new Map<string, string>();

  getItem(key: string): string | null {
    return this.values.get(key) ?? null;
  }

  setItem(key: string, value: string): void {
    this.values.set(key, value);
  }

  removeItem(key: string): void {
    this.values.delete(key);
  }
}

describe('web PKCE full-page redirect helpers', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: {
        location: {
          href: 'https://pulse.example.com/login',
          assign: vi.fn()
        },
        navigator: {
          maxTouchPoints: 0,
          userAgent:
            'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari/605.1.15'
        },
        sessionStorage: new MemoryStorage(),
        localStorage: new MemoryStorage()
      }
    });
  });

  it('uses full-page redirect for desktop web browsers so popup blockers cannot trap sign-in', () => {
    expect(shouldUseFullPageWebAuthRedirect()).toBe(true);
  });

  it('does not force full-page redirect without a browser navigation surface', () => {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: undefined
    });

    expect(shouldUseFullPageWebAuthRedirect()).toBe(false);
  });

  it('uses full-page redirect for iOS Safari and Chrome user agents', () => {
    expect(
      shouldUseFullPageWebAuthRedirect(
        'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Version/18.0 Mobile/15E148 Safari/604.1'
      )
    ).toBe(true);
    expect(
      shouldUseFullPageWebAuthRedirect(
        'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 CriOS/124.0.0.0 Mobile/15E148 Safari/604.1'
      )
    ).toBe(true);
  });

  it('stores PKCE state before navigating the current iOS browser tab', async () => {
    const request = {
      codeVerifier: 'verifier-123',
      makeAuthUrlAsync: vi.fn(async () => 'https://issuer.example/auth?state=state-123')
    };

    await beginFullPageWebAuthRedirect({
      cfg: {
        issuerUrl: 'https://issuer.example/realms/pulse',
        clientId: 'pulse-web',
        scopes: ['openid', 'profile'],
        audience: ''
      },
      discovery: { authorizationEndpoint: 'https://issuer.example/auth' },
      redirectUri: 'https://pulse.example.com/auth/callback',
      request
    });

    expect(request.makeAuthUrlAsync).toHaveBeenCalledTimes(1);
    expect(window.sessionStorage.getItem('pulse.webAuth.pkce.v1')).toContain('verifier-123');
    expect(window.location.assign).toHaveBeenCalledWith('https://issuer.example/auth?state=state-123');
  });

  it('exchanges a returned code using the stored verifier and clears pending state', async () => {
    window.sessionStorage.setItem(
      'pulse.webAuth.pkce.v1',
      JSON.stringify({
        state: 'state-123',
        codeVerifier: 'verifier-123',
        redirectUri: 'https://pulse.example.com/auth/callback',
        issuerUrl: 'https://issuer.example/realms/pulse',
        clientId: 'pulse-web',
        createdAtUnixMs: Date.now()
      })
    );
    fetchDiscoveryAsync.mockResolvedValueOnce({
      tokenEndpoint: 'https://issuer.example/token'
    });
    exchangeCodeAsync.mockResolvedValueOnce({
      accessToken: 'access-token',
      refreshToken: 'refresh-token',
      idToken: 'id-token',
      tokenType: 'Bearer',
      issuedAt: 1_700_000_000,
      expiresIn: 3600
    });

    const result = await completeFullPageWebAuthRedirect(
      'https://pulse.example.com/auth/callback?code=code-123&state=state-123'
    );

    expect(result.type).toBe('success');
    expect(exchangeCodeAsync).toHaveBeenCalledWith(
      {
        clientId: 'pulse-web',
        code: 'code-123',
        redirectUri: 'https://pulse.example.com/auth/callback',
        extraParams: {
          code_verifier: 'verifier-123'
        }
      },
      { tokenEndpoint: 'https://issuer.example/token' }
    );
    expect(window.sessionStorage.getItem('pulse.webAuth.pkce.v1')).toBeNull();
  });
});
