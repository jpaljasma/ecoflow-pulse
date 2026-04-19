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

type RuntimeConnectionOverrides = {
  apiUrl?: string;
  apiUrlExplicit?: boolean;
  wsUrl?: string;
  wsUrlExplicit?: boolean;
  oidcIssuerUrl?: string;
  oidcClientId?: string;
  oidcAudience?: string;
  oidcScopes?: string;
};

export type ConnectionProfileId = 'local' | 'cloud';

export type ConnectionProfileConfig = {
  id: ConnectionProfileId;
  label: string;
  apiUrl: string;
  apiUrlExplicit: boolean;
  wsUrl: string;
  wsUrlExplicit: boolean;
  oidcIssuerUrl: string;
  oidcClientId: string;
  oidcAudience: string;
  oidcScopes: string;
  configured: boolean;
};

type EnvShape = {
  isWeb: boolean;
  nativeHostHints: string[];
  defaultAssetBaseUrl: string;
  assetBaseUrl: string;
  closePageTransition: string;
  closePageTransitionMs: number;
  closeButtonAnimation: string;
  connectionProfileId: ConnectionProfileId;
  readonly defaultConnectionProfileId: ConnectionProfileId;
  readonly connectionProfiles: Record<ConnectionProfileId, ConnectionProfileConfig>;
  readonly activeConnectionProfile: ConnectionProfileConfig;
  apiUrl: string;
  apiUrlExplicit: boolean;
  wsUrl: string;
  wsUrlExplicit: boolean;
  oidcIssuerUrl: string;
  oidcClientId: string;
  oidcAudience: string;
  oidcScopes: string;
};

const constantsLike = Constants as unknown as ExpoConstantsLike;
const runtimeConnectionOverrides: RuntimeConnectionOverrides = {};

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
  nativeHostHints.find((host) => !isLoopbackHost(host)) ?? nativeHostHints[0] ?? '127.0.0.1';

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

function readConnectionProfileId(value: unknown): ConnectionProfileId | null {
  const normalized = typeof value === 'string' ? value.trim().toLowerCase() : '';
  switch (normalized) {
    case 'local':
    case 'cloud':
      return normalized;
    default:
      return null;
  }
}

const localApiUrlFromConfig =
  readConfiguredString(process.env.EXPO_PUBLIC_API_URL) ??
  readConfiguredString((extra as { apiUrl?: unknown }).apiUrl);
const localWsUrlFromConfig =
  readConfiguredString(process.env.EXPO_PUBLIC_WS_URL) ??
  readConfiguredString((extra as { wsUrl?: unknown }).wsUrl);
const localOidcIssuerUrl =
  readConfiguredString(process.env.EXPO_PUBLIC_OIDC_ISSUER_URL) ??
  readConfiguredString((extra as { oidcIssuerUrl?: unknown }).oidcIssuerUrl) ??
  '';
const localOidcClientId =
  readConfiguredString(process.env.EXPO_PUBLIC_OIDC_CLIENT_ID) ??
  readConfiguredString((extra as { oidcClientId?: unknown }).oidcClientId) ??
  '';
const localOidcAudience =
  readConfiguredString(process.env.EXPO_PUBLIC_OIDC_AUDIENCE) ??
  readConfiguredString((extra as { oidcAudience?: unknown }).oidcAudience) ??
  '';
const localOidcScopes =
  readConfiguredString(process.env.EXPO_PUBLIC_OIDC_SCOPES) ??
  readConfiguredString((extra as { oidcScopes?: unknown }).oidcScopes) ??
  'openid profile email offline_access';

const resolvedLocalApiUrl = localApiUrlFromConfig ?? defaultApiBase;
const resolvedLocalWsUrl =
  localWsUrlFromConfig ?? deriveWsUrlFromApiUrl(resolvedLocalApiUrl) ?? defaultWsUrl;

const cloudApiUrlFromConfig =
  readConfiguredString(process.env.EXPO_PUBLIC_CLOUD_API_URL) ??
  readConfiguredString((extra as { cloudApiUrl?: unknown }).cloudApiUrl);
const cloudWsUrlFromConfig =
  readConfiguredString(process.env.EXPO_PUBLIC_CLOUD_WS_URL) ??
  readConfiguredString((extra as { cloudWsUrl?: unknown }).cloudWsUrl);
const cloudOidcIssuerUrl =
  readConfiguredString(process.env.EXPO_PUBLIC_CLOUD_OIDC_ISSUER_URL) ??
  readConfiguredString((extra as { cloudOidcIssuerUrl?: unknown }).cloudOidcIssuerUrl) ??
  '';
