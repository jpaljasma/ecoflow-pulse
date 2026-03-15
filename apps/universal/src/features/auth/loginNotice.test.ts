import { describe, expect, it } from 'vitest';
import { buildLoginNotice, parseReauthReason } from '@/features/auth/loginNotice';

describe('loginNotice', () => {
  it('parses only supported reauthentication reasons', () => {
    expect(parseReauthReason('session_expired')).toBe('session_expired');
    expect(parseReauthReason(['session_expired'])).toBe('session_expired');
    expect(parseReauthReason('other')).toBeNull();
    expect(parseReauthReason(undefined)).toBeNull();
  });

  it('builds the inactivity notice copy for expired sessions', () => {
    expect(buildLoginNotice('session_expired')).toEqual({
      iconText: '!',
      headline: 'Please sign in again',
      detail: 'Your session needs to be refreshed after a long period of inactivity.',
      statusLabel: 'Session expired'
    });
    expect(buildLoginNotice(null)).toBeNull();
  });
});
