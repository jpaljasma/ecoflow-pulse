import { afterEach, describe, expect, it, vi } from 'vitest';

const ORIGINAL_ENV = { ...process.env };
const ORIGINAL_WINDOW = globalThis.window;

async function loadEnvModule({
  apiUrlEnv,
  wsUrlEnv,
  cloudApiUrlEnv,
  cloudWsUrlEnv,
  oidcIssuerUrlEnv,
  oidcClientIdEnv,
  cloudOidcIssuerUrlEnv,
  cloudOidcClientIdEnv,
  defaultConnectionProfileEnv,
  localDataPlaneEnv,
  extra = {}
}: {
  apiUrlEnv?: string;
  wsUrlEnv?: string;
  cloudApiUrlEnv?: string;
  cloudWsUrlEnv?: string;
  oidcIssuerUrlEnv?: string;
  oidcClientIdEnv?: string;
  cloudOidcIssuerUrlEnv?: string;
  cloudOidcClientIdEnv?: string;
  defaultConnectionProfileEnv?: string;
  localDataPlaneEnv?: string;
  extra?: Record<string, unknown>;
}) {
  vi.resetModules();

  if (apiUrlEnv === undefined) {
    delete process.env.EXPO_PUBLIC_API_URL;
  } else {
    process.env.EXPO_PUBLIC_API_URL = apiUrlEnv;
  }

  if (wsUrlEnv === undefined) {
    delete process.env.EXPO_PUBLIC_WS_URL;
  } else {
    process.env.EXPO_PUBLIC_WS_URL = wsUrlEnv;
  }

  if (cloudApiUrlEnv === undefined) {
    delete process.env.EXPO_PUBLIC_CLOUD_API_URL;
  } else {
    process.env.EXPO_PUBLIC_CLOUD_API_URL = cloudApiUrlEnv;
  }

  if (cloudWsUrlEnv === undefined) {
    delete process.env.EXPO_PUBLIC_CLOUD_WS_URL;
  } else {
    process.env.EXPO_PUBLIC_CLOUD_WS_URL = cloudWsUrlEnv;
  }

  if (oidcIssuerUrlEnv === undefined) {
    delete process.env.EXPO_PUBLIC_OIDC_ISSUER_URL;
  } else {
    process.env.EXPO_PUBLIC_OIDC_ISSUER_URL = oidcIssuerUrlEnv;
  }

  if (oidcClientIdEnv === undefined) {
    delete process.env.EXPO_PUBLIC_OIDC_CLIENT_ID;
  } else {
    process.env.EXPO_PUBLIC_OIDC_CLIENT_ID = oidcClientIdEnv;
  }

  if (cloudOidcIssuerUrlEnv === undefined) {
    delete process.env.EXPO_PUBLIC_CLOUD_OIDC_ISSUER_URL;
  } else {
    process.env.EXPO_PUBLIC_CLOUD_OIDC_ISSUER_URL = cloudOidcIssuerUrlEnv;
  }

  if (cloudOidcClientIdEnv === undefined) {
    delete process.env.EXPO_PUBLIC_CLOUD_OIDC_CLIENT_ID;
  } else {
    process.env.EXPO_PUBLIC_CLOUD_OIDC_CLIENT_ID = cloudOidcClientIdEnv;
  }

  if (defaultConnectionProfileEnv === undefined) {
    delete process.env.EXPO_PUBLIC_DEFAULT_CONNECTION_PROFILE;
  } else {
    process.env.EXPO_PUBLIC_DEFAULT_CONNECTION_PROFILE = defaultConnectionProfileEnv;
  }

  if (localDataPlaneEnv === undefined) {
    delete process.env.EXPO_PUBLIC_LOCAL_DATA_PLANE;
  } else {
    process.env.EXPO_PUBLIC_LOCAL_DATA_PLANE = localDataPlaneEnv;
  }

  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: {
      location: {
        origin: 'https://localhost',
        hostname: 'localhost',
        protocol: 'https:',
        port: ''
      }
    }
  });

  vi.doMock('react-native', () => ({
    Platform: { OS: 'web' }
  }));

  vi.doMock('expo-linking', () => ({
    default: {},
    createURL: () => 'ecoflowpulse:///'
  }));

  vi.doMock('expo-constants', () => ({
    default: {
      expoConfig: {
        extra
      }
    }
  }));

  return import('@/shared/config/env');
}