const cloudOidcClientId =
  readConfiguredString(process.env.EXPO_PUBLIC_CLOUD_OIDC_CLIENT_ID) ??
  readConfiguredString((extra as { cloudOidcClientId?: unknown }).cloudOidcClientId) ??
  '';
const cloudOidcAudience =
  readConfiguredString(process.env.EXPO_PUBLIC_CLOUD_OIDC_AUDIENCE) ??
  readConfiguredString((extra as { cloudOidcAudience?: unknown }).cloudOidcAudience) ??
  '';
const cloudOidcScopes =
  readConfiguredString(process.env.EXPO_PUBLIC_CLOUD_OIDC_SCOPES) ??
  readConfiguredString((extra as { cloudOidcScopes?: unknown }).cloudOidcScopes) ??
  localOidcScopes;

const resolvedCloudApiUrl = cloudApiUrlFromConfig ?? '';
const resolvedCloudWsUrl =
  cloudWsUrlFromConfig ?? deriveWsUrlFromApiUrl(resolvedCloudApiUrl) ?? '';

const connectionProfiles: Record<ConnectionProfileId, ConnectionProfileConfig> = {
  local: {
    id: 'local',
    label: 'Local',
    apiUrl: resolvedLocalApiUrl,
    apiUrlExplicit:
      readConfiguredString(process.env.EXPO_PUBLIC_API_URL) !== undefined ||
      readConfiguredString((extra as { apiUrl?: unknown }).apiUrl) !== undefined,
    wsUrl: resolvedLocalWsUrl,
    wsUrlExplicit:
      readConfiguredString(process.env.EXPO_PUBLIC_WS_URL) !== undefined ||
      readConfiguredString((extra as { wsUrl?: unknown }).wsUrl) !== undefined,
    oidcIssuerUrl: localOidcIssuerUrl,
    oidcClientId: localOidcClientId,
    oidcAudience: localOidcAudience,
    oidcScopes: localOidcScopes,
    configured: Boolean(resolvedLocalApiUrl && resolvedLocalWsUrl)
  },
  cloud: {
    id: 'cloud',
    label: 'Cloud',
    apiUrl: resolvedCloudApiUrl,
    apiUrlExplicit:
      readConfiguredString(process.env.EXPO_PUBLIC_CLOUD_API_URL) !== undefined ||
      readConfiguredString((extra as { cloudApiUrl?: unknown }).cloudApiUrl) !== undefined,
    wsUrl: resolvedCloudWsUrl,
    wsUrlExplicit:
      readConfiguredString(process.env.EXPO_PUBLIC_CLOUD_WS_URL) !== undefined ||
      readConfiguredString((extra as { cloudWsUrl?: unknown }).cloudWsUrl) !== undefined,
    oidcIssuerUrl: cloudOidcIssuerUrl,
    oidcClientId: cloudOidcClientId,
    oidcAudience: cloudOidcAudience,
    oidcScopes: cloudOidcScopes,
    configured: Boolean(resolvedCloudApiUrl && resolvedCloudWsUrl)
  }
};

const configuredDefaultConnectionProfileId =
  readConnectionProfileId(
    readConfiguredString(process.env.EXPO_PUBLIC_DEFAULT_CONNECTION_PROFILE) ??
      readConfiguredString((extra as { defaultConnectionProfile?: unknown }).defaultConnectionProfile)
  ) ?? 'local';

export const defaultConnectionProfileId =
  connectionProfiles[configuredDefaultConnectionProfileId].configured
    ? configuredDefaultConnectionProfileId
    : connectionProfiles.local.configured
      ? 'local'
      : 'cloud';

let activeConnectionProfileId: ConnectionProfileId = defaultConnectionProfileId;

function applyRuntimeOverrides(
  profileId: ConnectionProfileId,
  profile: ConnectionProfileConfig
): ConnectionProfileConfig {
  if (profileId !== activeConnectionProfileId) {
    return profile;
  }

  return {
    ...profile,
    apiUrl: runtimeConnectionOverrides.apiUrl ?? profile.apiUrl,
    apiUrlExplicit: runtimeConnectionOverrides.apiUrlExplicit ?? profile.apiUrlExplicit,
    wsUrl: runtimeConnectionOverrides.wsUrl ?? profile.wsUrl,
    wsUrlExplicit: runtimeConnectionOverrides.wsUrlExplicit ?? profile.wsUrlExplicit,
    oidcIssuerUrl: runtimeConnectionOverrides.oidcIssuerUrl ?? profile.oidcIssuerUrl,
    oidcClientId: runtimeConnectionOverrides.oidcClientId ?? profile.oidcClientId,
    oidcAudience: runtimeConnectionOverrides.oidcAudience ?? profile.oidcAudience,
    oidcScopes: runtimeConnectionOverrides.oidcScopes ?? profile.oidcScopes
  };
}

