import { beforeEach, describe, expect, it, vi } from 'vitest';

const { fetchDiscoveryAsync, refreshAsync } = vi.hoisted(() => ({
  fetchDiscoveryAsync: vi.fn(),
  refreshAsync: vi.fn()
}));
const { reportAuthSessionRecovery } = vi.hoisted(() => ({
  reportAuthSessionRecovery: vi.fn(async () => undefined)
}));

vi.mock('expo-auth-session', () => ({
  fetchDiscoveryAsync,
  refreshAsync
}));

vi.mock('@react-native-async-storage/async-storage', () => ({
  default: {
    getItem: vi.fn(async () => null),
    setItem: vi.fn(async () => undefined),
    removeItem: vi.fn(async () => undefined)
  }
}));

vi.mock('@/shared/config/env', () => ({
  env: {
    oidcIssuerUrl: 'https://auth.example.test/realms/pulse',
    oidcClientId: 'pulse-web',
    oidcAudience: '',
    oidcScopes: 'openid profile email offline_access'
  }
}));

vi.mock('@/features/auth/sessionRecoveryMetrics', () => ({
  reportAuthSessionRecovery
}));

import { recoverSessionForUnauthorizedRequest } from '@/features/auth/sessionRecoveryCoordinator';
import { useAuthStore } from '@/features/auth/store';

describe('sessionRecoveryCoordinator', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAuthStore.setState({
      hydrated: true,
      refreshing: false,
      session: {
        issuerUrl: 'https://auth.example.test/realms/pulse',
        clientId: 'pulse-web',
        accessToken: 'latest-token',
        refreshToken: 'refresh-token',
        idToken: 'id-token',
        tokenType: 'Bearer',
        expiresAtUnixMs: Date.now() + 60_000,
        updatedAtUnixMs: Date.now()
      },
      reauthRequest: {
        nonce: 0,
        reason: null
      }
    });
  });

  it('reuses a fresher token already present in the auth store', async () => {
    await expect(recoverSessionForUnauthorizedRequest('stale-token')).resolves.toBe('latest-token');
    expect(fetchDiscoveryAsync).not.toHaveBeenCalled();
    expect(refreshAsync).not.toHaveBeenCalled();
    expect(reportAuthSessionRecovery).toHaveBeenCalledWith('recovered_in_memory');
  });

  it('refreshes the session when the stored token is expired', async () => {
    useAuthStore.setState((state) => ({
      ...state,
      session: state.session
        ? {
            ...state.session,
            accessToken: 'expired-token',
            expiresAtUnixMs: Date.now() - 1_000
          }
        : null
    }));
    fetchDiscoveryAsync.mockResolvedValueOnce({ tokenEndpoint: 'https://auth.example.test/token' });
    refreshAsync.mockResolvedValueOnce({
      accessToken: 'refreshed-token',
      refreshToken: 'new-refresh-token',
      idToken: 'new-id-token',
      tokenType: 'Bearer',
      issuedAt: Math.floor(Date.now() / 1000),
      expiresIn: 3600
    });

    await expect(recoverSessionForUnauthorizedRequest('expired-token')).resolves.toBe('refreshed-token');
    expect(fetchDiscoveryAsync).toHaveBeenCalledTimes(1);
    expect(refreshAsync).toHaveBeenCalledTimes(1);
    expect(useAuthStore.getState().session?.accessToken).toBe('refreshed-token');
    expect(reportAuthSessionRecovery).toHaveBeenCalledWith('recovered_refresh');
  });
});