describe('env config resolution', () => {
  afterEach(() => {
    vi.resetModules();
    vi.unmock('react-native');
    vi.unmock('expo-linking');
    vi.unmock('expo-constants');
    process.env = { ...ORIGINAL_ENV };
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: ORIGINAL_WINDOW
    });
  });

  it('falls back to secure localhost defaults when EXPO_PUBLIC_API_URL and EXPO_PUBLIC_WS_URL are blank', async () => {
    const { env } = await loadEnvModule({
      apiUrlEnv: '',
      wsUrlEnv: ''
    });

    expect(env.apiUrl).toBe('https://localhost');
    expect(env.wsUrl).toBe('wss://localhost/ws');
    expect(env.apiUrlExplicit).toBe(false);
    expect(env.wsUrlExplicit).toBe(false);
  });

  it('falls back to secure localhost defaults when extra apiUrl and wsUrl are blank', async () => {
    const { env } = await loadEnvModule({
      extra: {
        apiUrl: '   ',
        wsUrl: ''
      }
    });

    expect(env.apiUrl).toBe('https://localhost');
    expect(env.wsUrl).toBe('wss://localhost/ws');
    expect(env.apiUrlExplicit).toBe(false);
    expect(env.wsUrlExplicit).toBe(false);
  });

  it('does not default the local OIDC client for an explicit custom issuer', async () => {
    const { readConnectionProfile } = await loadEnvModule({
      oidcIssuerUrlEnv: 'https://auth.example.com/realms/pulse'
    });

    expect(readConnectionProfile('local').oidcIssuerUrl).toBe('https://auth.example.com/realms/pulse');
    expect(readConnectionProfile('local').oidcClientId).toBe('');
  });

  it('exposes a cloud profile and honors it as the active default when configured', async () => {
    const { env, readConnectionProfile } = await loadEnvModule({
      cloudApiUrlEnv: 'https://pulse.example.com',
      cloudWsUrlEnv: 'wss://pulse.example.com/ws',
      cloudOidcIssuerUrlEnv: 'https://pulse.example.com/realms/pulse',
      cloudOidcClientIdEnv: 'pulse-universal-cloud',
      defaultConnectionProfileEnv: 'cloud'
    });

    expect(env.defaultConnectionProfileId).toBe('cloud');
    expect(env.connectionProfileId).toBe('cloud');
    expect(env.apiUrl).toBe('https://pulse.example.com');
    expect(env.wsUrl).toBe('wss://pulse.example.com/ws');
    expect(readConnectionProfile('cloud').oidcClientId).toBe('pulse-universal-cloud');
  });

  it('falls back to the local profile when cloud is requested as default but not configured', async () => {
    const { env } = await loadEnvModule({
      defaultConnectionProfileEnv: 'cloud'
    });

    expect(env.defaultConnectionProfileId).toBe('local');
    expect(env.connectionProfileId).toBe('local');
    expect(env.apiUrl).toBe('https://localhost');
  });

  it('models local-edge cloud-data mode as the active cloud profile', async () => {
    const { env, readConnectionProfile } = await loadEnvModule({
      cloudApiUrlEnv: 'https://pulse.example.com',
      cloudWsUrlEnv: 'wss://pulse.example.com/ws',
      cloudOidcIssuerUrlEnv: 'https://pulse.example.com/realms/pulse',
      cloudOidcClientIdEnv: 'pulse-universal-cloud',
      defaultConnectionProfileEnv: 'local',
      localDataPlaneEnv: 'cloud'
    });

    expect(env.defaultConnectionProfileId).toBe('cloud');
    expect(env.connectionProfileId).toBe('cloud');
    expect(readConnectionProfile('local').dataPlane).toBe('local');
    expect(readConnectionProfile('cloud').edge).toBe('local');
    expect(readConnectionProfile('cloud').dataPlane).toBe('cloud');
    expect(readConnectionProfile('cloud').apiUrl).toBe('https://localhost');
    expect(readConnectionProfile('cloud').wsUrl).toBe('wss://localhost/ws');
    expect(readConnectionProfile('cloud').oidcIssuerUrl).toBe('https://localhost/realms/pulse');
    expect(readConnectionProfile('cloud').oidcClientId).toBe('pulse-universal-app');
    expect(env.activeConnectionProfile.dataPlane).toBe('cloud');
  });
});