export function readConnectionProfiles(): Record<ConnectionProfileId, ConnectionProfileConfig> {
  return connectionProfiles;
}

export function readConnectionProfile(profileId: ConnectionProfileId): ConnectionProfileConfig {
  return connectionProfiles[profileId];
}

export function isConnectionProfileConfigured(profileId: ConnectionProfileId): boolean {
  return connectionProfiles[profileId].configured;
}

export function getActiveConnectionProfileId(): ConnectionProfileId {
  return activeConnectionProfileId;
}

export function setActiveConnectionProfileId(profileId: ConnectionProfileId): ConnectionProfileId {
  activeConnectionProfileId =
    connectionProfiles[profileId]?.configured ? profileId : defaultConnectionProfileId;
  return activeConnectionProfileId;
}

export function readActiveConnectionProfile(): ConnectionProfileConfig {
  return applyRuntimeOverrides(
    activeConnectionProfileId,
    readConnectionProfile(activeConnectionProfileId)
  );
}

export const env: EnvShape = {
  isWeb: Platform.OS === 'web',
  nativeHostHints,
  defaultAssetBaseUrl: defaultWebAssetBaseUrl,
  get assetBaseUrl() {
    return (
      readConfiguredString(process.env.EXPO_PUBLIC_ASSET_BASE_URL) ??
      readConfiguredString((extra as { assetBaseUrl?: unknown }).assetBaseUrl) ??
      defaultWebAssetBaseUrl
    );
  },
  get closePageTransition() {
    return (
      readConfiguredString(process.env.EXPO_PUBLIC_CLOSE_PAGE_TRANSITION) ??
      readConfiguredString((extra as { closePageTransition?: unknown }).closePageTransition) ??
      'none'
    );
  },
  get closePageTransitionMs() {
    return (
      Number(process.env.EXPO_PUBLIC_CLOSE_PAGE_TRANSITION_MS) ||
      (typeof extra.closePageTransitionMs === 'number' ? extra.closePageTransitionMs : 220)
    );
  },
  get closeButtonAnimation() {
    return (
      readConfiguredString(process.env.EXPO_PUBLIC_CLOSE_BUTTON_ANIMATION) ??
      readConfiguredString((extra as { closeButtonAnimation?: unknown }).closeButtonAnimation) ??
      'subtle'
    );
  },
  get connectionProfileId() {
    return getActiveConnectionProfileId();
  },
  set connectionProfileId(value: ConnectionProfileId) {
    setActiveConnectionProfileId(value);
  },
  get defaultConnectionProfileId() {
    return defaultConnectionProfileId;
  },
  get connectionProfiles() {
    return readConnectionProfiles();
  },
  get activeConnectionProfile() {
    return readActiveConnectionProfile();
  },
  get apiUrl() {
    return readActiveConnectionProfile().apiUrl;
  },
  set apiUrl(value: string) {
    runtimeConnectionOverrides.apiUrl = value;
  },
  get apiUrlExplicit() {
    return readActiveConnectionProfile().apiUrlExplicit;
  },
  set apiUrlExplicit(value: boolean) {
    runtimeConnectionOverrides.apiUrlExplicit = value;
  },
  get wsUrl() {
    return readActiveConnectionProfile().wsUrl;
  },
  set wsUrl(value: string) {
    runtimeConnectionOverrides.wsUrl = value;
  },
  get wsUrlExplicit() {
    return readActiveConnectionProfile().wsUrlExplicit;
  },
  set wsUrlExplicit(value: boolean) {
    runtimeConnectionOverrides.wsUrlExplicit = value;
  },
  get oidcIssuerUrl() {
    return readActiveConnectionProfile().oidcIssuerUrl;
  },
  set oidcIssuerUrl(value: string) {
    runtimeConnectionOverrides.oidcIssuerUrl = value;
  },
  get oidcClientId() {
    return readActiveConnectionProfile().oidcClientId;
  },
  set oidcClientId(value: string) {
    runtimeConnectionOverrides.oidcClientId = value;
  },
  get oidcAudience() {
    return readActiveConnectionProfile().oidcAudience;
  },
  set oidcAudience(value: string) {
    runtimeConnectionOverrides.oidcAudience = value;
  },
  get oidcScopes() {
    return readActiveConnectionProfile().oidcScopes;
  },
  set oidcScopes(value: string) {
    runtimeConnectionOverrides.oidcScopes = value;
  }
};
