import { describe, expect, it } from 'vitest';
import { buildHtmlDeliveryPlan, buildStaticHeaderPlan } from '../src/httpDelivery.js';

describe('pulse-platform http delivery helpers', () => {
  it('extracts preload hints and cross-origin preconnect headers from html', () => {
    const plan = buildHtmlDeliveryPlan(
      '<!doctype html><html><head><link rel="stylesheet" href="/styles.css"><script src="/_expo/static/js/web/index-123.js" defer></script></head></html>',
      ['https://api.example.com', 'wss://ws.example.com']
    );

    expect(plan.cacheControl).toBe('no-cache, no-store, must-revalidate');
    expect(plan.linkHeaderValues).toEqual(
      expect.arrayContaining([
        '<https://api.example.com>; rel=preconnect',
        '<https://api.example.com>; rel=dns-prefetch',
        '<https://ws.example.com>; rel=preconnect',
        '<https://ws.example.com>; rel=dns-prefetch',
        '</styles.css>; rel=preload; as=style',
        '</_expo/static/js/web/index-123.js>; rel=preload; as=script'
      ])
    );
  });

  it('marks hashed/static assets as immutable', () => {
    expect(
      buildStaticHeaderPlan('/app/dist', '/app/dist/_expo/static/js/web/index-1234567890abcdef.js')
    ).toEqual({ cacheControl: 'public, max-age=31536000, immutable' });
    expect(buildStaticHeaderPlan('/app/dist', '/app/dist/assets/logo.png')).toEqual({
      cacheControl: 'public, max-age=31536000, immutable'
    });
  });

  it('keeps html no-store and non-hashed files short-lived', () => {
    expect(buildStaticHeaderPlan('/app/dist', '/app/dist/index.html')).toEqual({
      cacheControl: 'no-cache, no-store, must-revalidate'
    });
    expect(buildStaticHeaderPlan('/app/dist', '/app/dist/manifest.json')).toEqual({
      cacheControl: 'public, max-age=3600'
    });
  });
});
