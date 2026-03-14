import Constants from 'expo-constants';
import * as Linking from 'expo-linking';
import { Platform } from 'react-native';

const extra = Constants.expoConfig?.extra ?? {};
type ExpoConstantsLike = {
  manifest?: {
    debuggerHost?: string;
    hostUri?: string;
  };
  manifest2?: {
    extra?: {
      expoGo?: { debuggerHost?: string };
      expoClient?: { hostUri?: string };
    };
  };
};
const constantsLike = Constants as unknown as ExpoConstantsLike;

function readConfiguredString(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined;
  const trimmed = value.trim();
  return trimmed ? trimmed : undefined;
}

function extractHost(value: unknown): string {
  if (typeof value !== 'string') return '';
  const trimmed = value.trim();
  if (!trimmed) return '';

  const withoutScheme = trimmed.replace(/^[a-z][a-z0-9+.-]*:\/\//i, '');
  const firstSegment = withoutScheme.split('/')[0] ?? '';
  if (!firstSegment) return '';

  const bracketedIpv6 = firstSegment.match(/^\[([^\]]+)\](?::\d+)?$/);
  if (bracketedIpv6?.[1]) {
    return bracketedIpv6[1].toLowerCase();
  }

  return (firstSegment.split(':')[0] ?? '').toLowerCase();
}

function isLoopbackHost(host: string): boolean {
  return host === '127.0.0.1' || host === 'localhost' || host === '::1';
}

export function shouldPreferSecureLocalEdge(host: string, port: string | undefined): boolean {
  return isLoopbackHost(host) && port === '8081';
}

function collectNativeHostHints(): string[] {
  if (Platform.OS === 'web') return [];

  const candidates: string[] = [];

  try {
    const url = Linking.createURL('/');
    const parsed = new URL(url);
    if (parsed.hostname) {
      candidates.push(parsed.hostname.toLowerCase());
    }
  } catch {
    // Fallback candidates below.
  }

  candidates.push(
    extractHost(Constants.expoConfig?.hostUri),
    extractHost(constantsLike.manifest?.debuggerHost),
    extractHost(constantsLike.manifest?.hostUri),
    extractHost(constantsLike.manifest2?.extra?.expoGo?.debuggerHost),
    extractHost(constantsLike.manifest2?.extra?.expoClient?.hostUri)
  );

  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const host of candidates) {
    if (!host || seen.has(host)) continue;
    seen.add(host);
    normalized.push(host);
  }

  return normalized;
}

const nativeHostHints = collectNativeHostHints();
const preferredNativeHost =
  nativeHostHints.find((host) => !isLoopbackHost(host)) ??
  nativeHostHints[0] ??
  '127.0.0.1';

const defaultWebLocation =
  Platform.OS === 'web' && typeof window !== 'undefined' ? window.location : undefined;
const defaultWebAssetBaseUrl = defaultWebLocation?.origin ?? '';
const defaultWebHost = defaultWebLocation?.hostname || '127.0.0.1';
const prefersSecureLocalEdge =
  Platform.OS === 'web' &&
  shouldPreferSecureLocalEdge(defaultWebHost, defaultWebLocation?.port);
const defaultWebHttpScheme =
  defaultWebLocation?.protocol === 'https:' || prefersSecureLocalEdge ? 'https' : 'http';
const defaultWebWsScheme =
  defaultWebLocation?.protocol === 'https:' || prefersSecureLocalEdge ? 'wss' : 'ws';
const defaultHttpHost = Platform.OS === 'web' ? defaultWebHost : preferredNativeHost;
const defaultWsHost = defaultHttpHost;
const defaultApiBase =
  Platform.OS === 'web'
    ? `${defaultWebHttpScheme}://${defaultHttpHost}`
    : `http://${defaultHttpHost}:18081`;
const defaultWsUrl =
  Platform.OS === 'web'
    ? `${defaultWebWsScheme}://${defaultWsHost}/ws`
    : `ws://${defaultWsHost}:8082/ws`;

function normalizeBasePath(pathname: string): string {
  if (!pathname || pathname === '/') return '';
  return pathname.replace(/\/+$/, '');
}

