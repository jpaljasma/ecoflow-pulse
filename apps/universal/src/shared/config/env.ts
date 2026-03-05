import Constants from 'expo-constants';
import * as Linking from 'expo-linking';
import { Platform } from 'react-native';

const extra = Constants.expoConfig?.extra ?? {};
const defaultWebLocation =
  Platform.OS === 'web' && typeof window !== 'undefined' ? window.location : undefined;
const defaultWebAssetBaseUrl = defaultWebLocation?.origin ?? '';
const defaultWebHost = defaultWebLocation?.hostname || '127.0.0.1';
const defaultWebHttpScheme = defaultWebLocation?.protocol === 'https:' ? 'https' : 'http';
const defaultWebWsScheme = defaultWebLocation?.protocol === 'https:' ? 'wss' : 'ws';
const defaultNativeHost = (() => {
  if (Platform.OS === 'web') return '';
  try {
    const url = Linking.createURL('/');
    const parsed = new URL(url);
    if (parsed.hostname) return parsed.hostname;
  } catch {
    // Fallback below.
  }
  const debuggerHost = (Constants as unknown as { manifest2?: { extra?: { expoGo?: { debuggerHost?: string } } } })
    .manifest2?.extra?.expoGo?.debuggerHost;
  if (typeof debuggerHost === 'string' && debuggerHost.length > 0) {
    return debuggerHost.split(':')[0] ?? '';
  }
  return '';
})();
const defaultHttpHost = Platform.OS === 'web' ? defaultWebHost : (defaultNativeHost || '127.0.0.1');
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

const apiUrlFromConfig = process.env.EXPO_PUBLIC_API_URL ??
  (typeof extra.apiUrl === 'string' ? extra.apiUrl : undefined);
const wsUrlFromConfig = process.env.EXPO_PUBLIC_WS_URL ??
  (typeof extra.wsUrl === 'string' ? extra.wsUrl : undefined);
const resolvedApiUrl = apiUrlFromConfig ?? defaultApiBase;
const resolvedWsUrl = wsUrlFromConfig ?? deriveWsUrlFromApiUrl(resolvedApiUrl) ?? defaultWsUrl;

export const env = {
  isWeb: Platform.OS === 'web',
  defaultAssetBaseUrl: defaultWebAssetBaseUrl,
  apiUrl: resolvedApiUrl,
  apiUrlExplicit:
    typeof process.env.EXPO_PUBLIC_API_URL === 'string' ||
    typeof extra.apiUrl === 'string',
  wsUrl: resolvedWsUrl,
  wsUrlExplicit:
    typeof process.env.EXPO_PUBLIC_WS_URL === 'string' ||
    typeof extra.wsUrl === 'string',
  assetBaseUrl:
    process.env.EXPO_PUBLIC_ASSET_BASE_URL ??
    (typeof extra.assetBaseUrl === 'string'
      ? extra.assetBaseUrl
      : defaultWebAssetBaseUrl),
  closePageTransition:
    process.env.EXPO_PUBLIC_CLOSE_PAGE_TRANSITION ??
    (typeof extra.closePageTransition === 'string'
      ? extra.closePageTransition
      : 'none'),
  closePageTransitionMs:
    Number(process.env.EXPO_PUBLIC_CLOSE_PAGE_TRANSITION_MS) ||
    (typeof extra.closePageTransitionMs === 'number' ? extra.closePageTransitionMs : 220),
  closeButtonAnimation:
    process.env.EXPO_PUBLIC_CLOSE_BUTTON_ANIMATION ??
    (typeof extra.closeButtonAnimation === 'string'
      ? extra.closeButtonAnimation
      : 'subtle'),
  oidcIssuerUrl:
    process.env.EXPO_PUBLIC_OIDC_ISSUER_URL ??
    (typeof extra.oidcIssuerUrl === 'string' ? extra.oidcIssuerUrl : ''),
  oidcClientId:
    process.env.EXPO_PUBLIC_OIDC_CLIENT_ID ??
    (typeof extra.oidcClientId === 'string' ? extra.oidcClientId : ''),
  oidcAudience:
    process.env.EXPO_PUBLIC_OIDC_AUDIENCE ??
    (typeof extra.oidcAudience === 'string' ? extra.oidcAudience : ''),
  oidcScopes:
    process.env.EXPO_PUBLIC_OIDC_SCOPES ??
    (typeof extra.oidcScopes === 'string' ? extra.oidcScopes : 'openid profile email offline_access')
};
