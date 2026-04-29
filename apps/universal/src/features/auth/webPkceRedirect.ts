import * as AuthSession from 'expo-auth-session';

import type { OidcConfig } from '@/features/auth/oidcConfig';

export const WEB_PKCE_REDIRECT_STORAGE_KEY = 'pulse.webAuth.pkce.v1';

const MAX_PENDING_AGE_MS = 10 * 60 * 1000;

type MinimalAuthRequest = {
  codeVerifier?: string;
  makeAuthUrlAsync: (discovery: AuthSession.AuthDiscoveryDocument) => Promise<string>;
};

type PendingWebPkceRedirect = {
  state: string;
  codeVerifier: string;
  redirectUri: string;
  issuerUrl: string;
  clientId: string;
  createdAtUnixMs: number;
};

type WebStorageLike = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

export type FullPageWebAuthRedirectResult =
  | { type: 'idle' }
  | {
      type: 'success';
      issuerUrl: string;
      clientId: string;
      token: AuthSession.TokenResponse;
    }
  | { type: 'error'; message: string };

export function shouldUseFullPageWebAuthRedirect(userAgent?: string): boolean {
  const ua =
    userAgent ??
    (typeof window !== 'undefined' && typeof window.navigator?.userAgent === 'string'
      ? window.navigator.userAgent
      : '');
  if (/\b(iPad|iPhone|iPod)\b/i.test(ua)) {
    return true;
  }

  const maxTouchPoints =
    typeof window !== 'undefined' && typeof window.navigator?.maxTouchPoints === 'number'
      ? window.navigator.maxTouchPoints
      : 0;
  return /\bMacintosh\b/i.test(ua) && maxTouchPoints > 1;
}

export async function beginFullPageWebAuthRedirect({
  cfg,
  discovery,
  redirectUri,
  request
}: {
  cfg: OidcConfig;
  discovery: AuthSession.AuthDiscoveryDocument;
  redirectUri: string;
  request: MinimalAuthRequest;
}): Promise<void> {
  if (typeof window === 'undefined') {
    throw new Error('Full-page web auth redirect requires a browser window');
  }

  const authUrl = await request.makeAuthUrlAsync(discovery);
  const state = readAuthUrlParam(authUrl, 'state');
  const codeVerifier = request.codeVerifier;
  if (!state || !codeVerifier) {
    throw new Error('Missing PKCE state or verifier');
  }

  writePendingRedirect({
    state,
    codeVerifier,
    redirectUri,
    issuerUrl: cfg.issuerUrl,
    clientId: cfg.clientId,
    createdAtUnixMs: Date.now()
  });
  window.location.assign(authUrl);
}

export async function completeFullPageWebAuthRedirect(
  currentUrl = typeof window !== 'undefined' ? window.location.href : ''
): Promise<FullPageWebAuthRedirectResult> {
  const pending = readPendingRedirect();
  if (!pending) {
    return { type: 'idle' };
  }

  const url = parseUrl(currentUrl);
  if (!url) {
    return { type: 'idle' };
  }

  const code = url.searchParams.get('code') ?? '';
  const state = url.searchParams.get('state') ?? '';
  const error = url.searchParams.get('error') ?? '';
  if (!code && !error) {
    return { type: 'idle' };
  }

  clearPendingRedirect();
  if (Date.now() - pending.createdAtUnixMs > MAX_PENDING_AGE_MS) {
    return { type: 'error', message: 'Sign-in session expired. Please try again.' };
  }
  if (error) {
    return {
      type: 'error',
      message: url.searchParams.get('error_description') ?? error
    };
  }
  if (!code) {
    return { type: 'error', message: 'Missing auth code' };
  }
  if (!state || state !== pending.state) {
    return { type: 'error', message: 'Sign-in state did not match. Please try again.' };
  }

  try {
    const discovery = await AuthSession.fetchDiscoveryAsync(pending.issuerUrl);
    const token = await AuthSession.exchangeCodeAsync(
      {
        clientId: pending.clientId,
        code,
        redirectUri: pending.redirectUri,
        extraParams: {
          code_verifier: pending.codeVerifier
        }
      },
      discovery
    );
    return {
      type: 'success',
      issuerUrl: pending.issuerUrl,
      clientId: pending.clientId,
      token
    };
  } catch (err) {
    return {
      type: 'error',
      message: err instanceof Error ? err.message : 'Token exchange failed'
    };
  }
}

function readAuthUrlParam(authUrl: string, param: string): string {
  const url = parseUrl(authUrl);
  return url?.searchParams.get(param) ?? '';
}

function parseUrl(input: string): URL | null {
  try {
    return new URL(input);
  } catch {
    return null;
  }
}

function writePendingRedirect(pending: PendingWebPkceRedirect): void {
  const storage = getWebStorage();
  if (!storage) {
    throw new Error('Browser storage is unavailable for sign-in');
  }
  storage.setItem(WEB_PKCE_REDIRECT_STORAGE_KEY, JSON.stringify(pending));
}

function readPendingRedirect(): PendingWebPkceRedirect | null {
  const raw = getStorageCandidates()
    .map((storage) => storage.getItem(WEB_PKCE_REDIRECT_STORAGE_KEY))
    .find((value): value is string => typeof value === 'string' && value.length > 0);
  if (!raw) {
    return null;
  }
  try {
    const parsed = JSON.parse(raw) as Partial<PendingWebPkceRedirect>;
    if (
      typeof parsed.state !== 'string' ||
      typeof parsed.codeVerifier !== 'string' ||
      typeof parsed.redirectUri !== 'string' ||
      typeof parsed.issuerUrl !== 'string' ||
      typeof parsed.clientId !== 'string' ||
      typeof parsed.createdAtUnixMs !== 'number'
    ) {
      return null;
    }
    return parsed as PendingWebPkceRedirect;
  } catch {
    return null;
  }
}

function clearPendingRedirect(): void {
  for (const storage of getStorageCandidates()) {
    storage.removeItem(WEB_PKCE_REDIRECT_STORAGE_KEY);
  }
}

function getWebStorage(): WebStorageLike | null {
  return getStorageCandidates()[0] ?? null;
}

function getStorageCandidates(): WebStorageLike[] {
  if (typeof window === 'undefined') {
    return [];
  }
  return [getStorageCandidate('sessionStorage'), getStorageCandidate('localStorage')].filter(
    (storage): storage is WebStorageLike => storage !== null
  );
}

function getStorageCandidate(name: 'sessionStorage' | 'localStorage'): WebStorageLike | null {
  try {
    return window[name] ?? null;
  } catch {
    return null;
  }
}