function deriveWsUrlFromApiUrl(apiUrl: string): string | undefined {
  let parsed: URL;
  try {
    parsed = new URL(apiUrl);
  } catch {
    return undefined;
  }

  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return undefined;
  }

  const wsProtocol = parsed.protocol === 'https:' ? 'wss:' : 'ws:';
  const normalizedPath = normalizeBasePath(parsed.pathname);
  const basePath = normalizedPath.endsWith('/api')
    ? normalizedPath.slice(0, normalizedPath.length - '/api'.length)
    : normalizedPath;
  const wsPath = `${basePath}/ws`.replace(/\/{2,}/g, '/');
  const host = parsed.port ? `${parsed.hostname}:${parsed.port}` : parsed.hostname;

  return `${wsProtocol}//${host}${wsPath}`;
}

const apiUrlFromConfig =
  readConfiguredString(process.env.EXPO_PUBLIC_API_URL) ??
  readConfiguredString((extra as { apiUrl?: unknown }).apiUrl);
const wsUrlFromConfig =
  readConfiguredString(process.env.EXPO_PUBLIC_WS_URL) ??
  readConfiguredString((extra as { wsUrl?: unknown }).wsUrl);
const resolvedApiUrl = apiUrlFromConfig ?? defaultApiBase;
const resolvedWsUrl = wsUrlFromConfig ?? deriveWsUrlFromApiUrl(resolvedApiUrl) ?? defaultWsUrl;

export const env = {
  isWeb: Platform.OS === 'web',
  nativeHostHints,
  defaultAssetBaseUrl: defaultWebAssetBaseUrl,
  apiUrl: resolvedApiUrl,
  apiUrlExplicit:
    readConfiguredString(process.env.EXPO_PUBLIC_API_URL) !== undefined ||
    readConfiguredString((extra as { apiUrl?: unknown }).apiUrl) !== undefined,
  wsUrl: resolvedWsUrl,
  wsUrlExplicit:
    readConfiguredString(process.env.EXPO_PUBLIC_WS_URL) !== undefined ||
    readConfiguredString((extra as { wsUrl?: unknown }).wsUrl) !== undefined,
  assetBaseUrl:
    readConfiguredString(process.env.EXPO_PUBLIC_ASSET_BASE_URL) ??
    readConfiguredString((extra as { assetBaseUrl?: unknown }).assetBaseUrl) ??
    defaultWebAssetBaseUrl,
  closePageTransition:
    readConfiguredString(process.env.EXPO_PUBLIC_CLOSE_PAGE_TRANSITION) ??
    readConfiguredString((extra as { closePageTransition?: unknown }).closePageTransition) ??
    'none',
  closePageTransitionMs:
    Number(process.env.EXPO_PUBLIC_CLOSE_PAGE_TRANSITION_MS) ||
    (typeof extra.closePageTransitionMs === 'number' ? extra.closePageTransitionMs : 220),
  closeButtonAnimation:
    readConfiguredString(process.env.EXPO_PUBLIC_CLOSE_BUTTON_ANIMATION) ??
    readConfiguredString((extra as { closeButtonAnimation?: unknown }).closeButtonAnimation) ??
    'subtle',
  oidcIssuerUrl:
    readConfiguredString(process.env.EXPO_PUBLIC_OIDC_ISSUER_URL) ??
    readConfiguredString((extra as { oidcIssuerUrl?: unknown }).oidcIssuerUrl) ??
    '',
  oidcClientId:
    readConfiguredString(process.env.EXPO_PUBLIC_OIDC_CLIENT_ID) ??
    readConfiguredString((extra as { oidcClientId?: unknown }).oidcClientId) ??
    '',
  oidcAudience:
    readConfiguredString(process.env.EXPO_PUBLIC_OIDC_AUDIENCE) ??
    readConfiguredString((extra as { oidcAudience?: unknown }).oidcAudience) ??
    '',
  oidcScopes:
    readConfiguredString(process.env.EXPO_PUBLIC_OIDC_SCOPES) ??
    readConfiguredString((extra as { oidcScopes?: unknown }).oidcScopes) ??
    'openid profile email offline_access'
};
