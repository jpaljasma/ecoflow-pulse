import { describe, expect, it } from 'vitest';
import { formatAuthMethodLabel, mergeCurrentUserBootstrap, resolveUserDisplayName } from '@/features/profile/model';

function sampleUser(overrides = {}) {
  return {
    id: '019d2b2c-98cd-7f33-b39d-5c8b7fd4c111',
    email: 'user@example.com',
    emailVerified: true,
    displayName: 'Pulse User',
    avatarUrl: 'https://example.com/avatar.png',
    authMethod: 'google',
    givenName: 'Pulse',
    familyName: 'User',
    locale: 'en-US',
    timezone: 'America/New_York',
    weatherLocationEnabled: false,
    weatherLocation: null,
    ...overrides
  };
}

describe('mergeCurrentUserBootstrap', () => {
  it('preserves authorization context when profile data updates', () => {
    const previous = {
      user: sampleUser(),
      authorization: {
        roles: ['viewer'],
        deviceCount: 3
      }
    };

    const next = mergeCurrentUserBootstrap(
      previous,
      sampleUser({
        displayName: 'Updated Pulse User',
        timezone: 'America/Los_Angeles'
      })
    );

    expect(next.authorization).toEqual({
      roles: ['viewer'],
      deviceCount: 3
    });
    expect(next.user.displayName).toBe('Updated Pulse User');
    expect(next.user.timezone).toBe('America/Los_Angeles');
  });

  it('creates a minimal bootstrap when no cached user exists yet', () => {
    const next = mergeCurrentUserBootstrap(undefined, sampleUser());

    expect(next).toEqual({
      user: sampleUser(),
      authorization: {
        roles: [],
        deviceCount: 0
      }
    });
  });
});

describe('formatAuthMethodLabel', () => {
  it('formats common identity provider aliases for profile display', () => {
    expect(formatAuthMethodLabel('google')).toBe('Google');
    expect(formatAuthMethodLabel('facebook')).toBe('Facebook');
    expect(formatAuthMethodLabel('')).toBe('Pulse account');
    expect(formatAuthMethodLabel('enterprise_sso')).toBe('Enterprise Sso');
  });
});

describe('resolveUserDisplayName', () => {
  it('prefers the explicit display name, then full name, then email', () => {
    expect(resolveUserDisplayName(sampleUser())).toBe('Pulse User');
    expect(resolveUserDisplayName(sampleUser({ displayName: '', givenName: 'Jaan', familyName: 'Paljasma' }))).toBe('Jaan Paljasma');
    expect(resolveUserDisplayName(sampleUser({ displayName: '', givenName: '', familyName: '' }))).toBe('user@example.com');
  });
});
