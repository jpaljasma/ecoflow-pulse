import AsyncStorage from '@react-native-async-storage/async-storage';
// eslint-disable-next-line @typescript-eslint/no-require-imports
const { create } = require('zustand') as typeof import('zustand');
// eslint-disable-next-line @typescript-eslint/no-require-imports
const { createJSONStorage, persist } = require('zustand/middleware') as typeof import('zustand/middleware');

type EnergySettingsState = {
  hydrated: boolean;
  gridPricePerKwh: string;
  currency: string;
  setHydrated: (hydrated: boolean) => void;
  setGridPricePerKwh: (value: string) => void;
  setCurrency: (value: string) => void;
};

export const useEnergySettingsStore = create<EnergySettingsState>()(
  persist(
    (set) => ({
      hydrated: false,
      gridPricePerKwh: '0.30',
      currency: 'USD',
      setHydrated: (hydrated) => set({ hydrated }),
      setGridPricePerKwh: (gridPricePerKwh) => set({ gridPricePerKwh }),
      setCurrency: (currency) => set({ currency })
    }),
    {
      name: 'pulse-energy-settings-v1',
      version: 1,
      storage: createJSONStorage(() => AsyncStorage),
      partialize: (state) => ({
        gridPricePerKwh: state.gridPricePerKwh,
        currency: state.currency
      }),
      onRehydrateStorage: () => (state) => {
        state?.setHydrated(true);
      }
    }
  )
);
