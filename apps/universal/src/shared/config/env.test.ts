import { afterEach, describe, expect, it, vi } from 'vitest';

const ORIGINAL_ENV = { ...process.env };
const ORIGINAL_WINDOW = globalThis.window;

async function loadEnvModule({
  apiUrlEnv,
  wsUrlEnv,
  extra = {}
}: {
  apiUrlEnv?: string;
  wsUrlEnv?: string;
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
});
