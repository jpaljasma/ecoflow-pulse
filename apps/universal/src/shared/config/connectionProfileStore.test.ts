import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const STORAGE_KEY = 'pulse-connection-profile-v1';
const persistedState = new Map<string, string>();
const ORIGINAL_ENV = { ...process.env };
const ORIGINAL_WINDOW = globalThis.window;

const asyncStorageMock = {
  getItem: vi.fn(async (key: string) => persistedState.get(key) ?? null),
  setItem: vi.fn(async (key: string, value: string) => {
    persistedState.set(key, value);
  }),
  removeItem: vi.fn(async (key: string) => {
    persistedState.delete(key);
  })
};

vi.mock('@react-native-async-storage/async-storage', () => ({
  default: asyncStorageMock
}));

async function loadStoreModule() {
  vi.resetModules();

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
        extra: {}
      }
    }
  }));

  return import('./connectionProfileStore');
}

describe('connection profile store', () => {
  beforeEach(() => {
    persistedState.clear();
    vi.clearAllMocks();
    process.env = {
      ...ORIGINAL_ENV,
      EXPO_PUBLIC_CLOUD_API_URL: 'https://pulse.example.com',
      EXPO_PUBLIC_CLOUD_WS_URL: 'wss://pulse.example.com/ws',
      EXPO_PUBLIC_CLOUD_OIDC_ISSUER_URL: 'https://pulse.example.com/realms/pulse',
      EXPO_PUBLIC_CLOUD_OIDC_CLIENT_ID: 'pulse-universal-cloud',
      EXPO_PUBLIC_API_URL: 'https://localhost',
      EXPO_PUBLIC_WS_URL: 'wss://localhost/ws',
      EXPO_PUBLIC_OIDC_ISSUER_URL: 'https://localhost/realms/pulse',
      EXPO_PUBLIC_OIDC_CLIENT_ID: 'pulse-universal-app',
      EXPO_PUBLIC_DEFAULT_CONNECTION_PROFILE: 'local'
    };
  });

  afterEach(() => {
    vi.resetModules();
    vi.doUnmock('react-native');
    vi.doUnmock('expo-linking');
    vi.doUnmock('expo-constants');
    process.env = { ...ORIGINAL_ENV };
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: ORIGINAL_WINDOW
    });
  });

  it('hydrates to the configured default profile when no preference is saved', async () => {
    const { useConnectionProfileStore } = await loadStoreModule();
    const { env } = await import('./env');

    expect(useConnectionProfileStore.getState().profileId).toBe('local');

    await useConnectionProfileStore.persist.rehydrate();

    expect(useConnectionProfileStore.getState().profileId).toBe('local');
    expect(useConnectionProfileStore.getState().hydrated).toBe(true);
    expect(env.connectionProfileId).toBe('local');
    expect(asyncStorageMock.getItem).toHaveBeenCalledWith(STORAGE_KEY);
  });

  it('restores a persisted cloud preference and syncs the active env profile', async () => {
    persistedState.set(
      STORAGE_KEY,
      JSON.stringify({
        state: { profileId: 'cloud', defaultProfileId: 'local' },
        version: 3
      })
    );

    const { useConnectionProfileStore } = await loadStoreModule();
    const { env } = await import('./env');

    await useConnectionProfileStore.persist.rehydrate();

    expect(useConnectionProfileStore.getState().profileId).toBe('cloud');
    expect(useConnectionProfileStore.getState().hydrated).toBe(true);
    expect(env.connectionProfileId).toBe('cloud');
    expect(env.apiUrl).toBe('https://pulse.example.com');
  });

  it('persists profile changes after the user switches to cloud', async () => {
    const { useConnectionProfileStore } = await loadStoreModule();

    await useConnectionProfileStore.persist.rehydrate();
    useConnectionProfileStore.getState().setProfileId('cloud');

    await vi.waitFor(() => {
      expect(asyncStorageMock.setItem).toHaveBeenCalledWith(
        STORAGE_KEY,
        expect.stringContaining('"profileId":"cloud"')
      );
    });

    expect(JSON.parse(persistedState.get(STORAGE_KEY) ?? '{}').state.profileId).toBe('cloud');
    expect(JSON.parse(persistedState.get(STORAGE_KEY) ?? '{}').state.defaultProfileId).toBe(
      'local'
    );
  });

  it('migrates old local selections to cloud in local-edge cloud-data mode', async () => {
    process.env.EXPO_PUBLIC_LOCAL_DATA_PLANE = 'cloud';
    persistedState.set(
      STORAGE_KEY,
      JSON.stringify({
        state: { profileId: 'local' },
        version: 1
      })
    );

    const { useConnectionProfileStore } = await loadStoreModule();
    const { env } = await import('./env');

    await useConnectionProfileStore.persist.rehydrate();

    expect(useConnectionProfileStore.getState().profileId).toBe('cloud');
    expect(env.connectionProfileId).toBe('cloud');
    expect(env.activeConnectionProfile.edge).toBe('local');
    expect(env.activeConnectionProfile.dataPlane).toBe('cloud');
  });

  it('switches stale local-mode state to cloud when the active build uses local edge with cloud data', async () => {
    process.env.EXPO_PUBLIC_LOCAL_DATA_PLANE = 'cloud';
    persistedState.set(
      STORAGE_KEY,
      JSON.stringify({
        state: { profileId: 'local', defaultProfileId: 'local' },
        version: 3
      })
    );

    const { useConnectionProfileStore } = await loadStoreModule();
    const { env } = await import('./env');

    await useConnectionProfileStore.persist.rehydrate();

    expect(useConnectionProfileStore.getState().profileId).toBe('cloud');
    expect(env.connectionProfileId).toBe('cloud');
    expect(env.apiUrl).toBe('https://localhost');
    expect(env.oidcIssuerUrl).toBe('https://localhost/realms/pulse');
    expect(env.activeConnectionProfile.edge).toBe('local');
    expect(env.activeConnectionProfile.dataPlane).toBe('cloud');
  });

  it('migrates stale cloud selections back to local in normal local mode', async () => {
    persistedState.set(
      STORAGE_KEY,
      JSON.stringify({
        state: { profileId: 'cloud', defaultProfileId: 'cloud' },
        version: 3
      })
    );

    const { useConnectionProfileStore } = await loadStoreModule();
    const { env } = await import('./env');

    await useConnectionProfileStore.persist.rehydrate();

    expect(useConnectionProfileStore.getState().profileId).toBe('local');
    expect(env.connectionProfileId).toBe('local');
    expect(env.oidcIssuerUrl).toBe('https://localhost/realms/pulse');
  });
});
