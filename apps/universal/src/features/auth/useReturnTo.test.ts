import { describe, expect, it } from 'vitest';
import { buildReturnTo, resolvePostLoginTarget, sanitizeReturnTo } from '@/features/auth/useReturnTo';

describe('sanitizeReturnTo', () => {
  it('returns null when returnTo is missing or unsafe', () => {
    expect(sanitizeReturnTo(undefined)).toBeNull();
    expect(sanitizeReturnTo('')).toBeNull();
    expect(sanitizeReturnTo('https://evil.example')).toBeNull();
    expect(sanitizeReturnTo('//evil.example')).toBeNull();
    expect(sanitizeReturnTo('/login')).toBeNull();
    expect(sanitizeReturnTo('/')).toBeNull();
  });

  it('keeps safe internal paths', () => {
    expect(sanitizeReturnTo('/devices')).toBe('/devices');
    expect(sanitizeReturnTo('/device/019d2b2c-98cd-7f33-b39d-5c8b7fd4c111')).toBe(
      '/device/019d2b2c-98cd-7f33-b39d-5c8b7fd4c111'
    );
  });
});

describe('resolvePostLoginTarget', () => {
  it('prefers an explicit safe returnTo', () => {
    expect(resolvePostLoginTarget('/energy?scope=all', 0)).toBe('/energy?scope=all');
  });

  it('falls back to devices when the user already has devices', () => {
    expect(resolvePostLoginTarget(null, 2)).toBe('/devices');
  });

  it('falls back to onboarding when the user has no devices', () => {
    expect(resolvePostLoginTarget(null, 0)).toBe('/onboarding');
    expect(resolvePostLoginTarget(null, undefined)).toBe('/onboarding');
  });
});

describe('buildReturnTo', () => {
  it('preserves path and non-returnTo query params', () => {
    expect(
      buildReturnTo('/device/123', {
        tab: 'history',
        returnTo: '/ignore',
        focus: ['solar', 'power']
      })
    ).toBe('/device/123?tab=history&focus=solar&focus=power');
  });
});
