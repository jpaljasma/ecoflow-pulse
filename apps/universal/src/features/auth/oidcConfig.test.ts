import { describe, expect, it } from 'vitest';
import { splitScopes } from '@/features/auth/scopes';

describe('splitScopes', () => {
  it('splits scopes by spaces and commas', () => {
    expect(splitScopes('openid profile,email offline_access')).toEqual([
      'openid',
      'profile',
      'email',
      'offline_access'
    ]);
  });

  it('drops empty values', () => {
    expect(splitScopes('  openid   ,  , profile ')).toEqual(['openid', 'profile']);
  });
});
