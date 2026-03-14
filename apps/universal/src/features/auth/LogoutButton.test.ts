import { describe, expect, it, vi } from 'vitest';
import { performLogout } from '@/features/auth/logout';

describe('performLogout', () => {
  it('tears down realtime/auth state before navigating home', async () => {
    const calls: string[] = [];

    await performLogout({
      disconnectRealtime: () => {
        calls.push('disconnect');
      },
      resetTelemetry: () => {
        calls.push('reset');
      },
      clearSession: () => {
        calls.push('clear-session');
      },
      clearQueries: () => {
        calls.push('clear-queries');
      },
      onComplete: () => {
        calls.push('complete');
      },
      navigateHome: () => {
        calls.push('navigate');
      }
    });

    expect(calls).toEqual([
      'disconnect',
      'reset',
      'clear-session',
      'clear-queries',
      'complete',
      'navigate'
    ]);
  });

  it('keeps working when onComplete is omitted', async () => {
    const navigateHome = vi.fn();

    await expect(
      performLogout({
        disconnectRealtime: vi.fn(),
        resetTelemetry: vi.fn(),
        clearSession: vi.fn(),
        clearQueries: vi.fn(),
        navigateHome
      })
    ).resolves.toBeUndefined();

    expect(navigateHome).toHaveBeenCalledOnce();
  });

  it('redirects through the OIDC logout endpoint on web when config and session are available', async () => {
    const assign = vi.fn();
    vi.stubGlobal('window', {
      location: {
        origin: 'https://localhost',
        assign
      }
    });

    await performLogout({
      disconnectRealtime: vi.fn(),
      resetTelemetry: vi.fn(),
      clearSession: vi.fn(),
      clearQueries: vi.fn(),
      navigateHome: vi.fn(),
      oidcConfig: {
        issuerUrl: 'https://localhost/realms/pulse',
        clientId: 'pulse-universal-app',
        audience: '',
        scopes: ['openid', 'profile', 'email']
      },
      session: {
        issuerUrl: 'https://localhost/realms/pulse',
        clientId: 'pulse-universal-app',
        accessToken: 'access-token',
        refreshToken: 'refresh-token',
        idToken: 'id-token',
        tokenType: 'Bearer',
        expiresAtUnixMs: 0,
        updatedAtUnixMs: 0
      }
    });

    expect(assign).toHaveBeenCalledWith(
      'https://localhost/realms/pulse/protocol/openid-connect/logout?client_id=pulse-universal-app&post_logout_redirect_uri=https%3A%2F%2Flocalhost%2F&id_token_hint=id-token'
    );
    vi.unstubAllGlobals();
  });
});
