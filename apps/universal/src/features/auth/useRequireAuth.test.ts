import { describe, expect, it } from 'vitest';
import { getRequireAuthDecision } from '@/features/auth/guard';

describe('getRequireAuthDecision', () => {
  it('allows access when auth is not configured', () => {
    expect(
      getRequireAuthDecision({
        authConfigured: false,
        authReady: false,
        sessionValid: false,
        pathname: '/devices',
        params: {}
      })
    ).toEqual({
      allowed: true,
      waiting: false,
      redirectTo: null
    });
  });

  it('waits while auth is hydrating', () => {
    expect(
      getRequireAuthDecision({
        authConfigured: true,
        authReady: false,
        sessionValid: false,
        pathname: '/devices',
        params: {}
      })
    ).toEqual({
      allowed: false,
      waiting: true,
      redirectTo: null
    });
  });

  it('redirects unauthenticated users to login with a safe returnTo path', () => {
    expect(
      getRequireAuthDecision({
        authConfigured: true,
        authReady: true,
        sessionValid: false,
        pathname: '/device/019cab9d-bcab-75c0-9c02-db3ae1105d61',
        params: {
          tab: 'packs'
        }
      })
    ).toEqual({
      allowed: false,
      waiting: false,
      redirectTo: {
        pathname: '/login',
        params: {
          returnTo: '/device/019cab9d-bcab-75c0-9c02-db3ae1105d61?tab=packs'
        }
      }
    });
  });

  it('does not redirect when the session is valid', () => {
    expect(
      getRequireAuthDecision({
        authConfigured: true,
        authReady: true,
        sessionValid: true,
        pathname: '/profile',
        params: {}
      })
    ).toEqual({
      allowed: true,
      waiting: false,
      redirectTo: null
    });
  });
});
