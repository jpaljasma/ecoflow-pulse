import { beforeEach, describe, expect, it, vi } from 'vitest';

const STORAGE_KEY = 'pulse-energy-settings-v1';
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

describe('energy settings store', () => {
  beforeEach(() => {
    persistedState.clear();
    vi.clearAllMocks();
    vi.resetModules();
  });

  it('boots with the default grid price and currency when there is no saved setting', async () => {
    const { useEnergySettingsStore } = await import('./store');

    expect(useEnergySettingsStore.getState().gridPricePerKwh).toBe('0.30');
    expect(useEnergySettingsStore.getState().currency).toBe('USD');

    await useEnergySettingsStore.persist.rehydrate();

    expect(useEnergySettingsStore.getState().hydrated).toBe(true);
    expect(asyncStorageMock.getItem).toHaveBeenCalledWith(STORAGE_KEY);
  });

  it('restores the persisted grid price and currency', async () => {
    persistedState.set(
      STORAGE_KEY,
      JSON.stringify({
        state: {
          gridPricePerKwh: '0.42',
          currency: 'CAD'
        },
        version: 1
      })
    );

    const { useEnergySettingsStore } = await import('./store');

    await useEnergySettingsStore.persist.rehydrate();

    expect(useEnergySettingsStore.getState().gridPricePerKwh).toBe('0.42');
    expect(useEnergySettingsStore.getState().currency).toBe('CAD');
  });

  it('persists changes after the user updates the local price preference', async () => {
    const { useEnergySettingsStore } = await import('./store');

    await useEnergySettingsStore.persist.rehydrate();
    useEnergySettingsStore.getState().setGridPricePerKwh('0.18');
    useEnergySettingsStore.getState().setCurrency('EUR');

    await vi.waitFor(() => {
      expect(asyncStorageMock.setItem).toHaveBeenCalledWith(
        STORAGE_KEY,
        expect.stringContaining('"gridPricePerKwh":"0.18"')
      );
    });

    expect(JSON.parse(persistedState.get(STORAGE_KEY) ?? '{}').state.currency).toBe('EUR');
  });
});
