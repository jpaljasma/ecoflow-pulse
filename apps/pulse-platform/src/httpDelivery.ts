import path from 'node:path';

export type StaticHeaderPlan = {
  cacheControl?: string;
};

export type HtmlDeliveryPlan = {
  cacheControl: string;
  linkHeaderValues: string[];
};

type AssetHint = {
  href: string;
  rel: 'preload' | 'modulepreload';
  as?: 'script' | 'style';
};

export function buildHtmlDeliveryPlan(indexHtml: string, preconnectOrigins: string[]): HtmlDeliveryPlan {
  const hints = collectAssetHints(indexHtml);
  const linkHeaderValues = [
    ...normalizePreconnectOrigins(preconnectOrigins).flatMap((origin) => [
      `<${origin}>; rel=preconnect`,
      `<${origin}>; rel=dns-prefetch`
    ]),
    ...hints.map((hint) =>
      hint.rel === 'modulepreload'
        ? `<${hint.href}>; rel=modulepreload`
        : `<${hint.href}>; rel=preload; as=${hint.as ?? 'script'}`
    )
  ];

  return {
    cacheControl: 'no-cache, no-store, must-revalidate',
    linkHeaderValues
  };
}

export function buildStaticHeaderPlan(publicDir: string, filePath: string): StaticHeaderPlan {
  const relativePath = path.relative(publicDir, filePath).replace(/\\/g, '/');
  if (!relativePath || relativePath === 'index.html' || relativePath.endsWith('.html')) {
    return { cacheControl: 'no-cache, no-store, must-revalidate' };
  }
  if (isImmutableAssetPath(relativePath)) {
    return { cacheControl: 'public, max-age=31536000, immutable' };
  }
  return { cacheControl: 'public, max-age=3600' };
}

function collectAssetHints(indexHtml: string): AssetHint[] {
  const hints: AssetHint[] = [];

  const scriptRegex = /<script\b[^>]*\bsrc="([^"]+)"[^>]*>\s*<\/script\s*>/g;
  for (const match of indexHtml.matchAll(scriptRegex)) {
    const href = match[1]?.trim();
    if (!href) continue;
    hints.push({ href, rel: 'preload', as: 'script' });
  }

  const stylesheetRegex = /<link[^>]+rel="stylesheet"[^>]+href="([^"]+)"[^>]*>/g;
  for (const match of indexHtml.matchAll(stylesheetRegex)) {
    const href = match[1]?.trim();
    if (!href) continue;
    hints.push({ href, rel: 'preload', as: 'style' });
  }

  const modulePreloadRegex = /<link[^>]+rel="modulepreload"[^>]+href="([^"]+)"[^>]*>/g;
  for (const match of indexHtml.matchAll(modulePreloadRegex)) {
    const href = match[1]?.trim();
    if (!href) continue;
    hints.push({ href, rel: 'modulepreload' });
  }

  return dedupeHints(hints);
}

function dedupeHints(hints: AssetHint[]): AssetHint[] {
  const seen = new Set<string>();
  const deduped: AssetHint[] = [];
  for (const hint of hints) {
    const key = `${hint.rel}|${hint.as ?? ''}|${hint.href}`;
    if (seen.has(key)) continue;
    seen.add(key);
    deduped.push(hint);
  }
  return deduped;
}

function normalizePreconnectOrigins(origins: string[]): string[] {
  const seen = new Set<string>();
  const normalized: string[] = [];

  for (const origin of origins) {
    const trimmed = origin.trim();
    if (!trimmed) continue;

    let url: URL;
    try {
      url = new URL(trimmed);
    } catch {
      continue;
    }

    const protocol =
      url.protocol === 'wss:' ? 'https:' : url.protocol === 'ws:' ? 'http:' : url.protocol;
    const normalizedOrigin = `${protocol}//${url.host}`;
    if (seen.has(normalizedOrigin)) continue;
    seen.add(normalizedOrigin);
    normalized.push(normalizedOrigin);
  }

  return normalized;
}

function isImmutableAssetPath(relativePath: string): boolean {
  if (relativePath.startsWith('_expo/static/')) return true;
  if (relativePath.startsWith('assets/')) return true;
  return /\.[0-9a-f]{8,}\./i.test(relativePath);
}
