import { describe, expect, it } from 'vitest';

import { buildApiRequestUrl } from '@/shared/api/url';

describe('buildApiRequestUrl', () => {
  it('joins origin-only API bases with API routes', () => {
    expect(buildApiRequestUrl('https://pulse.home.arpa', '/api/v1/me')).toBe(
      'https://pulse.home.arpa/api/v1/me'
    );
  });

  it('does not duplicate /api when the base already points at the edge API path', () => {
    expect(buildApiRequestUrl('https://pulse.home.arpa/api', '/api/v1/me')).toBe(
      'https://pulse.home.arpa/api/v1/me'
    );
  });

  it('preserves prefixes before an explicit /api base path', () => {
    expect(buildApiRequestUrl('https://example.test/pulse/api/', '/api/v1/me')).toBe(
      'https://example.test/pulse/api/v1/me'
    );
  });

  it('leaves absolute request URLs untouched', () => {
    expect(buildApiRequestUrl('https://pulse.home.arpa/api', 'https://other.test/api/v1/me')).toBe(
      'https://other.test/api/v1/me'
    );
  });
});
