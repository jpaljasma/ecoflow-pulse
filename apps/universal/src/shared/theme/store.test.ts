import { beforeEach, describe, expect, it, vi } from 'vitest';
import { defaultThemeFamily } from './catalog';

const STORAGE_KEY = 'pulse-theme-v1';
const persistedState = new Map<string, string>();

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

describe('theme store', () => {
  beforeEach(() => {
    persistedState.clear();
    vi.clearAllMocks();
    vi.resetModules();
  });

  it('boots with the configured default theme when there is no saved preference', async () => {
    const { useThemeStore } = await import('./store');

    expect(useThemeStore.getState().family).toBe(defaultThemeFamily);

    await useThemeStore.persist.rehydrate();

    expect(useThemeStore.getState().family).toBe(defaultThemeFamily);
    expect(useThemeStore.getState().hydrated).toBe(true);
    expect(asyncStorageMock.getItem).toHaveBeenCalledWith(STORAGE_KEY);
  });

  it('restores a persisted theme family over the default family', async () => {
    persistedState.set(
      STORAGE_KEY,
      JSON.stringify({
        state: { family: 'original' },
        version: 1
      })
    );

    const { useThemeStore } = await import('./store');

    await useThemeStore.persist.rehydrate();

    expect(useThemeStore.getState().family).toBe('original');
    expect(useThemeStore.getState().hydrated).toBe(true);
  });

  it('migrates legacy persisted variants into the new family-based preference', async () => {
    persistedState.set(
      STORAGE_KEY,
      JSON.stringify({
        state: { variant: 'original-light' },
        version: 0
      })
    );

    const { useThemeStore } = await import('./store');

    await useThemeStore.persist.rehydrate();

    expect(useThemeStore.getState().family).toBe('original');
  });

  it('persists theme family changes after a user selects a different palette', async () => {
    const { useThemeStore } = await import('./store');

    await useThemeStore.persist.rehydrate();
    useThemeStore.getState().setFamily('new');

    await vi.waitFor(() => {
      expect(asyncStorageMock.setItem).toHaveBeenCalledWith(
        STORAGE_KEY,
        expect.stringContaining('"family":"new"')
      );
    });

    expect(JSON.parse(persistedState.get(STORAGE_KEY) ?? '{}').state.family).toBe('new');
  });
});
